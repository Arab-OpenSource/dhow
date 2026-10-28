package executor

import (
	"context"
	"errors"
	"log/slog"
	"time"

	client "github.com/Arab-OpenSource/dhow/apps/runner/internal/Client"
	journal "github.com/Arab-OpenSource/dhow/apps/runner/internal/Journal"
	"github.com/Arab-OpenSource/dhow/internal/runnersig"
	runnerv1 "github.com/Arab-OpenSource/dhow/packages/proto/gen/go/dhow/runner/v1"
)

const drainHeadroom = time.Second

func CompleteRequest(attemptID string, ref *runnerv1.LeaseRef, res runnerv1.AttemptResult, exitCode int32, reason runnerv1.FailureReason) *runnerv1.CompleteRequest {
	return &runnerv1.CompleteRequest{
		Lease: ref, Result: res, ExitCode: exitCode, FailureReason: reason,
		Digest: runnersig.CompletionDigest(attemptID, ref.GetFencingToken(), int32(res), exitCode, int32(reason)),
	}
}

func (a *attempt) finish(ctx context.Context, r result) Outcome {
	a.destroy(ctx)
	out := Outcome{
		LeaseID: a.ref.GetLeaseId(), Result: r.state, Reason: r.reason, ExitCode: r.exitCode,
		SelfFenced: errors.Is(a.fence.LostErr(), ErrSelfFenced),
	}
	a.mu.Lock()
	out.KilledAt = a.killedAt
	a.mu.Unlock()
	if a.pipe != nil {
		a.closeLogs(ctx, r, out.SelfFenced)
	}
	if r.silent {
		a.forget()
		return out
	}
	out.Reported = a.complete(ctx, r)
	return out
}

func (a *attempt) closeLogs(ctx context.Context, r result, selfFenced bool) {
	if r.silent || selfFenced || a.stopping() {
		_ = a.pipe.Abort()
		return
	}
	limit := min(a.e.o.Timing.LogDrain, time.Until(a.fence.Deadline())-drainHeadroom)
	dctx, cancel := context.WithTimeout(ctx, max(limit, 0))
	defer cancel()
	if err := a.pipe.Close(dctx); err != nil {
		a.log.Warn("the job log did not reach the server before the attempt closed", "error", a.masked(err))
	}
}

func (a *attempt) report(ctx context.Context, r result) Outcome {
	out := Outcome{LeaseID: a.ref.GetLeaseId(), Result: r.state, Reason: r.reason, ExitCode: r.exitCode}
	out.Reported = a.complete(ctx, r)
	return out
}

func (a *attempt) forget() {
	if err := a.e.o.Journal.Remove(a.ref.GetLeaseId()); err != nil {
		a.log.Error("could not remove the lease from the journal", "error", a.masked(err))
	}
}

func (a *attempt) complete(ctx context.Context, r result) bool {
	window := a.e.o.Timing.CompleteWindow
	if a.stopping() {
		window = a.e.o.Timing.ShutdownGrace
	}
	cctx, cancel := context.WithTimeout(ctx, window)
	defer cancel()
	if err := a.e.o.Journal.SetCompletion(a.ref.GetLeaseId(), journal.Completion{
		Result: int32(r.state), ExitCode: r.exitCode, FailureReason: int32(r.reason),
	}); err != nil {
		a.log.Error("could not record the result in the journal", "error", a.masked(err))
	}
	req := CompleteRequest(a.grant.GetAttemptId(), a.ref, r.state, r.exitCode, r.reason)
	acked, definitive := sendComplete(cctx, a.e.o.API, a.e.o.Backoff, req, a.log)
	if definitive {
		a.forget()
	}
	return acked
}

func sendComplete(ctx context.Context, api API, b client.Backoff, req *runnerv1.CompleteRequest, log *slog.Logger) (bool, bool) {
	for n := 0; ; n++ {
		err := api.Complete(ctx, req)
		switch {
		case err == nil:
			log.Info("reported the attempt result", "result", req.GetResult().String(), "reason", req.GetFailureReason().String())
			return true, true
		case client.IsLeaseLost(err):
			log.Warn("the server had already closed this lease, so the result was not applied", "result", req.GetResult().String())
			return false, true
		case client.Is(err, client.CodeCompletionConflict):
			log.Error("the server recorded a different result for this attempt", "result", req.GetResult().String())
			return false, true
		case !client.Transient(err):
			log.Error("the server refused the attempt result", "code", client.Code(err).String(), "reason", client.Reason(err))
			return false, true
		}
		if b.Sleep(ctx, n, err) != nil {
			log.Warn("could not report the attempt result yet; the journal keeps it for the next start")
			return false, false
		}
	}
}

func (e *Executor) Recover(ctx context.Context) error {
	for _, entry := range e.o.Journal.Entries() {
		ref := &runnerv1.LeaseRef{LeaseId: entry.LeaseID, FencingToken: entry.FencingToken}
		res, exit, reason := runnerv1.AttemptResult_ATTEMPT_RESULT_LOST, int32(0), runnerv1.FailureReason_FAILURE_REASON_RUNNER_RESTARTED
		if c := entry.Completion; c != nil {
			res, exit, reason = runnerv1.AttemptResult(c.Result), c.ExitCode, runnerv1.FailureReason(c.FailureReason)
		}
		log := e.o.Log.With("lease_id", entry.LeaseID, "attempt_id", entry.AttemptID, "job_id", entry.JobID)
		log.Info("reporting an attempt that the last run of the runner left open", "result", res.String())
		_, definitive := sendComplete(ctx, e.o.API, e.o.Backoff, CompleteRequest(entry.AttemptID, ref, res, exit, reason), log)
		if !definitive {
			if err := ctx.Err(); err != nil {
				return err
			}
			continue
		}
		if err := e.o.Journal.Remove(entry.LeaseID); err != nil {
			return err
		}
	}
	return nil
}
