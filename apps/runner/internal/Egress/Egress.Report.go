package egress

import (
	"fmt"
	"sync"
	"time"
)

const reportWindow = time.Minute

type Reporter struct {
	mu    sync.Mutex
	now   func() time.Time
	seen  map[Decision]time.Time
	write func(line string)
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
	r.seen[d] = now
	r.mu.Unlock()
	r.write(Line(d))
}

func Line(d Decision) string {
	if d.Allowed {
		return fmt.Sprintf("dhow: egress allowed %s:%d", d.Host, d.Port)
	}
	return fmt.Sprintf("dhow: egress denied %s:%d (%s)", d.Host, d.Port, d.Reason)
}
