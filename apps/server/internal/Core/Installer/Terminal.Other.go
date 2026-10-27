//go:build !linux

package installer

import "errors"

var errNoTerminal = errors.New("not a terminal")

func IsTerminal(int) bool { return false }

func disableEcho(int) (func(), error) { return nil, errNoTerminal }
