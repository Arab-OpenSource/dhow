package config

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

var ErrStateBusy = errors.New("another dhow-runner process uses this state directory")

type StateLock struct{ f *os.File }

func LockState(path string) (*StateLock, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|unix.O_NOFOLLOW|unix.O_CLOEXEC, PrivateFileMode) //nolint:gosec // The runner's own state directory.
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, openErr(err))
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, unix.EWOULDBLOCK) {
			return nil, ErrStateBusy
		}
		return nil, fmt.Errorf("lock %s: %w", path, err)
	}
	return &StateLock{f: f}, nil
}

func (l *StateLock) Release() error {
	if l == nil || l.f == nil {
		return nil
	}
	err := l.f.Close()
	l.f = nil
	return err
}
