package ratelimit

import (
	"context"
	"fmt"
	"time"

	sqlc "github.com/Arab-OpenSource/dhow/apps/server/internal/Database/Sqlc"
)

type Postgres struct {
	q   *sqlc.Queries
	now func() time.Time
}

func NewPostgres(db sqlc.DBTX, now func() time.Time) *Postgres {
	return &Postgres{q: sqlc.New(db), now: now}
}

func (p *Postgres) clock() *time.Time {
	if p.now == nil {
		return nil
	}
	t := p.now().UTC()
	return &t
}

func (p *Postgres) Take(ctx context.Context, l Limit, subject string) (Decision, error) {
	key, err := Key(l, subject)
	if err != nil {
		return Decision{}, err
	}
	row, err := p.q.TakeRateLimit(ctx, sqlc.TakeRateLimitParams{
		Key: key, WindowSeconds: l.Window.Seconds(), Now: p.clock(),
		Ceiling: int32(l.Max + 1), //nolint:gosec // Limit.Valid caps Max at MaxHits.
	})
	if err != nil {
		return Decision{}, fmt.Errorf("take rate limit %s: %w", l.Name, err)
	}
	if int(row.Hits) <= l.Max {
		return Decision{Allowed: true}, nil
	}
	return Decision{RetryAfter: clampRetry(row.ExpiresAt.Sub(row.Now), l.Window)}, nil
}
