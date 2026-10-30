package unit_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	sources "github.com/Khaymah/dhow/apps/server/internal/Modules/Sources"
)

func TestByteBudget(t *testing.T) {
	b := sources.NewByteBudget(100)
	assert.True(t, b.TryAcquire(60))
	assert.False(t, b.TryAcquire(41))
	assert.True(t, b.TryAcquire(40))
	assert.False(t, b.TryAcquire(1))
	assert.False(t, b.TryAcquire(-1))
	b.Release(60)
	assert.Equal(t, int64(40), b.InUse())
	b.Release(1000)
	assert.Zero(t, b.InUse())

	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			if b.TryAcquire(10) {
				b.Release(10)
			}
		})
	}
	wg.Wait()
	assert.Zero(t, b.InUse())
}
