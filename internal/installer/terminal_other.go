//go:build !linux

package installer

import "errors"

var ErrNoTerminal = errors.New("not a terminal")

func IsTerminal(int) bool { return false }

func DisableEcho(int) (func(), error) { return nil, ErrNoTerminal }
