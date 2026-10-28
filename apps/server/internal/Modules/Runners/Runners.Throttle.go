package runners

import (
	"context"
	"time"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	auth "github.com/Khaymah/dhow/apps/server/internal/Core/Auth"
	sqlc "github.com/Khaymah/dhow/apps/server/internal/Database/Sqlc"
	dbtx "github.com/Khaymah/dhow/apps/server/internal/Database/Tx"
)

type throttleKey struct {
	key  string
	free int
}

func registerKey(ip string) throttleKey {
	return throttleKey{key: "register:" + auth.IPThrottleKey(ip), free: config.RunnerRegisterFreeAttempts}
}

func authenticateKey(ip string) throttleKey {
	return throttleKey{key: "authenticate:" + auth.IPThrottleKey(ip), free: config.RunnerAuthFreeAttempts}
}

func (s *Service) reserve(ctx context.Context, now time.Time, k throttleKey) (time.Duration, error) {
	var wait time.Duration
	err := dbtx.Run(ctx, s.db, func(q *sqlc.Queries) error {
		if err := q.EnsureRunnerThrottle(ctx, sqlc.EnsureRunnerThrottleParams{Key: k.key, Now: now}); err != nil {
			return err
		}
		row, err := q.LockRunnerThrottle(ctx, k.key)
		if err != nil {
			return err
		}
		if row.LockedUntil != nil && row.LockedUntil.After(now) {
			wait = row.LockedUntil.Sub(now)
			return nil
		}
		failures := row.Failures + 1
		if row.LastFailureAt.Before(now.Add(-config.ThrottleResetAfter)) {
			failures = 1
		}
		var until *time.Time
		if lock := auth.LockDuration(int(failures), k.free, config.ThrottleBaseLock, config.ThrottleMaxLock); lock > 0 {
			t := now.Add(lock)
			until = &t
		}
		return q.UpdateRunnerThrottle(ctx, sqlc.UpdateRunnerThrottleParams{Key: k.key, Failures: failures, Now: now, LockedUntil: until})
	})
	return wait, err
}

func (s *Service) refund(ctx context.Context, now time.Time, k throttleKey) {
	err := s.queries().RefundRunnerThrottle(ctx, sqlc.RefundRunnerThrottleParams{Key: k.key, Free: int32(k.free), Now: now}) //nolint:gosec // k.free is a small constant.
	if err != nil {
		s.log.WarnContext(ctx, "refund runner throttle", "error", err)
	}
}
