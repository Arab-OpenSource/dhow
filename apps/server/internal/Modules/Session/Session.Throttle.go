package session

import (
	"context"
	"time"

	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
	auth "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Auth"
	sqlc "github.com/Arab-OpenSource/dhow/apps/server/internal/Database/Sqlc"
	dbtx "github.com/Arab-OpenSource/dhow/apps/server/internal/Database/Tx"
)

type throttleKey struct {
	key  string
	free int
}

func accountKey(email string) throttleKey {
	return throttleKey{key: auth.AccountThrottleKey(emailDigest(email)), free: config.AccountFreeAttempts}
}

func ipKey(ip string) throttleKey {
	return throttleKey{key: auth.IPThrottleKey(ip), free: config.IPFreeAttempts}
}

func (s *Service) reserve(ctx context.Context, now time.Time, keys ...throttleKey) (time.Duration, error) {
	var wait time.Duration
	err := dbtx.Run(ctx, s.db, func(q *sqlc.Queries) error {
		rows := make([]sqlc.LockThrottleRow, 0, len(keys))
		for _, k := range keys {
			if err := q.EnsureThrottle(ctx, sqlc.EnsureThrottleParams{Key: k.key, Now: now}); err != nil {
				return err
			}
			row, err := q.LockThrottle(ctx, k.key)
			if err != nil {
				return err
			}
			if row.LockedUntil != nil && row.LockedUntil.After(now) {
				wait = row.LockedUntil.Sub(now)
				return nil
			}
			rows = append(rows, row)
		}
		for i, k := range keys {
			failures := rows[i].Failures + 1
			if rows[i].LastFailureAt.Before(now.Add(-config.ThrottleResetAfter)) {
				failures = 1
			}
			var until *time.Time
			if lock := auth.LockDuration(int(failures), k.free, config.ThrottleBaseLock, config.ThrottleMaxLock); lock > 0 {
				t := now.Add(lock)
				until = &t
			}
			err := q.UpdateThrottle(ctx, sqlc.UpdateThrottleParams{Key: k.key, Failures: failures, Now: now, LockedUntil: until})
			if err != nil {
				return err
			}
		}
		return nil
	})
	return wait, err
}

func (s *Service) settleSuccess(ctx context.Context, now time.Time, account, ip throttleKey) error {
	return dbtx.Run(ctx, s.db, func(q *sqlc.Queries) error {
		if err := q.ResetThrottle(ctx, sqlc.ResetThrottleParams{Key: account.key, Now: now}); err != nil {
			return err
		}
		return q.RefundThrottle(ctx, sqlc.RefundThrottleParams{Key: ip.key, Free: int32(ip.free), Now: now}) //nolint:gosec // ip.free is a small constant.
	})
}

func (s *Service) resetFailures(ctx context.Context, now time.Time, k throttleKey) error {
	return s.queries().ResetThrottle(ctx, sqlc.ResetThrottleParams{Key: k.key, Now: now})
}
