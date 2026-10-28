//go:build chaos

package chaos_test

import (
	"context"
	"net"
	"net/http"
	"sort"
	"sync"
	"time"
)

type tick struct {
	Job     string
	Attempt string
	Token   string
	At      time.Time
}

type sink struct {
	mu    sync.Mutex
	ticks []tick
	port  string
}

func newSink() (*sink, func(), error) {
	s := &sink{}
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		return nil, nil, err
	}
	_, s.port, _ = net.SplitHostPort(ln.Addr().String())
	srv := &http.Server{Handler: http.HandlerFunc(s.record), ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	return s, func() { _ = srv.Close() }, nil
}

func (s *sink) record(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	s.mu.Lock()
	s.ticks = append(s.ticks, tick{Job: q.Get("job"), Attempt: q.Get("attempt"), Token: q.Get("token"), At: time.Now()})
	s.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (s *sink) of(job string) []tick {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []tick
	for _, t := range s.ticks {
		if t.Job == job {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At.Before(out[j].At) })
	return out
}

func (s *sink) overlaps(job string) []string {
	ticks := s.of(job)
	last := map[string]time.Time{}
	first := map[string]time.Time{}
	for _, t := range ticks {
		if _, ok := first[t.Attempt]; !ok {
			first[t.Attempt] = t.At
		}
		last[t.Attempt] = t.At
	}
	var bad []string
	for a, af := range first {
		for b, bf := range first {
			if a != b && af.Before(bf) && last[a].After(bf) {
				bad = append(bad, a+" kept running after "+b+" started")
			}
		}
	}
	return bad
}

func (s *sink) attempts(job string) []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range s.of(job) {
		if !seen[t.Attempt] {
			seen[t.Attempt] = true
			out = append(out, t.Attempt)
		}
	}
	return out
}
