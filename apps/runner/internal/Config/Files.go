package config

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/unix"
)

const (
	groupOtherBits = 0o077
	groupOtherW    = 0o022
)

var (
	ErrNotDirectory = errors.New("is not a directory")
	ErrNotFile      = errors.New("is not a regular file (symlinks are refused)")
	ErrOpenMode     = errors.New("is readable or writable by other users")
	ErrSharedDir    = errors.New("is writable by other users")
	ErrWrongOwner   = errors.New("belongs to another user")
	ErrTooLarge     = errors.New("is too large for a runner state file")
)

func checkStateDir(dir string) error {
	info, err := os.Lstat(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s %w", dir, ErrNotDirectory)
	}
	if info.Mode().Perm()&groupOtherW != 0 {
		return fmt.Errorf("%s %w (mode %04o); run chmod 700 %s", dir, ErrSharedDir, info.Mode().Perm(), dir)
	}
	if err := checkOwner(dir, info); err != nil {
		return err
	}
	for _, name := range []string{IdentityFileName, PendingFileName, JournalFileName} {
		if err := CheckPrivateFile(filepath.Join(dir, name)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return nil
}

func CheckPrivateFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s %w", path, ErrNotFile)
	}
	if info.Mode().Perm()&groupOtherBits != 0 {
		return fmt.Errorf("%s %w (mode %04o); run chmod 600 %s", path, ErrOpenMode, info.Mode().Perm(), path)
	}
	return checkOwner(path, info)
}

func checkOwner(path string, info fs.FileInfo) error {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}
	if int(st.Uid) != os.Geteuid() {
		return fmt.Errorf("%s %w (uid %d)", path, ErrWrongOwner, st.Uid)
	}
	return nil
}

func EnsurePrivateDir(dir string) error {
	if err := os.MkdirAll(dir, PrivateDirMode); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return fmt.Errorf("read %s: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s %w", dir, ErrNotDirectory)
	}
	if info.Mode().Perm()&groupOtherW != 0 {
		return fmt.Errorf("%s %w (mode %04o)", dir, ErrSharedDir, info.Mode().Perm())
	}
	return checkOwner(dir, info)
}

const maxStateFile = 1 << 20

func ReadPrivateFile(path string) ([]byte, error) {
	dir, name := filepath.Dir(path), filepath.Base(path)
	dirfd, err := unix.Open(dir, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", dir, openErr(err))
	}
	defer func() { _ = unix.Close(dirfd) }()
	var st unix.Stat_t
	if err := unix.Fstat(dirfd, &st); err != nil {
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}
	if st.Mode&groupOtherW != 0 {
		return nil, fmt.Errorf("%s %w (mode %04o)", dir, ErrSharedDir, st.Mode&0o777)
	}
	if int(st.Uid) != os.Geteuid() {
		return nil, fmt.Errorf("%s %w (uid %d)", dir, ErrWrongOwner, st.Uid)
	}
	fd, err := unix.Openat(dirfd, name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, openErr(err))
	}
	f := os.NewFile(uintptr(fd), path)
	defer func() { _ = f.Close() }()
	if err := unix.Fstat(fd, &st); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	switch {
	case st.Mode&unix.S_IFMT != unix.S_IFREG:
		return nil, fmt.Errorf("%s %w", path, ErrNotFile)
	case st.Mode&groupOtherBits != 0:
		return nil, fmt.Errorf("%s %w (mode %04o); run chmod 600 %s", path, ErrOpenMode, st.Mode&0o777, path)
	case int(st.Uid) != os.Geteuid():
		return nil, fmt.Errorf("%s %w (uid %d)", path, ErrWrongOwner, st.Uid)
	}
	data, err := io.ReadAll(io.LimitReader(f, maxStateFile+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if len(data) > maxStateFile {
		return nil, fmt.Errorf("%s %w", path, ErrTooLarge)
	}
	return data, nil
}

func openErr(err error) error {
	if errors.Is(err, unix.ELOOP) || errors.Is(err, unix.ENOTDIR) {
		return errors.Join(ErrNotFile, err)
	}
	return err
}

func WriteFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create a temporary file in %s: %w", dir, err)
	}
	name := tmp.Name()
	keep := false
	defer func() {
		if !keep {
			_ = tmp.Close()
			_ = os.Remove(name)
		}
	}()
	if err := tmp.Chmod(PrivateFileMode); err != nil {
		return fmt.Errorf("chmod %s: %w", name, err)
	}
	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("fsync %s: %w", name, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close %s: %w", name, err)
	}
	if err := os.Rename(name, path); err != nil {
		return fmt.Errorf("rename %s: %w", path, err)
	}
	keep = true
	return syncDir(dir)
}

func RemoveDurable(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return syncDir(filepath.Dir(path))
}

func ShredFile(path string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0) //nolint:gosec // The runner's own state directory.
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open %s: %w", path, openErr(err))
	}
	info, err := f.Stat()
	if err == nil && info.Mode().IsRegular() && info.Size() <= maxStateFile {
		_, err = f.Write(make([]byte, info.Size()))
		if err == nil {
			err = f.Sync()
		}
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("overwrite %s: %w", path, err)
	}
	return RemoveDurable(path)
}

func syncDir(dir string) error {
	d, err := os.Open(dir) //nolint:gosec // The runner's own state directory.
	if err != nil {
		return fmt.Errorf("open %s: %w", dir, err)
	}
	defer func() { _ = d.Close() }()
	if err := d.Sync(); err != nil {
		return fmt.Errorf("fsync %s: %w", dir, err)
	}
	return nil
}
