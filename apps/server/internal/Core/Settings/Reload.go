package instancesettings

import (
	"context"
	"time"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
)

func (s *Store) Reload(ctx context.Context) {
	backoff := config.SettingsReloadRetryMin
	for attempt := 1; ; attempt++ {
		err := s.Load(ctx)
		if err == nil {
			if attempt > 1 {
				s.log.InfoContext(ctx, "instance settings reloaded after a failure", "attempts", attempt)
			}
			return
		}
		if ctx.Err() != nil {
			return
		}
		s.log.ErrorContext(ctx, "instance settings were not reloaded; readiness fails until a reload succeeds",
			"error", err, "attempt", attempt, "retry_in", backoff.String())
		if !s.wait(ctx, backoff) {
			return
		}
		backoff = min(backoff*2, config.SettingsReloadRetryMax)
	}
}

func (s *Store) Ready(context.Context) error {
	if s.failed.Load() {
		return ErrReloadFailed
	}
	return nil
}

func sleep(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
