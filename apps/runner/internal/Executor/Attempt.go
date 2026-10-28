package executor

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"connectrpc.com/connect"

	client "github.com/Arab-OpenSource/dhow/apps/runner/internal/Client"
	config "github.com/Arab-OpenSource/dhow/apps/runner/internal/Config"
	journal "github.com/Arab-OpenSource/dhow/apps/runner/internal/Journal"
	logs "github.com/Arab-OpenSource/dhow/apps/runner/internal/Logs"
	masking "github.com/Arab-OpenSource/dhow/apps/runner/internal/Masking"
	sandbox "github.com/Arab-OpenSource/dhow/apps/runner/internal/Sandbox"
	runnerv1 "github.com/Arab-OpenSource/dhow/packages/proto/gen/go/dhow/runner/v1"
)

const teardownTimeout = 30 * time.Second

type result struct {
	state    runnerv1.AttemptResult
	reason   runnerv1.FailureReason
	exitCode int32
	silent   bool
}

func succeeded() result { return result{state: runnerv1.AttemptResult_ATTEMPT_RESULT_SUCCEEDED} }

func failed(reason runnerv1.FailureReason, exitCode int32) result {
	return result{state: runnerv1.AttemptResult_ATTEMPT_RESULT_FAILED, reason: reason, exitCode: exitCode}
}

func canceledResult() result {
	return result{state: runnerv1.AttemptResult_ATTEMPT_RESULT_CANCELED, reason: runnerv1.FailureReason_FAILURE_REASON_CANCELED}
}

func timedOut() result {
	return result{state: runnerv1.AttemptResult_ATTEMPT_RESULT_TIMED_OUT, reason: runnerv1.FailureReason_FAILURE_REASON_TIMED_OUT}
}

func lost(reason runnerv1.FailureReason) result {
	return result{state: runnerv1.AttemptResult_ATTEMPT_RESULT_LOST, reason: reason}
}

func gone() result { return result{silent: true} }

type attempt struct {
	e          *Executor
	grant      *runnerv1.LeaseGrant
	grantBytes []byte
	ref        *runnerv1.LeaseRef
	leasedAt   time.Time
	log        *slog.Logger
	dir        string
	masker     *masking.Masker
	pipe       *logs.Pipeline
	fence      *fence
	rpc        context.Context
	stopRPC    context.CancelFunc
	secrets    map[string][]byte

	mu       sync.Mutex
	box      sandbox.Sandbox
	killedAt time.Time
}

func newAttempt(e *Executor, g *runnerv1.LeaseGrant, grantBytes []byte, leasedAt time.Time) *attempt {
	return &attempt{
		e: e, grant: g, grantBytes: grantBytes, leasedAt: leasedAt,
		ref: &runnerv1.LeaseRef{LeaseId: g.GetLeaseId(), FencingToken: g.GetFencingToken()},
		log: e.o.Log.With("lease_id", g.GetLeaseId(), "attempt_id", g.GetAttemptId(), "job_id", g.GetJobId()),
		dir: filepath.Join(e.o.Config.AttemptsDir(), g.GetAttemptId()), masker: masking.New(),
	}
}

func (a *attempt) entry() journal.Entry {
	return journal.Entry{LeaseID: a.ref.GetLeaseId(), AttemptID: a.grant.GetAttemptId(), JobID: a.grant.GetJobId(), FencingToken: a.ref.GetFencingToken()}
}

func (a *attempt) run(ctx context.Context) Outcome {
	a.rpc, a.stopRPC = context.WithCancel(context.WithoutCancel(ctx))
	defer a.stopRPC()
	defer a.masker.Clear()
	ttl := time.Duration(a.grant.GetLeaseTtlSeconds()) * time.Second
	a.fence = newFence(a.e.o.API, a.ref, a.e.o.Timing, a.leasedAt, ttl, a.log, a.onLost)
	go a.fence.run(a.rpc)
	defer a.fence.Stop()
	if err := a.e.o.Journal.Add(a.entry()); err != nil {
		a.log.Error("could not record the lease in the journal, so the job does not run", "error", err)
		return a.report(ctx, failed(diskReason(err), 0))
	}
	if err := config.EnsurePrivateDir(a.dir); err != nil {
		a.log.Error("could not create the attempt directory", "error", err)
		return a.finish(ctx, failed(diskReason(err), 0))
	}
	defer func() { _ = os.RemoveAll(a.dir) }()
	o := logs.Options{
		Appender: appender{api: a.e.o.API, ref: a.ref}, Masker: a.masker, Dir: a.dir, OnControl: a.fence.observe,
		Log: a.log, Backoff: func(n int) time.Duration { return a.e.o.Backoff.Delay(n, nil) },
	}
	if a.e.o.LogOptions != nil {
		a.e.o.LogOptions(&o)
	}
	a.pipe = logs.New(o)
	return a.finish(ctx, a.execute(ctx))
}

func diskReason(err error) runnerv1.FailureReason {
	if errors.Is(err, syscall.ENOSPC) || errors.Is(err, syscall.EDQUOT) {
		return runnerv1.FailureReason_FAILURE_REASON_RUNNER_DISK_FULL
	}
	return runnerv1.FailureReason_FAILURE_REASON_SANDBOX_FAILED
}

func (a *attempt) onLost(reason error) {
	a.mu.Lock()
	box := a.box
	if box != nil && a.killedAt.IsZero() {
		a.killedAt = time.Now()
	}
	a.mu.Unlock()
	if box != nil {
		if err := box.Kill(context.WithoutCancel(a.rpc)); err != nil {
			a.log.Error("could not kill the sandbox", "error", err)
		}
	}
	if errors.Is(reason, ErrSelfFenced) {
		a.log.Warn("no heartbeat reached the server in time, so the sandbox was killed before the lease could expire")
	}
}

func (a *attempt) setBox(box sandbox.Sandbox) bool {
	a.mu.Lock()
	a.box = box
	a.mu.Unlock()
	select {
	case <-a.fence.Lost():
		a.onLost(a.fence.LostErr())
		return false
	default:
		return true
	}
}

func (a *attempt) destroy() {
	a.mu.Lock()
	box := a.box
	a.mu.Unlock()
	if box == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(a.rpc), teardownTimeout)
	defer cancel()
	if err := box.Destroy(ctx); err != nil {
		a.log.Error("could not remove the sandbox", "error", err)
	}
}

func (a *attempt) retry(ctx context.Context, fn func(context.Context) error) error {
	for n := 0; ; n++ {
		err := fn(a.rpc)
		if err == nil || !client.Transient(err) {
			return err
		}
		if werr := a.waitBackoff(ctx, n, err); werr != nil {
			return werr
		}
	}
}

func (a *attempt) lostErr() error {
	if err := a.fence.LostErr(); err != nil {
		return err
	}
	return ErrLeaseLost
}

func (a *attempt) waitBackoff(ctx context.Context, n int, err error) error {
	t := time.NewTimer(a.e.o.Backoff.Delay(n, err))
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-a.fence.Lost():
		return a.lostErr()
	case <-ctx.Done():
		return ctx.Err()
	}
}

type appender struct {
	api API
	ref *runnerv1.LeaseRef
}

func (p appender) Append(ctx context.Context, chunks []*runnerv1.LogChunk) (*runnerv1.AppendResponse, error) {
	res, err := p.api.Append(ctx, p.ref, chunks)
	switch {
	case err == nil:
		return res, nil
	case client.IsLeaseLost(err):
		return nil, errors.Join(logs.ErrLeaseLost, err)
	case client.Code(err) == connect.CodeUnimplemented:
		return nil, errors.Join(logs.ErrUnsupported, err)
	case client.Transient(err):
		return nil, err
	default:
		return nil, errors.Join(logs.ErrRejected, err)
	}
}
