package jobs

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/google/uuid"

	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
	sqlc "github.com/Arab-OpenSource/dhow/apps/server/internal/Database/Sqlc"
)

type hub struct {
	listen Listener
	log    *slog.Logger

	mu   sync.Mutex
	subs map[uuid.UUID]map[chan struct{}]struct{}
}

func newHub(listen Listener, log *slog.Logger) *hub {
	return &hub{listen: listen, log: log, subs: map[uuid.UUID]map[chan struct{}]struct{}{}}
}

func (h *hub) subscribe(org uuid.UUID) (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	h.mu.Lock()
	if h.subs[org] == nil {
		h.subs[org] = map[chan struct{}]struct{}{}
	}
	h.subs[org][ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		delete(h.subs[org], ch)
		if len(h.subs[org]) == 0 {
			delete(h.subs, org)
		}
	}
}

func (h *hub) wake(org uuid.UUID, all bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for key, chans := range h.subs {
		if !all && key != org {
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
		h.log.WarnContext(ctx, "job queue listener stopped, retrying", "error", err, "retry_in", backoff.String())
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
	if err := sqlc.New(conn).ListenJobsQueued(ctx); err != nil {
		return err
	}
	h.wake(uuid.Nil, true)
	for {
		n, err := conn.WaitForNotification(ctx)
		if err != nil {
			return err
		}
		org, perr := uuid.Parse(n.Payload)
		if perr != nil {
			h.log.WarnContext(ctx, "job queue notification without an organization", "error", perr)
			continue
		}
		h.wake(org, false)
	}
}
