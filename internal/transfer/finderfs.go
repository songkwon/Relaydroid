//go:build darwin

package transfer

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
	"time"

	"golang.org/x/net/webdav"
)

type FinderFS struct{ Device Device }

type finderInfo struct{ entry Entry }

func (f finderInfo) Name() string { return f.entry.Name }
func (f finderInfo) Size() int64  { return f.entry.Size }
func (f finderInfo) Mode() fs.FileMode {
	if f.IsDir() {
		return fs.ModeDir | 0555
	}
	return 0444
}
func (f finderInfo) ModTime() time.Time {
	if f.entry.Modified == 0 {
		return time.Unix(0, 0)
	}
	return time.Unix(f.entry.Modified, 0)
}
func (f finderInfo) IsDir() bool { return f.entry.Directory }
func (f finderInfo) Sys() any    { return nil }

func (f FinderFS) lookup(ctx context.Context, r remote, p string) (Entry, error) {
	current := Entry{Name: "Android", Path: f.Device.Root, Directory: true}
	if path.Clean(p) == "/" {
		return current, nil
	}
	for _, segment := range strings.Split(strings.TrimPrefix(path.Clean(p), "/"), "/") {
		if !current.Directory {
			return Entry{}, fs.ErrNotExist
		}
		children, err := r.list(ctx, current.Path)
		if err != nil {
			return Entry{}, err
		}
		found := false
		for _, child := range children {
			if child.Name == segment && !child.Link {
				current, found = child, true
				break
			}
		}
		if !found {
			return Entry{}, fs.ErrNotExist
		}
	}
	return current, nil
}

func (f FinderFS) Mkdir(context.Context, string, os.FileMode) error { return fs.ErrPermission }
func (f FinderFS) RemoveAll(context.Context, string) error          { return fs.ErrPermission }
func (f FinderFS) Rename(context.Context, string, string) error     { return fs.ErrPermission }

func (f FinderFS) Stat(ctx context.Context, p string) (os.FileInfo, error) {
	if path.Clean(p) == "/" {
		return finderInfo{Entry{Name: "Android", Path: f.Device.Root, Directory: true}}, nil
	}
	r, err := openRemote(ctx, f.Device)
	if err != nil {
		return nil, err
	}
	defer r.close()
	item, err := f.lookup(ctx, r, p)
	if err != nil {
		return nil, err
	}
	return finderInfo{item}, nil
}

func (f FinderFS) OpenFile(ctx context.Context, p string, flags int, _ os.FileMode) (webdav.File, error) {
	if flags != os.O_RDONLY {
		return nil, fs.ErrPermission
	}
	r, err := openRemote(ctx, f.Device)
	if err != nil {
		return nil, err
	}
	defer r.close()
	item, err := f.lookup(ctx, r, p)
	if err != nil {
		return nil, err
	}
	result := &finderFile{ctx: ctx, fs: f, item: item}
	if item.Directory {
		children, err := r.list(ctx, item.Path)
		if err != nil {
			return nil, err
		}
		for _, child := range children {
			if !child.Link {
				result.children = append(result.children, finderInfo{child})
			}
		}
	}
	return result, nil
}

type finderFile struct {
	ctx      context.Context
	fs       FinderFS
	item     Entry
	children []os.FileInfo
	offset   int64
	file     *os.File
	temp     string
}

func (f *finderFile) Stat() (os.FileInfo, error) { return finderInfo{f.item}, nil }
func (f *finderFile) Write([]byte) (int, error)  { return 0, fs.ErrPermission }
func (f *finderFile) Readdir(count int) ([]os.FileInfo, error) {
	if !f.item.Directory {
		return nil, fs.ErrInvalid
	}
	if count <= 0 || count >= len(f.children) {
		if count > 0 && len(f.children) == 0 {
			return nil, io.EOF
		}
		items := f.children
		f.children = nil
		return items, nil
	}
	items := f.children[:count]
	f.children = f.children[count:]
	return items, nil
}

func (f *finderFile) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
	case io.SeekCurrent:
		offset += f.offset
	case io.SeekEnd:
		offset += f.item.Size
	default:
		return 0, fs.ErrInvalid
	}
	if offset < 0 || f.item.Directory {
		return 0, fs.ErrInvalid
	}
	f.offset = offset
	return offset, nil
}

func (f *finderFile) Read(buffer []byte) (int, error) {
	if f.item.Directory {
		return 0, fs.ErrInvalid
	}
	if f.file == nil {
		temp, err := os.MkdirTemp("", "relay-finder-")
		if err != nil {
			return 0, err
		}
		f.temp = temp
		target := path.Join(temp, "payload")
		r, err := openRemote(f.ctx, f.fs.Device)
		if err != nil {
			return 0, err
		}
		err = r.pull(f.ctx, f.item.Path, target)
		r.close()
		if err != nil {
			return 0, err
		}
		f.file, err = os.Open(target)
		if err != nil {
			return 0, err
		}
	}
	n, err := f.file.ReadAt(buffer, f.offset)
	f.offset += int64(n)
	return n, err
}

func (f *finderFile) Close() error {
	if f.file != nil {
		_ = f.file.Close()
	}
	if f.temp != "" {
		return os.RemoveAll(f.temp)
	}
	return nil
}
