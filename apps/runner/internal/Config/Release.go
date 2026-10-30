//go:build !dhowtest

package config

import "errors"

var ErrFakeDriver = errors.New("the fake driver exists only in test builds (-tags dhowtest)")

func loadFake(_ func(string) string, driver string) (string, error) {
	if driver == DriverFake {
		return "", ErrFakeDriver
	}
	return "", nil
}

func buildEnv() []string { return nil }
