//go:build linux

package installer

import (
	"errors"

	"golang.org/x/sys/unix"
)

var errNoTerminal = errors.New("not a terminal")

func IsTerminal(fd int) bool {
	if fd < 0 {
		return false
	}
	_, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	return err == nil
}

func disableEcho(fd int) (func(), error) {
	if fd < 0 {
		return nil, errNoTerminal
	}
	t, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return nil, errNoTerminal
	}
	old := *t
	t.Lflag &^= unix.ECHO
	if err := unix.IoctlSetTermios(fd, unix.TCSETS, t); err != nil {
		return nil, err
	}
	return func() { _ = unix.IoctlSetTermios(fd, unix.TCSETS, &old) }, nil
}
