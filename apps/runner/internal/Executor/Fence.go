package executor

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	client "github.com/Arab-OpenSource/dhow/apps/runner/internal/Client"
	config "github.com/Arab-OpenSource/dhow/apps/runner/internal/Config"
	runnerv1 "github.com/Arab-OpenSource/dhow/packages/proto/gen/go/dhow/runner/v1"
)

const heartbeatRetry = time.Second

var (
	ErrSelfFenced = errors.New("no heartbeat succeeded in time, so the runner stopped the job before the server could give the lease away")
	ErrLeaseLost  = errors.New("the server says this lease is lost")
)

type fence struct {
	api      API
	ref      *runnerv1.LeaseRef
	log      *slog.Logger
	margin   time.Duration
	interval time.Duration
	onLost   func(error)

	mu       sync.Mutex
	ttl      time.Duration
	deadline time.Time
	timer    *time.Timer
	lostErr  error

	lost       chan struct{}
	lostOnce   sync.Once
	cancel     chan struct{}
	cancelOnce sync.Once
	stop       chan struct{}
	stopOnce   sync.Once
	done       chan struct{}
}

func newFence(api API, ref *runnerv1.LeaseRef, t Timing, leasedAt time.Time, ttl time.Duration, log *slog.Logger, onLost func(error)) *fence {
	f := &fence{
		api: api, ref: ref, log: log, margin: t.SelfFenceMargin, ttl: ttl, onLost: onLost,
		lost: make(chan struct{}), cancel: make(chan struct{}), stop: make(chan struct{}), done: make(chan struct{}),
	}
	f.interval = min(t.HeartbeatInterval, (ttl-f.margin)/config.HeartbeatsPerWindow)
	f.deadline = leasedAt.Add(ttl - f.margin)
	f.timer = time.AfterFunc(time.Until(f.deadline), f.expire)
	return f
}

func (f *fence) expire() {
	f.mu.Lock()
	late := !time.Now().Before(f.deadline)
	if !late {
		f.timer.Reset(time.Until(f.deadline))
	}
	f.mu.Unlock()
	if late {
		f.markLost(ErrSelfFenced)
	}
}

func (f *fence) Deadline() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.deadline
}

func (f *fence) extend(sentAt time.Time, ctrl *runnerv1.LeaseControl) {
	f.observe(ctrl)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.lostErr != nil {
		return
	}
	next := sentAt.Add(f.ttl - f.margin)
	if next.After(f.deadline) {
		f.deadline = next
		f.timer.Reset(time.Until(next))
	}
}

func (f *fence) observe(ctrl *runnerv1.LeaseControl) {
	if ctrl == nil {
		return
	}
	if ctrl.GetCancelRequested() {
		f.cancelOnce.Do(func() { close(f.cancel) })
	}
	ttl := time.Duration(ctrl.GetLeaseTtlSeconds()) * time.Second
	if ttl >= config.MinLeaseTTL && ttl <= config.MaxLeaseTTL {
		f.mu.Lock()
		f.ttl = ttl
		f.mu.Unlock()
	}
}

func (f *fence) markLost(reason error) {
	f.lostOnce.Do(func() {
		f.mu.Lock()
		f.lostErr = reason
		f.timer.Stop()
		f.mu.Unlock()
		close(f.lost)
		if f.onLost != nil {
			f.onLost(reason)
		}
	})
}

func (f *fence) Lost() <-chan struct{} { return f.lost }

func (f *fence) LostErr() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lostErr
}

func (f *fence) Canceled() <-chan struct{} { return f.cancel }

func (f *fence) IsCanceled() bool {
	select {
	case <-f.cancel:
		return true
	default:
		return false
	}
}

func (f *fence) Stop() {
	f.stopOnce.Do(func() {
		close(f.stop)
		f.mu.Lock()
		f.timer.Stop()
		f.mu.Unlock()
	})
	<-f.done
}

func (f *fence) run(ctx context.Context) {
	defer close(f.done)
	wait := f.interval
	for {
		t := time.NewTimer(wait)
		select {
		case <-f.stop:
			t.Stop()
			return
		case <-f.lost:
			t.Stop()
			return
		case <-t.C:
		}
		wait = f.beat(ctx)
	}
}

func (f *fence) beat(ctx context.Context) time.Duration {
	remaining := time.Until(f.Deadline())
	if remaining <= 0 {
		return heartbeatRetry
	}
	hctx, cancel := context.WithTimeout(ctx, min(config.RPCTimeout, remaining))
	defer cancel()
	sent := time.Now()
	ctrl, err := f.api.Heartbeat(hctx, f.ref)
	switch {
	case err == nil:
		f.extend(sent, ctrl)
		return f.interval
	case client.IsLeaseLost(err):
		f.log.Warn("the server says the lease is lost, so the job stops")
		f.markLost(ErrLeaseLost)
		return f.interval
	default:
		f.log.Warn("a heartbeat failed", "code", client.Code(err).String(), "reason", client.Reason(err))
		return min(heartbeatRetry, f.interval)
	}
}
