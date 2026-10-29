package installer

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"syscall"
)

const maxShred = 1 << 20

var ErrNotRegular = errors.New("is not a regular file or directory (a symlink or a special file); remove it by hand")

type LinkSafe interface {
	Lstat(path string) (FileInfo, error)
	ChmodNoFollow(path string, mode FileMode) error
	ReadFileNoFollow(path string, limit int64) ([]byte, FileInfo, error)
	ShredNoFollow(path string) error
}

type SafeSystem interface {
	System
	LinkSafe
}

func (i FileInfo) IsRegular() bool { return i.Mode.Type() == 0 }

func (i FileInfo) IsSymlink() bool { return i.Mode&fs.ModeSymlink != 0 }

func infoOf(st fs.FileInfo) FileInfo {
	info := FileInfo{Mode: st.Mode(), Size: st.Size(), IsDir: st.IsDir(), UID: -1, GID: -1}
	if sys, ok := st.Sys().(*syscall.Stat_t); ok {
		info.UID = int(sys.Uid)
		info.GID = int(sys.Gid)
	}
	return info
}

func (Host) Lstat(path string) (FileInfo, error) {
	st, err := os.Lstat(path)
	if err != nil {
		return FileInfo{}, err
	}
	return infoOf(st), nil
}

func openNoFollow(path string, flag int) (*os.File, FileInfo, error) {
	f, err := os.OpenFile(path, flag|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0) //nolint:gosec // Paths the installer owns; O_NOFOLLOW refuses a planted symlink.
	if err != nil {
		if errors.Is(err, syscall.ELOOP) {
			return nil, FileInfo{}, fmt.Errorf("%s %w", path, ErrNotRegular)
		}
		return nil, FileInfo{}, err
	}
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, FileInfo{}, err
	}
	info := infoOf(st)
	if !info.IsRegular() && !info.IsDir {
		_ = f.Close()
		return nil, FileInfo{}, fmt.Errorf("%s %w", path, ErrNotRegular)
	}
	return f, info, nil
}

func (Host) ChmodNoFollow(path string, mode FileMode) error {
	f, _, err := openNoFollow(path, os.O_RDONLY)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	uid, gid, err := ids(mode)
	if err != nil {
		return err
	}
	if err := f.Chown(uid, gid); err != nil {
		return fmt.Errorf("chown %s: %w", path, err)
	}
	if err := f.Chmod(mode.Perm); err != nil {
		return fmt.Errorf("chmod %s: %w", path, err)
	}
	return nil
}

func (Host) ReadFileNoFollow(path string, limit int64) ([]byte, FileInfo, error) {
	f, info, err := openNoFollow(path, os.O_RDONLY)
	if err != nil {
		return nil, FileInfo{}, err
	}
	defer func() { _ = f.Close() }()
	if info.IsDir {
		return nil, FileInfo{}, fmt.Errorf("%s %w", path, ErrNotRegular)
	}
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, FileInfo{}, err
	}
	if int64(len(data)) > limit {
		return nil, FileInfo{}, fmt.Errorf("%s is larger than %d bytes", path, limit)
	}
	return data, info, nil
}

func (Host) ShredNoFollow(path string) error {
	st, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if st.Mode().IsRegular() && st.Size() <= maxShred {
		if err := zeroFile(path); err != nil && !errors.Is(err, ErrNotRegular) {
			return err
		}
	}
	return os.Remove(path)
}

func zeroFile(path string) error {
	f, info, err := openNoFollow(path, os.O_WRONLY)
	if err != nil {
		return err
	}
	if info.IsRegular() {
		if _, err = f.Write(make([]byte, info.Size)); err == nil {
			err = f.Sync()
		}
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return fmt.Errorf("overwrite %s: %w", path, err)
	}
	return nil
}
