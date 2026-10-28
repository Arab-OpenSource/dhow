package agent

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"

	agentv1 "github.com/Arab-OpenSource/dhow/packages/proto/gen/go/dhow/agent/v1"
)

var (
	ErrSecretsDir  = errors.New("the secrets directory is not a directory the agent owns")
	ErrSecretsName = errors.New("a secret file name must be one path element")
)

type secretFile struct {
	file *os.File
	size int
}

func writeSecretFiles(dir string, files []*agentv1.SecretFile) ([]secretFile, error) {
	if len(files) == 0 {
		return nil, nil
	}
	d, err := openSecretsDir(dir)
	if err != nil {
		return nil, err
	}
	defer func() { _ = d.Close() }()
	if err := clearSecretsDir(d); err != nil {
		return nil, err
	}
	written := make([]secretFile, 0, len(files))
	for _, f := range files {
		s, err := writeSecretFile(d, f)
		if err != nil {
			wipeSecretFiles(dir, written)
			return nil, err
		}
		written = append(written, s)
	}
	return written, nil
}

func openSecretsDir(dir string) (*os.File, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("prepare the secrets directory: %w", err)
	}
	fd, err := unix.Open(dir, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("open the secrets directory: %w", err)
	}
	d := os.NewFile(uintptr(fd), dir)
	var st unix.Stat_t
	if err := unix.Fstat(fd, &st); err != nil {
		_ = d.Close()
		return nil, fmt.Errorf("stat the secrets directory: %w", err)
	}
	if int(st.Uid) != os.Getuid() {
		_ = d.Close()
		return nil, ErrSecretsDir
	}
	if err := unix.Fchmod(fd, 0o700); err != nil {
		_ = d.Close()
		return nil, fmt.Errorf("lock down the secrets directory: %w", err)
	}
	return d, nil
}

func clearSecretsDir(d *os.File) error {
	names, err := d.Readdirnames(-1)
	if err != nil {
		return fmt.Errorf("list the secrets directory: %w", err)
	}
	for _, name := range names {
		if err := os.RemoveAll(filepath.Join(d.Name(), name)); err != nil {
			return fmt.Errorf("clear the secrets directory: %w", err)
		}
	}
	return nil
}

func writeSecretFile(d *os.File, f *agentv1.SecretFile) (secretFile, error) {
	name := f.GetName()
	if name == "" || name == "." || name == ".." || strings.ContainsRune(name, '/') {
		return secretFile{}, ErrSecretsName
	}
	fd, err := unix.Openat(int(d.Fd()), name, unix.O_RDWR|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0o400)
	if err != nil {
		return secretFile{}, fmt.Errorf("create a secret file: %w", err)
	}
	file := os.NewFile(uintptr(fd), name)
	if _, err := file.Write(f.GetData()); err != nil {
		_ = file.Close()
		return secretFile{}, fmt.Errorf("write a secret file: %w", err)
	}
	return secretFile{file: file, size: len(f.GetData())}, nil
}

func wipeSecretFiles(dir string, files []secretFile) {
	if len(files) == 0 {
		return
	}
	for _, s := range files {
		_, _ = s.file.WriteAt(make([]byte, s.size), 0)
		_ = s.file.Close()
	}
	d, err := openSecretsDir(dir)
	if err != nil {
		return
	}
	_ = clearSecretsDir(d)
	_ = d.Close()
}
