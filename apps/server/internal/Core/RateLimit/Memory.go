package ratelimit

import (
	"context"
	"sync"
	"time"
)

type bucket struct {
	tokens float64
	at     time.Time
	window time.Duration
}

type Memory struct {
	mu      sync.Mutex
	tracked int
	now     func() time.Time
	buckets map[string]bucket
}

func NewMemory(tracked int, now func() time.Time) *Memory {
	if now == nil {
		now = time.Now
	}
	return &Memory{tracked: max(tracked, 1), now: now, buckets: map[string]bucket{}}
}

func (m *Memory) Take(_ context.Context, l Limit, subject string) (Decision, error) {
	key, err := Key(l, subject)
	if err != nil {
		return Decision{}, err
	}
	now := m.now()
	interval := l.Window / time.Duration(l.Max)
	capacity := float64(l.Max)
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.buckets[key]
	if !ok {
		b = bucket{tokens: capacity, at: now, window: l.Window}
	}
	if elapsed := now.Sub(b.at); elapsed > 0 {
		b.tokens = min(capacity, b.tokens+float64(elapsed)/float64(interval))
		b.at = now
	}
	if b.tokens < 1 {
		m.buckets[key] = b
		return Decision{RetryAfter: clampRetry(time.Duration((1-b.tokens)*float64(interval)), l.Window)}, nil
	}
	b.tokens--
	m.buckets[key] = b
	if len(m.buckets) > m.tracked {
		m.sweep(now)
	}
	return Decision{Allowed: true}, nil
}

func (m *Memory) sweep(now time.Time) {
	for k, b := range m.buckets {
		if now.Sub(b.at) >= b.window {
			delete(m.buckets, k)
		}
	}
}

func (m *Memory) Tracked() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.buckets)
}
