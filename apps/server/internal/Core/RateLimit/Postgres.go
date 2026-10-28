package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"time"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	sqlc "github.com/Khaymah/dhow/apps/server/internal/Database/Sqlc"
)

type Postgres struct {
	q       *sqlc.Queries
	now     func() time.Time
	mu      sync.Mutex
	blocked map[string]time.Time
	tracked int
}

func NewPostgres(db sqlc.DBTX, now func() time.Time) *Postgres {
	return &Postgres{q: sqlc.New(db), now: now, blocked: map[string]time.Time{}, tracked: config.RateLimitTrackedKeys}
}

func (p *Postgres) clock() *time.Time {
	if p.now == nil {
		return nil
	}
	t := p.now().UTC()
	return &t
}

func (p *Postgres) local() time.Time {
	if p.now == nil {
		return time.Now()
	}
	return p.now()
}

func (p *Postgres) knownBlocked(key string, at time.Time) (time.Duration, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	until, ok := p.blocked[key]
	if !ok {
		return 0, false
	}
	if !at.Before(until) {
		delete(p.blocked, key)
		return 0, false
	}
	return until.Sub(at), true
}

func (p *Postgres) remember(key string, until time.Time, at time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.blocked) >= p.tracked {
		for k, u := range p.blocked {
			if !at.Before(u) {
				delete(p.blocked, k)
			}
		}
	}
	if len(p.blocked) < p.tracked {
		p.blocked[key] = until
	}
}

func (p *Postgres) Blocked() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.blocked)
}

func (p *Postgres) Take(ctx context.Context, l Limit, subject string) (Decision, error) {
	key, err := Key(l, subject)
	if err != nil {
		return Decision{}, err
	}
	at := p.local()
	if wait, ok := p.knownBlocked(key, at); ok {
		return Decision{RetryAfter: clampRetry(wait, l.Window)}, nil
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
	retry := clampRetry(row.ExpiresAt.Sub(row.Now), l.Window)
	p.remember(key, at.Add(retry), at)
	return Decision{RetryAfter: retry}, nil
}
