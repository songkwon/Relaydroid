package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/songkwon/relaydroid/internal/transfer"
	"golang.org/x/net/webdav"
)

type finderMount struct {
	server *http.Server
	point  string
}

func (r *Relaydroid) FinderMount(ctx context.Context, device transfer.Device) (string, error) {
	r.mountMu.Lock()
	defer r.mountMu.Unlock()
	if r.mount != nil {
		return r.mount.point, r.app.Env.OpenFileManager(r.mount.point, false)
	}
	mount, err := mountFinderVolume(ctx, device)
	if err != nil {
		return "", err
	}
	r.mount = mount
	_ = r.app.Env.OpenFileManager(mount.point, false)
	return mount.point, nil
}

func mountFinderVolume(ctx context.Context, device transfer.Device) (*finderMount, error) {
	ctx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()
	if _, err := transfer.ListRemote(ctx, device, device.Root); err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	var token [24]byte
	if _, err := rand.Read(token[:]); err != nil {
		listener.Close()
		return nil, err
	}
	prefix := "/" + hex.EncodeToString(token[:])
	host := listener.Addr().String()
	dav := &webdav.Handler{Prefix: prefix, FileSystem: transfer.FinderFS{Device: device}, LockSystem: webdav.NewMemLS()}
	server := &http.Server{
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    32 << 10,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.Host != host || (req.URL.Path != prefix && !strings.HasPrefix(req.URL.Path, prefix+"/")) {
				http.NotFound(w, req)
				return
			}
			switch req.Method {
			case "OPTIONS":
				w.Header().Set("DAV", "1")
				w.Header().Set("Allow", "OPTIONS, GET, HEAD, PROPFIND")
				w.Header().Set("MS-Author-Via", "DAV")
			case "GET", "HEAD", "PROPFIND":
				req.Body = http.MaxBytesReader(w, req.Body, 64<<10)
				dav.ServeHTTP(w, req)
			default:
				w.Header().Set("Allow", "OPTIONS, GET, HEAD, PROPFIND")
				w.WriteHeader(http.StatusMethodNotAllowed)
			}
		}),
	}
	go func() { _ = server.Serve(listener) }()
	point, err := os.MkdirTemp("", "Relaydroid-Android-")
	if err != nil {
		server.Close()
		return nil, err
	}
	url := "http://" + host + prefix + "/"
	out, err := exec.CommandContext(ctx, "/sbin/mount_webdav", "-S", "-v", "Relaydroid Android", "-o", "rdonly", url, point).CombinedOutput()
	if err != nil {
		server.Close()
		_ = os.Remove(point)
		return nil, fmt.Errorf("Finder 挂载失败：%s %w", strings.TrimSpace(string(out)), err)
	}
	return &finderMount{server: server, point: point}, nil
}

func (r *Relaydroid) FinderUnmount() error {
	r.mountMu.Lock()
	defer r.mountMu.Unlock()
	if r.mount == nil {
		return nil
	}
	m := r.mount
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "/sbin/umount", m.point).CombinedOutput()
	if err != nil {
		// Finder can eject the disk itself. An empty mount directory is safe to remove.
		if removeErr := os.Remove(m.point); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return fmt.Errorf("无法推出 Finder 中的设备：%s %w", strings.TrimSpace(string(out)), err)
		}
	}
	r.mount = nil
	_ = m.server.Close()
	_ = os.Remove(m.point)
	return nil
}
