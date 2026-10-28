package e2e_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Arab-OpenSource/dhow/apps/server/internal/App/Testing"
	runnerv1 "github.com/Arab-OpenSource/dhow/packages/proto/gen/go/dhow/runner/v1"
)

func mutatingSpec() apptest.JobSpec {
	return apptest.SimpleSpec(map[string]any{"Name": "build", "Run": "make"}, map[string]any{"Name": "deploy", "Run": "deploy", "Mutating": true})
}

func TestALostAttemptBeforeAnyMutationIsRetried(t *testing.T) {
	w := newWorld(t)
	id := w.job(t, mutatingSpec(), nil)
	r := w.runner(apptest.SeedRunnerSpec{})
	g := leaseNow(t, r)
	_, err := r.Start(g)
	require.NoError(t, err)
	require.NoError(t, r.BeginStep(g.Ref, 0, false))
	w.env.ExpireLease(g.Ref.GetLeaseId())
	w.env.Reconcile()

	v := w.get(t, id)
	assert.Equal(t, "queued", v.Job.State)
	assert.Equal(t, "interrupted", v.Attempts[0].Steps[0].State)
	next := leaseNow(t, r)
	assert.Greater(t, next.Ref.GetFencingToken(), g.Ref.GetFencingToken())
	assert.Equal(t, 2, w.get(t, id).Attempts[1].Number)
}

func TestALostAttemptAfterAMutatingStepBeganIsNeverRetried(t *testing.T) {
	w := newWorld(t)
	id := w.job(t, mutatingSpec(), nil)
	r := w.runner(apptest.SeedRunnerSpec{})
	g := leaseNow(t, r)
	_, err := r.Start(g)
	require.NoError(t, err)
	require.NoError(t, r.BeginStep(g.Ref, 0, false))
	require.NoError(t, r.EndStep(g.Ref, 0, 0))
	err = r.BeginStep(g.Ref, 1, false)
	require.Error(t, err, "the runner must declare the mutating step as mutating")
	require.NoError(t, r.BeginStep(g.Ref, 1, true))
	w.env.ExpireLease(g.Ref.GetLeaseId())
	w.env.Reconcile()

	v := w.get(t, id)
	assert.Equal(t, "lost", v.Job.State)
	assert.Equal(t, "LOST_AFTER_MUTATION", v.Job.FailureReason)
	assert.NotNil(t, v.Attempts[0].MutationStartedAt)
	noLease(t, r)
}

func TestRetriesStopAfterMaxInfraRetries(t *testing.T) {
	w := newWorld(t)
	id := w.job(t, apptest.SimpleSpec(), map[string]any{"MaxInfraRetries": 1})
	r := w.runner(apptest.SeedRunnerSpec{})
	for range 2 {
		g := leaseNow(t, r)
		w.env.ExpireLease(g.Ref.GetLeaseId())
		w.env.Reconcile()
	}
	v := w.get(t, id)
	assert.Equal(t, "lost", v.Job.State)
	assert.Equal(t, "RETRIES_EXHAUSTED", v.Job.FailureReason)
	assert.Equal(t, 2, v.Job.AttemptCount)
	noLease(t, r)
}

func TestARestartedRunnerHandsTheJobBackAtOnce(t *testing.T) {
	w := newWorld(t)
	id := w.job(t, apptest.SimpleSpec(), nil)
	r := w.runner(apptest.SeedRunnerSpec{})
	g := leaseNow(t, r)
	_, err := r.Start(g)
	require.NoError(t, err)
	require.NoError(t, r.Complete(g, lostRes, 0, runnerv1.FailureReason_FAILURE_REASON_RUNNER_RESTARTED))
	v := w.get(t, id)
	assert.Equal(t, "queued", v.Job.State, "no wait for the lease to expire")
	assert.Equal(t, "RUNNER_RESTARTED", v.Attempts[0].FailureReason)
	leaseNow(t, r)
}

func TestFailureOutcomes(t *testing.T) {
	tests := []struct {
		name       string
		start      bool
		result     runnerv1.AttemptResult
		exit       int32
		reason     runnerv1.FailureReason
		wantState  string
		wantReason string
	}{
		{"a failing user step is never retried", true, failed, 2, runnerv1.FailureReason_FAILURE_REASON_STEP_FAILED, "failed", "STEP_FAILED"},
		{"out of memory is the user's", true, failed, 137, runnerv1.FailureReason_FAILURE_REASON_OOM_KILLED, "failed", "OOM_KILLED"},
		{"a missing shell is the user's", true, failed, 127, runnerv1.FailureReason_FAILURE_REASON_SHELL_NOT_FOUND, "failed", "SHELL_NOT_FOUND"},
		{"an image pull failure is retried", false, failed, 0, runnerv1.FailureReason_FAILURE_REASON_IMAGE_PULL_FAILED, "queued", ""},
		{"a sandbox failure is retried", true, failed, 0, runnerv1.FailureReason_FAILURE_REASON_SANDBOX_FAILED, "queued", ""},
		{"a full runner disk is retried", true, failed, 0, runnerv1.FailureReason_FAILURE_REASON_RUNNER_DISK_FULL, "queued", ""},
		{"a timeout reported by the runner", true, runnerv1.AttemptResult_ATTEMPT_RESULT_TIMED_OUT, 0, noReason, "timed_out", "TIMED_OUT"},
		{"cancel before start", false, canceled, 0, noReason, "canceled", "CANCELED"},
		{"lost before start", false, lostRes, 0, runnerv1.FailureReason_FAILURE_REASON_RUNNER_RESTARTED, "queued", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newWorld(t)
			id := w.job(t, apptest.SimpleSpec(), nil)
			r := w.runner(apptest.SeedRunnerSpec{})
			g := leaseNow(t, r)
			if tt.start {
				_, err := r.Start(g)
				require.NoError(t, err)
			}
			require.NoError(t, r.Complete(g, tt.result, tt.exit, tt.reason))
			v := w.get(t, id)
			assert.Equal(t, tt.wantState, v.Job.State)
			assert.Equal(t, tt.wantReason, v.Job.FailureReason)
		})
	}
}

func TestARunnerTooSmallForTheJobDoesNotGetItAgain(t *testing.T) {
	w := newWorld(t)
	id := w.job(t, apptest.SimpleSpec(), nil)
	small := w.runner(apptest.SeedRunnerSpec{})
	g := leaseNow(t, small)
	require.NoError(t, small.Complete(g, failed, 0, runnerv1.FailureReason_FAILURE_REASON_RESOURCE_REQUEST_EXCEEDS_RUNNER))
	assert.Equal(t, "queued", w.get(t, id).Job.State)
	noLease(t, small)
	big := w.runner(apptest.SeedRunnerSpec{})
	assert.Equal(t, id.String(), leaseNow(t, big).Grant.GetJobId())
}
