package sandbox

import (
	"context"
	"errors"
	"io"
	"syscall"

	runnerv1 "github.com/Khaymah/dhow/packages/proto/gen/go/dhow/runner/v1"
)

const (
	LabelRunner  = "dhow.runner"
	LabelAttempt = "dhow.attempt"
)

var (
	ErrImagePull    = errors.New("the job image could not be pulled")
	ErrSandbox      = errors.New("the sandbox could not be created")
	ErrShellMissing = errors.New("the job image has no /bin/sh")
	ErrDestroyed    = errors.New("the sandbox is gone")
)

type Resources struct {
	CPUMillis   int64
	MemoryBytes int64
	Pids        int64
	DiskBytes   int64
}

type PrepareSpec struct {
	RunnerID  string
	AttemptID string
	Image     string
	Isolation runnerv1.Isolation
	Resources Resources
	Dir       string
	DiskMode  string
	Egress    []string
}

type StepSpec struct {
	Index   int32
	Name    string
	Script  string
	Env     map[string]string
	Files   map[string][]byte
	Stdout  io.Writer
	Stderr  io.Writer
	Started func()
}

type StepResult struct {
	ExitCode int32
	Reason   runnerv1.FailureReason
	Signal   syscall.Signal
}

type Driver interface {
	Name() string
	Isolation() []runnerv1.Isolation
	Cleanup(ctx context.Context, runnerID string) error
	Prepare(ctx context.Context, spec PrepareSpec) (Sandbox, error)
}

type Sandbox interface {
	Start(ctx context.Context) error
	Exec(ctx context.Context, step StepSpec) (StepResult, error)
	Signal(ctx context.Context, sig syscall.Signal) error
	Kill(ctx context.Context) error
	Destroy(ctx context.Context) error
	Inspect(ctx context.Context) (Inspection, error)
}

type Inspection struct {
	Running   bool
	Destroyed bool
}

func ExitCodeForSignal(sig syscall.Signal) int32 {
	const signalExitBase = 128
	return signalExitBase + int32(sig) //nolint:gosec // Signal numbers are below 65.
}
