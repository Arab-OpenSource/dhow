//go:build dhowtest

package config

import (
	"errors"
	"path/filepath"
)

const EnvFakeRecording = "DHOW_RUNNER_FAKE_RECORDING"

var ErrFakeRecording = errors.New(EnvFakeRecording + " must be an absolute path")

func loadFake(getenv func(string) string, driver string) (string, error) {
	path := getenv(EnvFakeRecording)
	if driver != DriverFake || path == "" {
		return "", nil
	}
	if !filepath.IsAbs(path) {
		return "", ErrFakeRecording
	}
	return filepath.Clean(path), nil
}
