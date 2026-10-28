package ratelimit_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ratelimit "github.com/Khaymah/dhow/apps/server/internal/Core/RateLimit"
	dbtest "github.com/Khaymah/dhow/apps/server/internal/Database/Testing"
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

func TestWithoutAClockTheDatabaseDecidesTheWindow(t *testing.T) {
	pool := dbtest.NewPool(t)
	tx, err := pool.Begin(context.Background())
	require.NoError(t, err)
	defer func() { require.NoError(t, tx.Rollback(context.Background())) }()
	p := ratelimit.NewPostgres(tx, nil)
	l := ratelimit.Limit{Name: "ip-oidc", Max: 1, Window: time.Second}
	require.True(t, take(t, p, l, "x").Allowed)
	time.Sleep(1200 * time.Millisecond)
	d := take(t, p, l, "x")
	assert.False(t, d.Allowed, "now() stands still inside a transaction, so only the database clock keeps this in one window")
	assert.Equal(t, time.Second, d.RetryAfter)
}

func TestInstancesWithoutAClockShareTheDatabaseWindow(t *testing.T) {
	pool := dbtest.NewPool(t)
	l := ratelimit.Limit{Name: "ip-oidc", Max: 1, Window: time.Second}
	first, second := ratelimit.NewPostgres(pool, nil), ratelimit.NewPostgres(pool, nil)
	require.True(t, take(t, first, l, "x").Allowed)
	require.False(t, take(t, second, l, "x").Allowed)
	time.Sleep(1100 * time.Millisecond)
	assert.True(t, take(t, second, l, "x").Allowed, "both instances read one clock, so the next window starts for everybody")
}

func TestAKeyKnownToBeOverTheLimitIsRefusedWithoutTheDatabase(t *testing.T) {
	pool := dbtest.NewPool(t)
	c := newClock()
	p := ratelimit.NewPostgres(pool, c.Now)
	l := ratelimit.Limit{Name: "ip-oidc", Max: 2, Window: 10 * time.Minute}
	for range 2 {
		require.True(t, take(t, p, l, "x").Allowed)
	}
	first := take(t, p, l, "x")
	require.False(t, first.Allowed)
	assert.Equal(t, 1, p.Blocked())
	_, err := pool.Exec(context.Background(), `DELETE FROM "RateLimits"`)
	require.NoError(t, err)

	c.Advance(time.Minute)
	d := take(t, p, l, "x")
	assert.False(t, d.Allowed, "the refusal comes from memory: the row is gone")
	assert.Equal(t, first.RetryAfter-time.Minute, d.RetryAfter)
	var rows int
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT count(*) FROM "RateLimits"`).Scan(&rows))
	assert.Zero(t, rows, "nothing was written")
	assert.True(t, take(t, p, l, "y").Allowed, "other keys still go to the database")

	c.Advance(first.RetryAfter)
	assert.True(t, take(t, p, l, "x").Allowed, "after the window the database decides again")
	assert.Zero(t, p.Blocked())
}
