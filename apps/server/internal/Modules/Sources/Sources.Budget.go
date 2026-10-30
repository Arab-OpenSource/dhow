package sources

import "sync"

type ByteBudget struct {
	mu       sync.Mutex
	capacity int64
	used     int64
}

func NewByteBudget(capacity int64) *ByteBudget { return &ByteBudget{capacity: capacity} }

func (b *ByteBudget) TryAcquire(n int64) bool {
	if n < 0 {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if n > b.capacity-b.used {
		return false
	}
	b.used += n
	return true
}

func (b *ByteBudget) Release(n int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.used = max(b.used-n, 0)
}

func (b *ByteBudget) InUse() int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.used
}
