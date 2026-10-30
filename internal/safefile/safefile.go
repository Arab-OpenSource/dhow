package safefile

import (
	"errors"
	"io"
)

var (
	ErrSymlink    = errors.New("the path is a symbolic link")
	ErrNotRegular = errors.New("the path is not a regular file")
)

func ReadRegular(path string, limit int64) (src []byte, err error) {
	f, err := open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			src, err = nil, cerr
		}
	}()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, ErrNotRegular
	}
	return io.ReadAll(io.LimitReader(f, limit))
}
