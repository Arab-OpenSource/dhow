package e2e_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Arab-OpenSource/dhow/apps/server/internal/App/Testing"
	runnerv1 "github.com/Arab-OpenSource/dhow/packages/proto/gen/go/dhow/runner/v1"
)

func TestFailureReasonsMustMatchTheResult(t *testing.T) {
	w := newWorld(t)
	w.job(t, apptest.SimpleSpec(), nil)
	r := w.runner(apptest.SeedRunnerSpec{})
	g := leaseNow(t, r)
	_, err := r.Start(g)
	require.NoError(t, err)
	require.NoError(t, r.BeginStep(g.Ref, 0, false))

	reason := func(v int32) runnerv1.FailureReason { return runnerv1.FailureReason(v) }
	tests := []struct {
		name   string
		result runnerv1.AttemptResult
		reason runnerv1.FailureReason
	}{
		{"succeeded with a reason", succeeded, runnerv1.FailureReason_FAILURE_REASON_STEP_FAILED},
		{"failed because the runner restarted", failed, runnerv1.FailureReason_FAILURE_REASON_RUNNER_RESTARTED},
		{"failed with a cancel reason", failed, runnerv1.FailureReason_FAILURE_REASON_CANCELED},
		{"lost because of memory", lostRes, runnerv1.FailureReason_FAILURE_REASON_OOM_KILLED},
		{"canceled because of a timeout", canceled, runnerv1.FailureReason_FAILURE_REASON_TIMED_OUT},
		{"an unknown reason", failed, reason(99)},
		{"an unknown result", runnerv1.AttemptResult(42), noReason},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := r.Complete(g, tt.result, 1, tt.reason)
			var ce *connect.Error
			require.ErrorAs(t, err, &ce)
			assert.Equal(t, connect.CodeInvalidArgument, ce.Code(), ce.Message())
		})
	}
	for _, bad := range []runnerv1.FailureReason{runnerv1.FailureReason_FAILURE_REASON_LEASE_LOST, reason(99)} {
		_, err := r.Client.Lease.EndStep(context.Background(), connect.NewRequest(&runnerv1.EndStepRequest{Lease: g.Ref, Index: 0, ExitCode: 1, FailureReason: bad}))
		requireRPC(t, err, connect.CodeInvalidArgument, "VALIDATION_FAILED: FailureReason")
	}
	require.NoError(t, r.Complete(g, failed, 137, runnerv1.FailureReason_FAILURE_REASON_OOM_KILLED), "a valid pair still works")
}
