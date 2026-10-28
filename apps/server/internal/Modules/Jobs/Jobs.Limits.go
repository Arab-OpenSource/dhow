package jobs

import (
	"sync"
	"time"

	"github.com/google/uuid"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	apperrors "github.com/Khaymah/dhow/apps/server/internal/Core/Errors"
)

type streamLimiter struct {
	mu      sync.Mutex
	max     int
	perUser int
	perOrg  int
	total   int
	users   map[uuid.UUID]int
	orgs    map[uuid.UUID]int
}

func newStreamLimiter(logs config.Logs) *streamLimiter {
	return &streamLimiter{
		max: logs.MaxStreams, perUser: config.LogStreamsPerUser, perOrg: logs.OrgStreams(),
		users: map[uuid.UUID]int{}, orgs: map[uuid.UUID]int{},
	}
}

func (l *streamLimiter) acquire(user, org uuid.UUID) (func(), error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.total >= l.max || l.users[user] >= l.perUser || l.orgs[org] >= l.perOrg {
		return nil, apperrors.TooManyStreams(config.LogStreamRetryAfter)
	}
	l.total++
	l.users[user]++
	l.orgs[org]++
	var once sync.Once
	return func() {
		once.Do(func() {
			l.mu.Lock()
			defer l.mu.Unlock()
			l.total--
			release(l.users, user)
			release(l.orgs, org)
		})
	}, nil
}

func release(counts map[uuid.UUID]int, key uuid.UUID) {
	if counts[key]--; counts[key] <= 0 {
		delete(counts, key)
	}
}

func (l *streamLimiter) open() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.total
}

type bucket struct {
	tokens float64
	at     time.Time
}

type rateLimiter struct {
	mu      sync.Mutex
	burst   float64
	refill  time.Duration
	tracked int
	buckets map[uuid.UUID]bucket
}

func newRateLimiter(burst int, refill time.Duration) *rateLimiter {
	return &rateLimiter{burst: float64(burst), refill: refill, tracked: config.LogTrackedUsers, buckets: map[uuid.UUID]bucket{}}
}

func (r *rateLimiter) allow(key uuid.UUID, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	b, ok := r.buckets[key]
	if !ok {
		b = bucket{tokens: r.burst, at: now}
	}
	b.tokens = min(r.burst, b.tokens+now.Sub(b.at).Seconds()/r.refill.Seconds())
	b.at = now
	if b.tokens < 1 {
		r.buckets[key] = b
		return apperrors.RateLimited(r.refill)
	}
	b.tokens--
	r.buckets[key] = b
	if len(r.buckets) > r.tracked {
		full := time.Duration(r.burst) * r.refill
		for id, other := range r.buckets {
			if now.Sub(other.at) >= full {
				delete(r.buckets, id)
			}
		}
	}
	return nil
}

func (s *Service) AllowLogRead(user uuid.UUID) error { return s.readRate.allow(user, time.Now()) }
