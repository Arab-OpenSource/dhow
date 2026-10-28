package jobs

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
	sqlc "github.com/Arab-OpenSource/dhow/apps/server/internal/Database/Sqlc"
)

type waiters map[uuid.UUID]map[chan struct{}]struct{}

type hub struct {
	listen Listener
	log    *slog.Logger

	mu   sync.Mutex
	subs waiters
	logs waiters
}

func newHub(listen Listener, log *slog.Logger) *hub {
	return &hub{listen: listen, log: log, subs: waiters{}, logs: waiters{}}
}

func (h *hub) add(set waiters, key uuid.UUID) (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	h.mu.Lock()
	if set[key] == nil {
		set[key] = map[chan struct{}]struct{}{}
	}
	set[key][ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		delete(set[key], ch)
		if len(set[key]) == 0 {
			delete(set, key)
		}
	}
}

func (h *hub) subscribe(org uuid.UUID) (<-chan struct{}, func()) { return h.add(h.subs, org) }

func (h *hub) subscribeLog(attempt uuid.UUID) (<-chan struct{}, func()) {
	return h.add(h.logs, attempt)
}

func (h *hub) signal(set waiters, key uuid.UUID, all bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for k, chans := range set {
		if !all && k != key {
			continue
		}
		for ch := range chans {
			select {
			case ch <- struct{}{}:
			default:
			}
		}
	}
}

func (h *hub) wake(org uuid.UUID, all bool) { h.signal(h.subs, org, all) }

func (h *hub) wakeLog(attempt uuid.UUID, all bool) { h.signal(h.logs, attempt, all) }

func (h *hub) run(ctx context.Context) {
	if h.listen == nil {
		return
	}
	backoff := config.JobListenRetryMin
	for ctx.Err() == nil {
		err := h.serve(ctx)
		if ctx.Err() != nil {
			return
		}
		h.log.WarnContext(ctx, "job listener stopped, retrying", "error", err, "retry_in", backoff.String())
		timer := time.NewTimer(backoff/2 + rand.N(backoff/2+1)) //nolint:gosec // Retry jitter needs no cryptographic randomness.
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		backoff = min(backoff*2, config.JobListenRetryMax)
	}
}

func (h *hub) serve(ctx context.Context) error {
	conn, err := h.listen(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := conn.Close(context.WithoutCancel(ctx)); cerr != nil {
			h.log.WarnContext(ctx, "close job queue listener", "error", cerr)
		}
	}()
	q := sqlc.New(conn)
	if err := q.ListenJobsQueued(ctx); err != nil {
		return err
	}
	if err := q.ListenLogAppended(ctx); err != nil {
		return err
	}
	h.wake(uuid.Nil, true)
	h.wakeLog(uuid.Nil, true)
	for {
		n, err := conn.WaitForNotification(ctx)
		if err != nil {
			return err
		}
		if n.Channel == config.LogAppendedChannel {
			h.logAppended(ctx, n.Payload)
			continue
		}
		org, perr := uuid.Parse(n.Payload)
		if perr != nil {
			h.log.WarnContext(ctx, "job queue notification without an organization", "error", perr)
			continue
		}
		h.wake(org, false)
	}
}

func (h *hub) logAppended(ctx context.Context, payload string) {
	id, _, _ := strings.Cut(payload, ":")
	attempt, err := uuid.Parse(id)
	if err != nil {
		h.log.WarnContext(ctx, "log notification without an attempt", "error", err)
		return
	}
	h.wakeLog(attempt, false)
}
