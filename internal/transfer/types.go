package transfer

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Device struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	State     string `json:"state"`
	Root      string `json:"root"`
	Transport uint64 `json:"transport,omitempty"`
}

type Entry struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Directory bool   `json:"directory"`
	Size      int64  `json:"size"`
	Modified  int64  `json:"modified"`
	Link      bool   `json:"link,omitempty"`
}

type Location struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type Scan struct {
	Devices []Device `json:"devices"`
	ADB     string   `json:"adb"`
	MTP     string   `json:"mtp"`
	Legacy  string   `json:"legacy,omitempty"`
}

type Request struct {
	ID          string   `json:"id"`
	Device      Device   `json:"device"`
	Direction   string   `json:"direction"`
	Destination string   `json:"destination"`
	Sources     []string `json:"sources"`
}

type Progress struct {
	ID      string `json:"id"`
	Current string `json:"current"`
	Done    int    `json:"done"`
	Total   int    `json:"total"`
}

type remote interface {
	list(context.Context, string) ([]Entry, error)
	pull(context.Context, string, string) error
	push(context.Context, string, string, string) (string, error)
	mkdir(context.Context, string, string) (string, error)
	rename(context.Context, string, string) error
	remove(context.Context, string) error
	close()
}

func checkName(name string) error {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\x00") {
		return errors.New("文件名不能为空，也不能包含斜线")
	}
	return nil
}

func checkLocalName(name string) error {
	if err := checkName(name); err != nil {
		return err
	}
	local, err := filepath.Localize(name)
	if err != nil || !filepath.IsLocal(local) || filepath.Base(local) != local {
		return fmt.Errorf("当前系统不支持文件名 %q", name)
	}
	return nil
}

func localEntry(directory string, e fs.DirEntry) (Entry, error) {
	info, err := e.Info()
	if err != nil {
		return Entry{}, err
	}
	return Entry{e.Name(), filepath.Join(directory, e.Name()), info.IsDir(), info.Size(), info.ModTime().Unix(), e.Type()&os.ModeSymlink != 0}, nil
}

func ListLocal(directory string) ([]Entry, error) {
	if !filepath.IsAbs(directory) {
		return nil, errors.New("请选择本机绝对路径")
	}
	items, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	result := make([]Entry, 0, len(items))
	for _, item := range items {
		entry, err := localEntry(directory, item)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		result = append(result, entry)
	}
	return result, nil
}

func unixDate(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}
