package executor

import (
	"context"
	"errors"
	"log/slog"
	"time"

	client "github.com/Arab-OpenSource/dhow/apps/runner/internal/Client"
	config "github.com/Arab-OpenSource/dhow/apps/runner/internal/Config"
	identity "github.com/Arab-OpenSource/dhow/apps/runner/internal/Identity"
	journal "github.com/Arab-OpenSource/dhow/apps/runner/internal/Journal"
	logs "github.com/Arab-OpenSource/dhow/apps/runner/internal/Logs"
	sandbox "github.com/Arab-OpenSource/dhow/apps/runner/internal/Sandbox"
	runnerv1 "github.com/Arab-OpenSource/dhow/packages/proto/gen/go/dhow/runner/v1"
)

type API interface {
	Start(ctx context.Context, ref *runnerv1.LeaseRef, proof []byte) (*runnerv1.StartResponse, error)
	Heartbeat(ctx context.Context, ref *runnerv1.LeaseRef) (*runnerv1.LeaseControl, error)
	BeginStep(ctx context.Context, ref *runnerv1.LeaseRef, index int32, mutating bool) (*runnerv1.LeaseControl, error)
	EndStep(ctx context.Context, ref *runnerv1.LeaseRef, index, exitCode int32, reason runnerv1.FailureReason) (*runnerv1.LeaseControl, error)
	Complete(ctx context.Context, req *runnerv1.CompleteRequest) error
	Append(ctx context.Context, ref *runnerv1.LeaseRef, chunks []*runnerv1.LogChunk) (*runnerv1.AppendResponse, error)
}

type Timing struct {
	HeartbeatInterval time.Duration
	SelfFenceMargin   time.Duration
	CancelKillWait    time.Duration
	LogDrain          time.Duration
	CompleteWindow    time.Duration
	ShutdownGrace     time.Duration
	ImagePull         time.Duration
}

func DefaultTiming() Timing {
	return Timing{
		HeartbeatInterval: config.HeartbeatInterval, SelfFenceMargin: config.SelfFenceMargin, CancelKillWait: config.CancelKillWait,
		LogDrain: config.LogDrainTimeout, CompleteWindow: config.CompleteRetryWindow, ShutdownGrace: config.ShutdownGrace,
		ImagePull: config.ImagePullTimeout,
	}
}

type Options struct {
	API        API
	Identity   *identity.Identity
	Journal    *journal.Journal
	Driver     sandbox.Driver
	Config     config.Config
	Log        *slog.Logger
	Backoff    client.Backoff
	Timing     Timing
	LogOptions func(*logs.Options)
	BootClock  func() (time.Duration, bool)
}

type Outcome struct {
	Refused    bool
	LeaseID    string
	Result     runnerv1.AttemptResult
	Reason     runnerv1.FailureReason
	ExitCode   int32
	Reported   bool
	SelfFenced bool
	KilledAt   time.Time
}

type Executor struct {
	o Options
}

func New(o Options) *Executor {
	if o.Log == nil {
		o.Log = slog.New(slog.DiscardHandler)
	}
	if o.Timing == (Timing{}) {
		o.Timing = DefaultTiming()
	}
	if o.Backoff == (client.Backoff{}) {
		o.Backoff = client.DefaultBackoff()
	}
	return &Executor{o: o}
}

func (e *Executor) Run(ctx context.Context, signed *runnerv1.SignedLeaseGrant, leasedAt time.Time) Outcome {
	grant, err := VerifyGrant(e.o.Identity, signed, time.Now())
	if err != nil {
		e.o.Log.Error("refused a lease grant and did not run it", "error", err)
		return Outcome{Refused: true}
	}
	a := newAttempt(e, grant, signed.GetGrantBytes(), leasedAt)
	err = e.o.Journal.Add(a.entry())
	if errors.Is(err, journal.ErrReplay) || errors.Is(err, journal.ErrExists) {
		e.o.Log.Error("refused a lease grant that replays an earlier one and did not run it", "error", err,
			"fencing_token", grant.GetFencingToken(), "high_water", e.o.Journal.HighWater())
		return Outcome{Refused: true}
	}
	return a.run(ctx, err)
}
