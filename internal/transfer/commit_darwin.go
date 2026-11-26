package transfer

import (
	"os"

	"golang.org/x/sys/unix"
)

func commitLocal(target *os.Root, stage, name string) error {
	dir, err := target.Open(".")
	if err != nil {
		return err
	}
	defer dir.Close()
	fd := int(dir.Fd())
	return unix.RenameatxNp(fd, stage, fd, name, unix.RENAME_EXCL)
}
