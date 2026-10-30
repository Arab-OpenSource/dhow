//go:build unix

package safefile

import (
	"errors"
	"os"
	"syscall"
)

func open(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0) //nolint:gosec // The caller names the file to read; it is read, never executed.
	if errors.Is(err, syscall.ELOOP) {
		return nil, ErrSymlink
	}
	return f, err
}
