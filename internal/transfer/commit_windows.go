package transfer

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

func commitLocal(target *os.Root, stage, name string) error {
	src, err := windows.UTF16PtrFromString(filepath.Join(target.Name(), stage))
	if err != nil {
		return err
	}
	dst, err := windows.UTF16PtrFromString(filepath.Join(target.Name(), name))
	if err != nil {
		return err
	}
	return windows.MoveFile(src, dst)
}
