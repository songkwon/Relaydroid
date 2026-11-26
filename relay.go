package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/songkwon/Relay/internal/transfer"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type Relay struct {
	mountMu  sync.Mutex
	mount    *finderMount
	legacyMu sync.Mutex
	legacy   transfer.LegacySession
	app      *application.App
	window   *application.WebviewWindow
	mu       sync.Mutex
	jobs     map[string]job
}

type job struct{ cancel context.CancelFunc }

type Machine struct {
	OS     string              `json:"os"`
	Home   string              `json:"home"`
	Places []transfer.Location `json:"places"`
}

func (r *Relay) Machine() (Machine, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Machine{}, err
	}
	m := Machine{OS: runtime.GOOS, Home: home, Places: []transfer.Location{{Name: "个人文件夹", Path: home}}}
	for _, pair := range [][2]string{{"下载", "Downloads"}, {"桌面", "Desktop"}, {"文稿", "Documents"}, {"图片", "Pictures"}} {
		p := filepath.Join(home, pair[1])
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			m.Places = append(m.Places, transfer.Location{Name: pair[0], Path: p})
		}
	}
	return m, nil
}

func (r *Relay) Local(directory string) ([]transfer.Entry, error) {
	return transfer.ListLocal(directory)
}

func (r *Relay) Parent(directory string) string { return filepath.Dir(directory) }

func (r *Relay) Scan(ctx context.Context) transfer.Scan {
	scan := transfer.Devices(ctx)
	r.legacyMu.Lock()
	scan.Legacy = r.legacy.Endpoint
	r.legacyMu.Unlock()
	return scan
}

func (r *Relay) Remote(ctx context.Context, d transfer.Device, p string) ([]transfer.Entry, error) {
	return transfer.ListRemote(ctx, d, p)
}

func (r *Relay) Choose(directory string, folders bool, multiple bool) ([]string, error) {
	dialog := r.app.Dialog.OpenFile().
		AttachToWindow(r.window).
		SetTitle("选择电脑上的项目").
		CanChooseDirectories(folders).
		CanChooseFiles(!folders)
	if st, err := os.Stat(directory); err == nil && st.IsDir() {
		dialog.SetDirectory(directory)
	}
	if multiple {
		return dialog.PromptForMultipleSelection()
	}
	choice, err := dialog.PromptForSingleSelection()
	if choice == "" || err != nil {
		return []string{}, err
	}
	return []string{choice}, nil
}

func (r *Relay) Reveal(p string) error {
	if !filepath.IsAbs(p) {
		return errors.New("请选择本机文件")
	}
	return r.app.Env.OpenFileManager(p, true)
}

func (r *Relay) OpenURL(target string) error {
	switch target {
	case "platform-tools":
		return r.app.Browser.OpenURL("https://developer.android.com/tools/releases/platform-tools")
	default:
		return errors.New("未知链接")
	}
}

func (r *Relay) WiFiServices(ctx context.Context) ([]transfer.WiFiService, error) {
	return transfer.DiscoverWiFi(ctx)
}

func (r *Relay) Pair(ctx context.Context, endpoint, code string) (transfer.PairResult, error) {
	return transfer.Pair(ctx, endpoint, code)
}

func (r *Relay) Connect(ctx context.Context, endpoint string) (transfer.Device, error) {
	return transfer.Connect(ctx, endpoint)
}

func (r *Relay) StartLegacyWiFi(ctx context.Context, d transfer.Device) (transfer.Device, error) {
	r.legacyMu.Lock()
	defer r.legacyMu.Unlock()
	if r.legacy.Endpoint != "" {
		return transfer.Device{}, errors.New("请先结束已开启的临时无线连接")
	}
	s, device, err := transfer.StartLegacyWiFi(ctx, d)
	r.legacy = s
	return device, err
}

func (r *Relay) StopLegacyWiFi(ctx context.Context) error {
	r.legacyMu.Lock()
	defer r.legacyMu.Unlock()
	return r.stopLegacyLocked(ctx)
}

func (r *Relay) stopLegacyLocked(ctx context.Context) error {
	if r.legacy.Endpoint == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	if err := transfer.StopLegacyWiFi(ctx, r.legacy); err != nil {
		return err
	}
	r.legacy = transfer.LegacySession{}
	return nil
}

func (r *Relay) Disconnect(ctx context.Context, id string) error {
	r.legacyMu.Lock()
	defer r.legacyMu.Unlock()
	if id != "" && r.legacy.Endpoint == id {
		return r.stopLegacyLocked(ctx)
	}
	return transfer.Disconnect(ctx, id)
}

func (r *Relay) NewFolder(ctx context.Context, d transfer.Device, p, name string) error {
	return transfer.NewFolder(ctx, d, p, name)
}
func (r *Relay) Rename(ctx context.Context, d transfer.Device, p, name string) error {
	return transfer.Rename(ctx, d, p, name)
}
func (r *Relay) Remove(ctx context.Context, d transfer.Device, p string) error {
	return transfer.Remove(ctx, d, p)
}

func (r *Relay) Transfer(ctx context.Context, req transfer.Request) error {
	if req.ID == "" {
		return errors.New("传输标识为空")
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	r.mu.Lock()
	if len(r.jobs) > 0 {
		r.mu.Unlock()
		return errors.New("已有传输正在进行")
	}
	r.jobs[req.ID] = job{cancel}
	r.mu.Unlock()
	defer func() { r.mu.Lock(); delete(r.jobs, req.ID); r.mu.Unlock() }()
	return transfer.Execute(ctx, req, func(p transfer.Progress) { r.app.Event.Emit("relay:progress", p) })
}

func (r *Relay) Cancel(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if j, ok := r.jobs[id]; ok {
		j.cancel()
	}
}
