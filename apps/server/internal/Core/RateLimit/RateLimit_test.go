package ratelimit_test

import (
	"context"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apperrors "github.com/Khaymah/dhow/apps/server/internal/Core/Errors"
	ratelimit "github.com/Khaymah/dhow/apps/server/internal/Core/RateLimit"
)

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func newClock() *clock { return &clock{t: time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC)} }

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func TestNetworkGroupsIPv6ByPrefixAndKeepsIPv4Whole(t *testing.T) {
	tests := []struct {
		name string
		addr string
		want string
	}{
		{"ipv4 is one address", "203.0.113.7", "203.0.113.7"},
		{"another ipv4 address is another network", "203.0.113.8", "203.0.113.8"},
		{"ipv4-mapped ipv6 counts as ipv4", "::ffff:203.0.113.7", "203.0.113.7"},
		{"ipv6 is grouped by /64", "2001:db8:1:2:aaaa:bbbb:cccc:dddd", "2001:db8:1:2::/64"},
		{"the same /64 with another interface id", "2001:db8:1:2::1", "2001:db8:1:2::/64"},
		{"a zone is dropped", "fe80::1%eth0", "fe80::/64"},
		{"loopback ipv6", "::1", "::/64"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ratelimit.Network(netip.MustParseAddr(tt.addr)))
			assert.Equal(t, tt.want, ratelimit.NetworkOf(tt.addr))
		})
	}
	assert.NotEqual(t, ratelimit.NetworkOf("2001:db8:1:2::1"), ratelimit.NetworkOf("2001:db8:1:3::1"), "the next /64 is another network")
	assert.Equal(t, ratelimit.UnknownNetwork, ratelimit.Network(netip.Addr{}))
	assert.Equal(t, ratelimit.UnknownNetwork, ratelimit.NetworkOf("not an address"))
}

func TestLimitValidation(t *testing.T) {
	good := ratelimit.Limit{Name: "ip-oidc", Max: 5, Window: time.Minute}
	assert.True(t, good.Valid())
	for name, l := range map[string]ratelimit.Limit{
		"no name":            {Max: 5, Window: time.Minute},
		"upper-case name":    {Name: "Oidc", Max: 5, Window: time.Minute},
		"name with a colon":  {Name: "a:b", Max: 5, Window: time.Minute},
		"zero max":           {Name: "a", Max: 0, Window: time.Minute},
		"max above the cap":  {Name: "a", Max: ratelimit.MaxHits + 1, Window: time.Minute},
		"sub-second window":  {Name: "a", Max: 1, Window: time.Millisecond},
		"zero window":        {Name: "a", Max: 1},
		"a very long name 1": {Name: "a" + strings.Repeat("b", 40), Max: 1, Window: time.Minute},
	} {
		t.Run(name, func(t *testing.T) {
			assert.False(t, l.Valid())
			_, err := ratelimit.Key(l, "x")
			assert.ErrorIs(t, err, ratelimit.ErrInvalidLimit)
		})
	}
	_, err := ratelimit.Key(good, "")
	require.ErrorIs(t, err, ratelimit.ErrInvalidKey)
	_, err = ratelimit.Key(good, strings.Repeat("x", ratelimit.MaxKeyLength))
	require.ErrorIs(t, err, ratelimit.ErrInvalidKey)
	key, err := ratelimit.Key(good, "203.0.113.7")
	require.NoError(t, err)
	assert.Equal(t, "ip-oidc:203.0.113.7", key)
}

func TestDecisionErrIsRateLimitedWithRetryAfter(t *testing.T) {
	require.NoError(t, ratelimit.Decision{Allowed: true}.Err())
	err := ratelimit.Decision{RetryAfter: 42 * time.Second}.Err()
	ae := apperrors.From(err)
	assert.Equal(t, 429, ae.Status)
	assert.Equal(t, apperrors.CodeRateLimited, ae.Code)
	assert.Equal(t, 42*time.Second, ae.RetryAfter)
}

func TestMemoryRefillsAtTheConfiguredRate(t *testing.T) {
	c := newClock()
	m := ratelimit.NewMemory(100, c.Now)
	l := ratelimit.Limit{Name: "stream", Max: 3, Window: 3 * time.Second}
	for i := range 3 {
		d, err := m.Take(context.Background(), l, "user")
		require.NoError(t, err)
		assert.True(t, d.Allowed, "take %d", i)
	}
	d, err := m.Take(context.Background(), l, "user")
	require.NoError(t, err)
	assert.False(t, d.Allowed)
	assert.Equal(t, time.Second, d.RetryAfter)
	other, err := m.Take(context.Background(), l, "someone-else")
	require.NoError(t, err)
	assert.True(t, other.Allowed, "every subject has its own bucket")

	c.Advance(time.Second)
	d, err = m.Take(context.Background(), l, "user")
	require.NoError(t, err)
	assert.True(t, d.Allowed, "one token came back after one interval")
	d, err = m.Take(context.Background(), l, "user")
	require.NoError(t, err)
	assert.False(t, d.Allowed)
}

func TestMemoryIgnoresAClockThatMovesBack(t *testing.T) {
	c := newClock()
	m := ratelimit.NewMemory(100, c.Now)
	l := ratelimit.Limit{Name: "stream", Max: 2, Window: time.Minute}
	for range 2 {
		d, err := m.Take(context.Background(), l, "user")
		require.NoError(t, err)
		require.True(t, d.Allowed)
	}
	c.Advance(-time.Hour)
	d, err := m.Take(context.Background(), l, "user")
	require.NoError(t, err)
	assert.False(t, d.Allowed, "going back in time never refills a bucket")
	c.Advance(time.Hour + 30*time.Second)
	d, err = m.Take(context.Background(), l, "user")
	require.NoError(t, err)
	assert.True(t, d.Allowed, "time moving forward again refills from the newest moment seen")
}

func TestMemorySweepsFullBucketsBeyondTheTrackedCount(t *testing.T) {
	c := newClock()
	m := ratelimit.NewMemory(2, c.Now)
	l := ratelimit.Limit{Name: "stream", Max: 1, Window: time.Second}
	for _, who := range []string{"a", "b"} {
		_, err := m.Take(context.Background(), l, who)
		require.NoError(t, err)
	}
	c.Advance(2 * time.Second)
	_, err := m.Take(context.Background(), l, "c")
	require.NoError(t, err)
	assert.Equal(t, 1, m.Tracked(), "buckets that refilled completely are forgotten")
}

func TestMemoryIsSafeUnderConcurrency(t *testing.T) {
	m := ratelimit.NewMemory(100, newClock().Now)
	l := ratelimit.Limit{Name: "stream", Max: 10, Window: time.Hour}
	var wg sync.WaitGroup
	var mu sync.Mutex
	allowed := 0
	for range 100 {
		wg.Go(func() {
			d, err := m.Take(context.Background(), l, "user")
			assert.NoError(t, err)
			if d.Allowed {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	assert.Equal(t, 10, allowed)
}
