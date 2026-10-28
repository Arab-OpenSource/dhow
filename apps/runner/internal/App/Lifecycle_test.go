//go:build dhowtest

package app_test

import (
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	runnertest "github.com/Arab-OpenSource/dhow/apps/runner/internal/App/Testing"
	fake "github.com/Arab-OpenSource/dhow/apps/runner/internal/Sandbox/Fake"
	runnerv1 "github.com/Arab-OpenSource/dhow/packages/proto/gen/go/dhow/runner/v1"
	"github.com/Arab-OpenSource/dhow/packages/proto/gen/go/dhow/runner/v1/runnerv1connect"
)

func signals(events []fake.Event) []syscall.Signal {
	var out []syscall.Signal
	for _, e := range events {
		if e.Kind == fake.EventSignal {
			out = append(out, e.Signal)
		}
	}
	return out
}

func TestCancellationTerminatesTheStepGracefully(t *testing.T) {
	srv := runnertest.NewServer(t)
	r := runnertest.Register(t, srv, srv.URL)
	srv.Enqueue(runnertest.Spec(), nil)
	running := r.Start(fake.Recording{Steps: []fake.Step{{Output: []fake.Output{{Data: "working\n"}}, HangMillis: 120_000}}}, runnertest.StartOptions{})
	runnertest.WaitEvent(t, running.Driver, fake.EventExec, finishWithin)
	srv.RequestCancel(srv.Leases()[0].ID)

	out := running.Next(t, finishWithin)
	assert.Equal(t, runnerv1.AttemptResult_ATTEMPT_RESULT_CANCELED, out.Result)
	assert.Equal(t, []syscall.Signal{syscall.SIGTERM}, signals(running.Driver.Events()))
	l := srv.Leases()[0]
	assert.Equal(t, "FAILURE_REASON_CANCELED", l.Steps[0])
	require.Len(t, l.Completes, 1)
	assert.Equal(t, runnerv1.FailureReason_FAILURE_REASON_CANCELED, l.Completes[0].GetFailureReason())
}

func TestAStepThatIgnoresSIGTERMIsKilledAfterTheGrace(t *testing.T) {
	srv := runnertest.NewServer(t)
	r := runnertest.Register(t, srv, srv.URL)
	srv.Enqueue(runnertest.Spec(), nil)
	running := r.Start(fake.Recording{Steps: []fake.Step{{HangMillis: 120_000, IgnoreTerm: true}}}, runnertest.StartOptions{})
	runnertest.WaitEvent(t, running.Driver, fake.EventExec, finishWithin)
	srv.RequestCancel(srv.Leases()[0].ID)
	term := runnertest.WaitEvent(t, running.Driver, fake.EventSignal, finishWithin)
	out := running.Next(t, finishWithin)
	assert.Equal(t, runnerv1.AttemptResult_ATTEMPT_RESULT_CANCELED, out.Result)
	sigs := signals(running.Driver.Events())
	require.Equal(t, []syscall.Signal{syscall.SIGTERM, syscall.SIGKILL}, sigs)
	var kill fake.Event
	for _, e := range running.Driver.Events() {
		if e.Kind == fake.EventSignal && e.Signal == syscall.SIGKILL {
			kill = e
		}
	}
	assert.InDelta(t, 1.0, kill.At.Sub(term.At).Seconds(), 0.3, "SIGKILL follows after the one second grace")
}

func TestTheJobTimeoutEndsTheAttemptTimedOut(t *testing.T) {
	srv := runnertest.NewServer(t)
	r := runnertest.Register(t, srv, srv.URL)
	spec := runnertest.Spec()
	spec.TimeoutSeconds = 1
	srv.Enqueue(spec, nil)
	running := r.Start(fake.Recording{Steps: []fake.Step{{HangMillis: 120_000}}}, runnertest.StartOptions{})
	out := running.Next(t, finishWithin)
	assert.Equal(t, runnerv1.AttemptResult_ATTEMPT_RESULT_TIMED_OUT, out.Result)
	assert.Equal(t, "FAILURE_REASON_TIMED_OUT", srv.Leases()[0].Steps[0])
}

func TestAStepTimeoutFailsTheStep(t *testing.T) {
	srv := runnertest.NewServer(t)
	r := runnertest.Register(t, srv, srv.URL)
	srv.Enqueue(runnertest.Spec(&runnerv1.Step{Name: "slow", Run: "sleep", TimeoutSeconds: 1}), nil)
	running := r.Start(fake.Recording{Steps: []fake.Step{{HangMillis: 120_000}}}, runnertest.StartOptions{})
	out := running.Next(t, finishWithin)
	assert.Equal(t, runnerv1.AttemptResult_ATTEMPT_RESULT_FAILED, out.Result)
	assert.Equal(t, runnerv1.FailureReason_FAILURE_REASON_STEP_FAILED, out.Reason)
	l := srv.Leases()[0]
	assert.Equal(t, "FAILURE_REASON_TIMED_OUT", l.Steps[0])
	assert.Contains(t, logText(l), "ran past its timeout")
}

func TestALeaseTheServerDeclaresLostKillsTheJobWithoutAReport(t *testing.T) {
	srv := runnertest.NewServer(t)
	r := runnertest.Register(t, srv, srv.URL)
	srv.Configure(func(k *runnertest.Knobs) { k.LeaseTTL = 20 * time.Second })
	srv.Enqueue(runnertest.Spec(), nil)
	running := r.Start(fake.Recording{Steps: []fake.Step{{HangMillis: 120_000}}}, runnertest.StartOptions{})
	runnertest.WaitEvent(t, running.Driver, fake.EventExec, finishWithin)
	srv.ExpireLease(srv.Leases()[0].ID)
	runnertest.WaitEvent(t, running.Driver, fake.EventKill, finishWithin)
	out := running.Next(t, finishWithin)
	assert.False(t, out.SelfFenced)
	assert.Empty(t, srv.Leases()[0].Completes)
	assert.Zero(t, srv.Calls(runnerv1connect.LeaseServiceCompleteProcedure))
}

func TestLogShippingProblemsNeverFailTheJob(t *testing.T) {
	cases := map[string]func(*runnertest.Knobs){
		"truncated":      func(k *runnertest.Knobs) { k.LogCapBytes = 20 },
		"unimplemented":  func(k *runnertest.Knobs) { k.AppendUnimplemented = true },
		"flaky storage":  func(k *runnertest.Knobs) { k.AppendUnavailable = 3 },
		"flaky complete": func(k *runnertest.Knobs) { k.CompleteUnavailable = 2 },
	}
	for name, knob := range cases {
		t.Run(name, func(t *testing.T) {
			srv := runnertest.NewServer(t)
			r := runnertest.Register(t, srv, srv.URL)
			srv.Configure(knob)
			srv.Enqueue(runnertest.Spec(), nil)
			var output []fake.Output
			for range 50 {
				output = append(output, fake.Output{Data: "a line of output that keeps going\n"})
			}
			running := r.Start(fake.Recording{Steps: []fake.Step{{Output: output}}}, runnertest.StartOptions{})
			out := running.Next(t, finishWithin)
			assert.Equal(t, runnerv1.AttemptResult_ATTEMPT_RESULT_SUCCEEDED, out.Result)
			assert.True(t, out.Reported)
			l := srv.Leases()[0]
			if name == "flaky storage" || name == "flaky complete" {
				assert.Equal(t, 50, countLines(logText(l), "a line of output"))
			}
		})
	}
}

func countLines(text, needle string) int {
	n := 0
	for i := 0; i+len(needle) <= len(text); i++ {
		if text[i:i+len(needle)] == needle {
			n++
		}
	}
	return n
}
