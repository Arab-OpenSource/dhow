package egress

import (
	"fmt"
	"sync"
	"time"
)

const (
	reportWindow = time.Minute
	maxReported  = 256
)

type Reporter struct {
	mu      sync.Mutex
	now     func() time.Time
	seen    map[Decision]time.Time
	write   func(line string)
	dropped int
	noted   time.Time
}

func NewReporter(now func() time.Time, write func(line string)) *Reporter {
	if now == nil {
		now = time.Now
	}
	return &Reporter{now: now, seen: map[Decision]time.Time{}, write: write}
}

func (r *Reporter) Report(d Decision) {
	r.mu.Lock()
	now := r.now()
	if at, ok := r.seen[d]; ok && now.Sub(at) < reportWindow {
		r.mu.Unlock()
		return
	}
	if len(r.seen) >= maxReported {
		r.prune(now)
	}
	if len(r.seen) >= maxReported {
		r.dropped++
		line := ""
		if now.Sub(r.noted) >= reportWindow {
			line = fmt.Sprintf("dhow: egress made %d more decisions this minute that are not listed", r.dropped)
			r.dropped, r.noted = 0, now
		}
		r.mu.Unlock()
		if line != "" {
			r.write(line)
		}
		return
	}
	r.seen[d] = now
	r.mu.Unlock()
	r.write(Line(d))
}

func (r *Reporter) prune(now time.Time) {
	for d, at := range r.seen {
		if now.Sub(at) >= reportWindow {
			delete(r.seen, d)
		}
	}
}

func Line(d Decision) string {
	if d.Allowed {
		return fmt.Sprintf("dhow: egress allowed %s:%d", d.Host, d.Port)
	}
	return fmt.Sprintf("dhow: egress denied %s:%d (%s)", d.Host, d.Port, d.Reason)
}
