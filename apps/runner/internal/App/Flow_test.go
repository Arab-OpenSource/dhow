//go:build dhowtest

package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	runnertest "github.com/Arab-OpenSource/dhow/apps/runner/internal/App/Testing"
	executor "github.com/Arab-OpenSource/dhow/apps/runner/internal/Executor"
	fake "github.com/Arab-OpenSource/dhow/apps/runner/internal/Sandbox/Fake"
	runnerv1 "github.com/Arab-OpenSource/dhow/packages/proto/gen/go/dhow/runner/v1"
	"github.com/Arab-OpenSource/dhow/packages/proto/gen/go/dhow/runner/v1/runnerv1connect"
)

const finishWithin = 20 * time.Second

func logText(l runnertest.Lease) string {
	var b strings.Builder
	for _, c := range l.Chunks {
		b.Write(c.GetData())
	}
	return b.String()
}

func TestAJobRunsFromLeaseToSuccess(t *testing.T) {
	srv := runnertest.NewServer(t)
	r := runnertest.Register(t, srv, srv.URL)
	srv.Enqueue(runnertest.Spec(&runnerv1.Step{Name: "build", Run: "make"}, &runnerv1.Step{Name: "test", Run: "make test"}), nil)
	running := r.Start(fake.Recording{Steps: []fake.Step{
		{Output: []fake.Output{{Data: "compiling\n"}, {Stream: "stderr", Data: "warning: x\n"}}},
		{Output: []fake.Output{{Data: "ok 12 tests\n"}}},
	}}, runnertest.StartOptions{})

	out := running.Next(t, finishWithin)
	assert.False(t, out.Refused)
	assert.True(t, out.Reported)
	assert.Equal(t, runnerv1.AttemptResult_ATTEMPT_RESULT_SUCCEEDED, out.Result)

	leases := srv.Leases()
	require.Len(t, leases, 1)
	l := leases[0]
	require.Len(t, l.Completes, 1)
	assert.Equal(t, runnerv1.AttemptResult_ATTEMPT_RESULT_SUCCEEDED, l.Completes[0].GetResult())
	assert.True(t, l.Started)
	assert.Equal(t, "FAILURE_REASON_UNSPECIFIED", l.Steps[0])
	assert.Equal(t, "FAILURE_REASON_UNSPECIFIED", l.Steps[1])
	text := logText(l)
	for _, want := range []string{"compiling\n", "warning: x\n", "ok 12 tests\n", "dhow: step 1: build", "dhow: step 2: test"} {
		assert.Contains(t, text, want)
	}
	for i, c := range l.Chunks {
		assert.EqualValues(t, i+1, c.GetSeq(), "chunks arrive in order and without gaps")
	}
	assert.Positive(t, srv.Calls(runnerv1connect.LeaseServiceHeartbeatProcedure)+srv.Calls(runnerv1connect.LeaseServiceStartProcedure))

	entries, err := os.ReadDir(filepath.Join(r.StateDir, "attempts"))
	require.NoError(t, err)
	assert.Empty(t, entries, "the attempt directory is removed at teardown")
	journal, err := os.ReadFile(filepath.Join(r.StateDir, "journal"))
	require.NoError(t, err)
	assert.NotContains(t, string(journal), l.ID, "a reported attempt leaves the journal")
	assert.Contains(t, kinds(running.Driver.Events()), fake.EventDestroy)
}

func kinds(events []fake.Event) []fake.EventKind {
	out := make([]fake.EventKind, 0, len(events))
	for _, e := range events {
		out = append(out, e.Kind)
	}
	return out
}

func TestAFailingStepFailsTheAttemptAndStopsTheRest(t *testing.T) {
	srv := runnertest.NewServer(t)
	r := runnertest.Register(t, srv, srv.URL)
	srv.Enqueue(runnertest.Spec(&runnerv1.Step{Name: "a", Run: "x"}, &runnerv1.Step{Name: "b", Run: "y"}), nil)
	running := r.Start(fake.Recording{Steps: []fake.Step{{ExitCode: 3}, {}}}, runnertest.StartOptions{})

	out := running.Next(t, finishWithin)
	assert.Equal(t, runnerv1.AttemptResult_ATTEMPT_RESULT_FAILED, out.Result)
	assert.Equal(t, runnerv1.FailureReason_FAILURE_REASON_STEP_FAILED, out.Reason)
	assert.EqualValues(t, 3, out.ExitCode)
	l := srv.Leases()[0]
	assert.Equal(t, "FAILURE_REASON_STEP_FAILED", l.Steps[0])
	_, ran := l.Steps[1]
	assert.False(t, ran, "the next step never begins")
	assert.Contains(t, logText(l), "dhow: step 1 exited with code 3")
}

func TestAJobAskingForMoreThanTheRunnerAllowsIsRejectedBeforeStart(t *testing.T) {
	srv := runnertest.NewServer(t)
	r := runnertest.Register(t, srv, srv.URL)
	spec := runnertest.Spec()
	spec.Resources.MemoryBytes = 8 << 30
	srv.Enqueue(spec, nil)
	running := r.Start(fake.Recording{}, runnertest.StartOptions{})

	out := running.Next(t, finishWithin)
	assert.Equal(t, runnerv1.AttemptResult_ATTEMPT_RESULT_FAILED, out.Result)
	assert.Equal(t, runnerv1.FailureReason_FAILURE_REASON_RESOURCE_REQUEST_EXCEEDS_RUNNER, out.Reason)
	l := srv.Leases()[0]
	assert.False(t, l.Started, "Start is never called")
	assert.NotContains(t, kinds(running.Driver.Events()), fake.EventPrepare)
	assert.Contains(t, logText(l), "RESOURCE_REQUEST_EXCEEDS_RUNNER")
}

func TestAnInvalidSpecIsRejected(t *testing.T) {
	srv := runnertest.NewServer(t)
	r := runnertest.Register(t, srv, srv.URL)
	spec := runnertest.Spec()
	spec.Image = "unpinned:latest"
	srv.Enqueue(spec, nil)
	running := r.Start(fake.Recording{}, runnertest.StartOptions{})
	out := running.Next(t, finishWithin)
	assert.Equal(t, runnerv1.FailureReason_FAILURE_REASON_SPEC_REJECTED, out.Reason)
	assert.NotContains(t, kinds(running.Driver.Events()), fake.EventPrepare)
}

func TestImagePullFailureIsAnInfrastructureFailure(t *testing.T) {
	srv := runnertest.NewServer(t)
	r := runnertest.Register(t, srv, srv.URL)
	srv.Enqueue(runnertest.Spec(), nil)
	running := r.Start(fake.Recording{PrepareError: fake.PrepareImagePull}, runnertest.StartOptions{})
	out := running.Next(t, finishWithin)
	assert.Equal(t, runnerv1.FailureReason_FAILURE_REASON_IMAGE_PULL_FAILED, out.Reason)
	assert.False(t, srv.Leases()[0].Started)
}

var _ = executor.Outcome{}
