//go:build dhowtest

package app

import (
	"errors"
	"fmt"

	config "github.com/Arab-OpenSource/dhow/apps/runner/internal/Config"
	sandbox "github.com/Arab-OpenSource/dhow/apps/runner/internal/Sandbox"
	fake "github.com/Arab-OpenSource/dhow/apps/runner/internal/Sandbox/Fake"
)

var ErrDriverMissing = errors.New("is not built into this test build; only the fake driver is")

func newDriver(cfg config.Config) (sandbox.Driver, error) {
	if cfg.Driver != config.DriverFake {
		return nil, fmt.Errorf("%s=%s %w", config.EnvDriver, cfg.Driver, ErrDriverMissing)
	}
	rec, err := fake.Load(cfg.FakeRecording)
	if err != nil {
		return nil, err
	}
	return fake.New(rec, fake.Options{}), nil
}
