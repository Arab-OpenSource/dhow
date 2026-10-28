//go:build !dhowtest

package app

import (
	"errors"
	"fmt"

	config "github.com/Arab-OpenSource/dhow/apps/runner/internal/Config"
	sandbox "github.com/Arab-OpenSource/dhow/apps/runner/internal/Sandbox"
)

var ErrDriverMissing = errors.New("is not built into this dhow-runner yet; the Podman driver arrives with M3.6 and the Kubernetes driver with M3.11")

func newDriver(cfg config.Config) (sandbox.Driver, error) {
	return nil, fmt.Errorf("%s=%s %w", config.EnvDriver, cfg.Driver, ErrDriverMissing)
}
