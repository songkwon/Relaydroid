package transfer

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"time"
)

func Devices(ctx context.Context) Scan {
	result := Scan{Devices: []Device{}, ADB: "可用", MTP: "可用"}
	type response struct {
		items []Device
		err   error
	}
	adb := make(chan response, 1)
	go func() { items, err := adbDevices(ctx); adb <- response{items, err} }()
	items, err := mtpDevices(ctx)
	if err != nil {
		result.MTP = err.Error()
	}
	a := <-adb
	if a.err != nil {
		result.ADB = a.err.Error()
	}
	for _, device := range a.items {
		if device.Kind == "adb-usb" && device.State == "device" {
			result.Devices = append(result.Devices, device)
		}
	}
	result.Devices = append(result.Devices, items...)
	for _, device := range a.items {
		if device.Kind != "adb-usb" || device.State != "device" {
			result.Devices = append(result.Devices, device)
		}
	}
	return result
}

func openRemote(ctx context.Context, device Device) (remote, error) {
	switch device.Kind {
	case "mtp":
		return openMTP(ctx, device.ID)
	case "adb-wifi", "adb-usb":
		if !adbSerialPattern.MatchString(device.ID) {
			return nil, errors.New("设备标识无效")
		}
		return adbRemote{device.ID, device.Transport}, nil
	default:
		return nil, errors.New("请选择已连接的 Android 设备")
	}
}

func ListRemote(ctx context.Context, device Device, directory string) ([]Entry, error) {
	r, err := openRemote(ctx, device)
	if err != nil {
		return nil, err
	}
	defer r.close()
	return r.list(ctx, directory)
}

func NewFolder(ctx context.Context, device Device, directory, name string) error {
	if err := checkName(name); err != nil {
		return err
	}
	r, err := openRemote(ctx, device)
	if err != nil {
		return err
	}
	defer r.close()
	_, err = r.mkdir(ctx, directory, name)
	return err
}

func Rename(ctx context.Context, device Device, p, name string) error {
	if err := checkName(name); err != nil {
		return err
	}
	r, err := openRemote(ctx, device)
	if err != nil {
		return err
	}
	defer r.close()
	return r.rename(ctx, p, name)
}

func Remove(ctx context.Context, device Device, p string) error {
	r, err := openRemote(ctx, device)
	if err != nil {
		return err
	}
	defer r.close()
	return r.remove(ctx, p)
}

func Execute(ctx context.Context, req Request, notify func(Progress)) error {
	if req.Direction != "upload" && req.Direction != "download" {
		return errors.New("无效的传输方向")
	}
	if len(req.Sources) == 0 {
		return nil
	}
	r, err := openRemote(ctx, req.Device)
	if err != nil {
		return err
	}
	defer r.close()

	var target *os.Root
	remoteItems := make(map[string]Entry)
	if req.Direction == "download" {
		if !filepath.IsAbs(req.Destination) {
			return errors.New("请选择电脑上的目标文件夹")
		}
		target, err = os.OpenRoot(req.Destination)
		if err != nil {
			return err
		}
		defer target.Close()
		parents := make(map[string]bool)
		for _, source := range req.Sources {
			parent := path.Dir(source)
			if parents[parent] {
				continue
			}
			items, err := r.list(ctx, parent)
			if err != nil {
				return err
			}
			for _, item := range items {
				remoteItems[item.Path] = item
			}
			parents[parent] = true
		}
	}

	var failures []error
	status := Progress{ID: req.ID, Total: len(req.Sources)}
	for _, source := range req.Sources {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		name := filepath.Base(source)
		if req.Direction == "download" {
			if item, ok := remoteItems[source]; ok {
				name = item.Name
			}
		}
		status.Current = name
		notify(status)
		switch req.Direction {
		case "upload":
			err = upload(ctx, r, source, req.Destination)
		case "download":
			item, ok := remoteItems[source]
			if !ok {
				err = errors.New("设备上已找不到这个项目")
				break
			}
			err = download(ctx, r, item, req.Destination, target)
		}
		if err != nil {
			failures = append(failures, fmt.Errorf("%s：%w", name, err))
		}
		status.Done++
		notify(status)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return errors.Join(failures...)
}

func upload(ctx context.Context, r remote, source, destination string) error {
	if !filepath.IsAbs(source) {
		return errors.New("请选择电脑上的文件或文件夹")
	}
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	return uploadEntry(ctx, r, source, destination, info)
}

func uploadEntry(ctx context.Context, r remote, local, dest string, info fs.FileInfo) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	name := info.Name()
	if err := checkName(name); err != nil {
		return err
	}
	if info.Mode().IsRegular() {
		_, err := r.push(ctx, local, dest, name)
		return err
	}
	if !info.IsDir() {
		return errors.New("只接受常规文件和文件夹，已跳过链接或特殊文件")
	}

	// Create a new destination directory. A failure can then clean up only
	// data created by this transfer, without touching existing files.
	parent, err := r.mkdir(ctx, dest, name)
	if err != nil {
		return err
	}
	if err := uploadChildren(ctx, r, local, parent); err != nil {
		cleanup, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if removeErr := r.remove(cleanup, parent); removeErr != nil {
			return errors.Join(err, fmt.Errorf("清理未完成的设备文件夹失败：%w", removeErr))
		}
		return err
	}
	return nil
}

func uploadChildren(ctx context.Context, r remote, local, dest string) error {
	children, err := os.ReadDir(local)
	if err != nil {
		return err
	}
	for _, child := range children {
		info, err := child.Info()
		if err != nil {
			return err
		}
		if err := uploadEntry(ctx, r, filepath.Join(local, child.Name()), dest, info); err != nil {
			return fmt.Errorf("%s：%w", child.Name(), err)
		}
	}
	return nil
}

func download(ctx context.Context, r remote, item Entry, local string, target *os.Root) error {
	if err := checkLocalName(item.Name); err != nil {
		return err
	}
	if _, err := target.Lstat(item.Name); err == nil {
		return errors.New("目标已有同名项目")
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	stage, err := os.MkdirTemp(local, ".relay-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	staged := filepath.Join(stage, "data")
	if err := downloadEntry(ctx, r, item, staged); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return commitLocal(target, filepath.Join(filepath.Base(stage), "data"), item.Name)
}

func downloadEntry(ctx context.Context, r remote, item Entry, local string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if item.Link {
		return errors.New("已跳过设备上的链接")
	}
	if !item.Directory {
		return r.pull(ctx, item.Path, local)
	}
	if err := os.Mkdir(local, 0755); err != nil {
		return err
	}
	children, err := r.list(ctx, item.Path)
	if err != nil {
		return err
	}
	names := make(map[string]bool)
	for _, child := range children {
		if err := checkLocalName(child.Name); err != nil {
			return err
		}
		if names[child.Name] {
			return fmt.Errorf("设备上存在重复的文件名 %q", child.Name)
		}
		names[child.Name] = true
		target := filepath.Join(local, child.Name)
		if _, err := os.Lstat(target); !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("本机文件系统无法区分目录中的 %q", child.Name)
		}
		if err := downloadEntry(ctx, r, child, target); err != nil {
			return fmt.Errorf("%s：%w", child.Name, err)
		}
	}
	return nil
}
