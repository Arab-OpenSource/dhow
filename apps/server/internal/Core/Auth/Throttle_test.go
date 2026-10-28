package auth_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	auth "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Auth"
)

func TestLockDuration(t *testing.T) {
	const base, maxLock = 30 * time.Second, 15 * time.Minute
	tests := []struct {
		name     string
		failures int
		free     int
		want     time.Duration
	}{
		{"no failures", 0, 5, 0},
		{"below the free attempts", 4, 5, 0},
		{"the last free attempt locks for the base", 5, 5, 30 * time.Second},
		{"doubles", 6, 5, time.Minute},
		{"doubles again", 7, 5, 2 * time.Minute},
		{"and again", 8, 5, 4 * time.Minute},
		{"and again", 9, 5, 8 * time.Minute},
		{"capped at the maximum", 10, 5, maxLock},
		{"far past the cap", 500, 5, maxLock},
		{"a huge count never overflows", 1 << 30, 5, maxLock},
		{"a stricter account limit", 20, 20, 30 * time.Second},
		{"a free count of zero never locks", 3, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, auth.LockDuration(tt.failures, tt.free, base, maxLock))
		})
	}
}

func TestLockDurationIsMonotonic(t *testing.T) {
	prev := time.Duration(0)
	for failures := range 200 {
		d := auth.LockDuration(failures, 5, 30*time.Second, 15*time.Minute)
		assert.GreaterOrEqual(t, d, prev, "failures=%d", failures)
		prev = d
	}
}

func TestThrottleKeys(t *testing.T) {
	assert.Equal(t, "account:abc", auth.AccountThrottleKey("abc"))
	assert.Equal(t, "ip:203.0.113.7", auth.IPThrottleKey("203.0.113.7"))
	assert.Equal(t, "ip:2001:db8:1:2::/64", auth.IPThrottleKey("2001:db8:1:2::abcd"), "one host usually holds a whole /64")
	assert.Equal(t, auth.IPThrottleKey("2001:db8:1:2::1"), auth.IPThrottleKey("2001:db8:1:2:ffff::1"))
	assert.Equal(t, "ip:203.0.113.7", auth.IPThrottleKey("::ffff:203.0.113.7"))
	assert.Equal(t, "ip:unknown", auth.IPThrottleKey(""))
}
