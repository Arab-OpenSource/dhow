package app

import (
	config "github.com/Arab-OpenSource/dhow/apps/runner/internal/Config"
	runnerv1 "github.com/Arab-OpenSource/dhow/packages/proto/gen/go/dhow/runner/v1"
)

func driverIsolation(cfg config.Config) []runnerv1.Isolation {
	levels := []runnerv1.Isolation{runnerv1.Isolation_ISOLATION_CONTAINER}
	if cfg.Driver == config.DriverPodman && cfg.RunscPath != "" {
		levels = append(levels, runnerv1.Isolation_ISOLATION_GVISOR)
	}
	return levels
}
