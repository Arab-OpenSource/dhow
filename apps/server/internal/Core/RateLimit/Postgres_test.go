package ratelimit_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ratelimit "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/RateLimit"
	dbtest "github.com/Arab-OpenSource/dhow/apps/server/internal/Database/Testing"
)

func take(t *testing.T, l ratelimit.Limiter, limit ratelimit.Limit, subject string) ratelimit.Decision {
	t.Helper()
	d, err := l.Take(context.Background(), limit, subject)
	require.NoError(t, err)
	return d
}

func TestPostgresCountsAFixedWindow(t *testing.T) {
	pool := dbtest.NewPool(t)
	c := newClock()
	c.Advance(2 * time.Minute)
	p := ratelimit.NewPostgres(pool, c.Now)
	l := ratelimit.Limit{Name: "ip-oidc", Max: 3, Window: 10 * time.Minute}
	for i := range 3 {
		assert.True(t, take(t, p, l, "203.0.113.7").Allowed, "request %d", i)
	}
	d := take(t, p, l, "203.0.113.7")
	assert.False(t, d.Allowed)
	assert.Equal(t, 8*time.Minute, d.RetryAfter, "until the window ends")
	assert.True(t, take(t, p, l, "203.0.113.8").Allowed, "another network has its own counter")
	assert.True(t, take(t, p, ratelimit.Limit{Name: "ip-passkey", Max: 3, Window: 10 * time.Minute}, "203.0.113.7").Allowed,
		"another group has its own counter")

	c.Advance(8 * time.Minute)
	assert.True(t, take(t, p, l, "203.0.113.7").Allowed, "a new window starts from zero")

	var hits int
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT "Hits" FROM "RateLimits" WHERE "Key" = 'ip-oidc:203.0.113.7'`).Scan(&hits))
	assert.Equal(t, 1, hits, "one row per key, reset in place")
}

func TestPostgresStopsCountingAboveTheCeiling(t *testing.T) {
	pool := dbtest.NewPool(t)
	p := ratelimit.NewPostgres(pool, newClock().Now)
	l := ratelimit.Limit{Name: "ip-oidc", Max: 2, Window: time.Minute}
	for range 20 {
		take(t, p, l, "x")
	}
	var hits int
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT "Hits" FROM "RateLimits"`).Scan(&hits))
	assert.Equal(t, 3, hits, "denied requests stop at the limit plus one")
}

func TestPostgresClockMovingBackDoesNotResetTheWindow(t *testing.T) {
	pool := dbtest.NewPool(t)
	c := newClock()
	p := ratelimit.NewPostgres(pool, c.Now)
	l := ratelimit.Limit{Name: "ip-oidc", Max: 2, Window: 10 * time.Minute}
	require.True(t, take(t, p, l, "x").Allowed)
	require.True(t, take(t, p, l, "x").Allowed)

	c.Advance(-time.Hour)
	d := take(t, p, l, "x")
	assert.False(t, d.Allowed, "an older window counts into the newest one, it never starts a fresh count")
	assert.Equal(t, 10*time.Minute, d.RetryAfter, "Retry-After is capped at one window")

	behind := ratelimit.NewPostgres(pool, func() time.Time { return c.Now().Add(-5 * time.Minute) })
	assert.False(t, take(t, behind, l, "x").Allowed, "an instance with a slow clock shares the newest window")
}

func TestTwoInstancesShareOneCounter(t *testing.T) {
	pool := dbtest.NewPool(t)
	c := newClock()
	first, second := ratelimit.NewPostgres(pool, c.Now), ratelimit.NewPostgres(pool, c.Now)
	l := ratelimit.Limit{Name: "ip-invitation", Max: 4, Window: time.Minute}
	for i := range 4 {
		instance := first
		if i%2 == 1 {
			instance = second
		}
		require.True(t, take(t, instance, l, "x").Allowed)
	}
	assert.False(t, take(t, first, l, "x").Allowed)
	assert.False(t, take(t, second, l, "x").Allowed)
}

func TestPostgresAllowsExactlyTheLimitUnderConcurrency(t *testing.T) {
	pool := dbtest.NewPool(t)
	p := ratelimit.NewPostgres(pool, newClock().Now)
	l := ratelimit.Limit{Name: "ip-oidc", Max: 7, Window: time.Hour}
	var allowed atomic.Int32
	var wg sync.WaitGroup
	for range 40 {
		wg.Go(func() {
			d, err := p.Take(context.Background(), l, "x")
			assert.NoError(t, err)
			if d.Allowed {
				allowed.Add(1)
			}
		})
	}
	wg.Wait()
	assert.Equal(t, int32(7), allowed.Load())
}

func TestPostgresRefusesAnInvalidLimit(t *testing.T) {
	pool := dbtest.NewPool(t)
	p := ratelimit.NewPostgres(pool, newClock().Now)
	_, err := p.Take(context.Background(), ratelimit.Limit{Name: "x", Max: 0, Window: time.Minute}, "a")
	require.ErrorIs(t, err, ratelimit.ErrInvalidLimit)
	_, err = p.Take(context.Background(), ratelimit.Limit{Name: "x", Max: 1, Window: time.Minute}, "")
	require.ErrorIs(t, err, ratelimit.ErrInvalidKey)
}
