//go:build !unix

package safefile

import (
	"io/fs"
	"os"
)

func open(path string) (*os.File, error) {
	info, err := os.Lstat(path)
	switch {
	case err != nil:
		return nil, err
	case info.Mode()&fs.ModeSymlink != 0:
		return nil, ErrSymlink
	case !info.Mode().IsRegular():
		return nil, ErrNotRegular
	}
	return os.Open(path) //nolint:gosec // The caller names the file to read; it is read, never executed.
}
