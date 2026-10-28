package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
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
	for _, name := range []string{IdentityFileName, JournalFileName} {
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

func ReadPrivateFile(path string) ([]byte, error) {
	if err := CheckPrivateFile(path); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path) //nolint:gosec // The path is the runner's own state file, checked just above.
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return data, nil
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
