package session

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

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

func (s *Service) lockedFor(ctx context.Context, now time.Time, keys ...throttleKey) (time.Duration, error) {
	q := s.queries()
	var wait time.Duration
	for _, k := range keys {
		row, err := q.GetThrottle(ctx, k.key)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return 0, err
		}
		if row.LockedUntil != nil && row.LockedUntil.After(now) {
			wait = max(wait, row.LockedUntil.Sub(now))
		}
	}
	return wait, nil
}

func (s *Service) recordFailure(ctx context.Context, now time.Time, keys ...throttleKey) error {
	for _, k := range keys {
		err := dbtx.Run(ctx, s.db, func(q *sqlc.Queries) error {
			failures, err := q.RecordThrottleFailure(ctx, sqlc.RecordThrottleFailureParams{
				Key: k.key, Now: now, ResetBefore: now.Add(-config.ThrottleResetAfter),
			})
			if err != nil {
				return err
			}
			lock := auth.LockDuration(int(failures), k.free, config.ThrottleBaseLock, config.ThrottleMaxLock)
			if lock <= 0 {
				return nil
			}
			return q.SetThrottleLock(ctx, sqlc.SetThrottleLockParams{Key: k.key, LockedUntil: now.Add(lock), Now: now})
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) resetFailures(ctx context.Context, now time.Time, k throttleKey) error {
	return s.queries().ResetThrottle(ctx, sqlc.ResetThrottleParams{Key: k.key, Now: now})
}
