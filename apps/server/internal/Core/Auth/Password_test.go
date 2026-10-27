package auth_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	auth "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Auth"
)

func fastParams() auth.Params {
	return auth.Params{MemoryKiB: 8, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32}
}

func newHasher(ctx context.Context, t *testing.T, p auth.Params) *auth.Hasher {
	t.Helper()
	h, err := auth.NewHasher(ctx, p, 2)
	require.NoError(t, err)
	return h
}

func TestDefaultParamsFollowOWASP(t *testing.T) {
	p := auth.DefaultParams()
	assert.EqualValues(t, 19*1024, p.MemoryKiB)
	assert.EqualValues(t, 2, p.Iterations)
	assert.EqualValues(t, 1, p.Parallelism)
	assert.EqualValues(t, 16, p.SaltLength)
	assert.EqualValues(t, 32, p.KeyLength)
}

func TestHashEncodesPHC(t *testing.T) {
	ctx := t.Context()
	h := newHasher(ctx, t, auth.DefaultParams())
	encoded, err := h.Hash(ctx, "correct horse battery staple")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(encoded, "$argon2id$v=19$m=19456,t=2,p=1$"), encoded)
	assert.Len(t, strings.Split(encoded, "$"), 6)
	assert.NotContains(t, encoded, "correct horse")
}

func TestHashUsesAFreshSalt(t *testing.T) {
	ctx := t.Context()
	h := newHasher(ctx, t, fastParams())
	a, err := h.Hash(ctx, "same password 123")
	require.NoError(t, err)
	b, err := h.Hash(ctx, "same password 123")
	require.NoError(t, err)
	assert.NotEqual(t, a, b)
}

func TestVerify(t *testing.T) {
	ctx := context.Background()
	h := newHasher(ctx, t, fastParams())
	encoded, err := h.Hash(ctx, "correct horse battery staple")
	require.NoError(t, err)
	tests := []struct {
		name       string
		password   string
		encoded    string
		wantOK     bool
		wantErr    bool
		wantRehash bool
	}{
		{"correct password", "correct horse battery staple", encoded, true, false, false},
		{"wrong password", "correct horse battery stapl", encoded, false, false, false},
		{"empty password", "", encoded, false, false, false},
		{"case matters", "Correct horse battery staple", encoded, false, false, false},
		{"not a hash", "x", "plaintext", false, true, false},
		{"wrong algorithm", "x", strings.Replace(encoded, "argon2id", "argon2i", 1), false, true, false},
		{"wrong version", "x", strings.Replace(encoded, "v=19", "v=16", 1), false, true, false},
		{"truncated", "x", encoded[:len(encoded)-10], false, true, false},
		{"absurd memory is refused", "x", "$argon2id$v=19$m=4294967295,t=2,p=1$c2FsdHNhbHRzYWx0c2FsdA$" + strings.Split(encoded, "$")[5], false, true, false},
		{"zero iterations is refused", "x", strings.Replace(encoded, "t=1", "t=0", 1), false, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, rehash, err := h.Verify(ctx, tt.password, tt.encoded)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantRehash, rehash)
			if tt.wantErr {
				assert.ErrorIs(t, err, auth.ErrMalformedHash)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRehashWhenParametersChange(t *testing.T) {
	ctx := context.Background()
	old := newHasher(ctx, t, fastParams())
	encoded, err := old.Hash(ctx, "correct horse battery staple")
	require.NoError(t, err)

	stronger := fastParams()
	stronger.Iterations = 2
	moreMemory := fastParams()
	moreMemory.MemoryKiB = 16
	longerKey := fastParams()
	longerKey.KeyLength = 48
	tests := []struct {
		name   string
		params auth.Params
		want   bool
	}{
		{"same parameters", fastParams(), false},
		{"more iterations", stronger, true},
		{"more memory", moreMemory, true},
		{"longer key", longerKey, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHasher(ctx, t, tt.params)
			ok, rehash, err := h.Verify(ctx, "correct horse battery staple", encoded)
			require.NoError(t, err)
			assert.True(t, ok)
			assert.Equal(t, tt.want, rehash)
			assert.Equal(t, tt.want, h.NeedsRehash(encoded))
		})
	}
	assert.True(t, old.NeedsRehash("garbage"))
}

func TestNewHasherRejectsWeakParameters(t *testing.T) {
	tests := []struct {
		name string
		edit func(*auth.Params)
	}{
		{"memory", func(p *auth.Params) { p.MemoryKiB = 1 }},
		{"iterations", func(p *auth.Params) { p.Iterations = 0 }},
		{"parallelism", func(p *auth.Params) { p.Parallelism = 0 }},
		{"salt", func(p *auth.Params) { p.SaltLength = 4 }},
		{"key", func(p *auth.Params) { p.KeyLength = 8 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := fastParams()
			tt.edit(&p)
			_, err := auth.NewHasher(t.Context(), p, 1)
			assert.Error(t, err)
		})
	}
}

func TestVerifyDummyTakesTheSameCodePath(t *testing.T) {
	ctx := context.Background()
	h := newHasher(ctx, t, auth.DefaultParams())
	encoded, err := h.Hash(ctx, "correct horse battery staple")
	require.NoError(t, err)

	timeIt := func(fn func()) time.Duration {
		best := time.Hour
		for range 3 {
			start := time.Now()
			fn()
			best = min(best, time.Since(start))
		}
		return best
	}
	real := timeIt(func() { _, _, _ = h.Verify(ctx, "wrong wrong wrong", encoded) })
	dummy := timeIt(func() { _ = h.VerifyDummy(ctx, "wrong wrong wrong") })
	assert.Greater(t, dummy, real/2, "the dummy verify must cost about as much as a real one")
	assert.Less(t, dummy, real*2)
}

func TestHashHonoursContextWhileWaitingForASlot(t *testing.T) {
	h, err := auth.NewHasher(t.Context(), auth.DefaultParams(), 1)
	require.NoError(t, err)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = h.Hash(context.Background(), "occupies the only slot")
	}()
	time.Sleep(5 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	_, err = h.Hash(ctx, "waits")
	wg.Wait()
	if err != nil {
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	}
}
