package e2e_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Arab-OpenSource/dhow/apps/server/internal/App/Testing"
	"github.com/Arab-OpenSource/dhow/internal/runnersig"
	runnerv1 "github.com/Arab-OpenSource/dhow/packages/proto/gen/go/dhow/runner/v1"
)

const (
	succeeded = runnerv1.AttemptResult_ATTEMPT_RESULT_SUCCEEDED
	failed    = runnerv1.AttemptResult_ATTEMPT_RESULT_FAILED
	canceled  = runnerv1.AttemptResult_ATTEMPT_RESULT_CANCELED
	lostRes   = runnerv1.AttemptResult_ATTEMPT_RESULT_LOST
	noReason  = runnerv1.FailureReason_FAILURE_REASON_UNSPECIFIED
)

func TestAJobRunsFromLeaseToSuccess(t *testing.T) {
	w := newWorld(t)
	id := w.job(t, apptest.SimpleSpec(), nil)
	r := w.runner(apptest.SeedRunnerSpec{})

	g := leaseNow(t, r)
	assert.Equal(t, id.String(), g.Grant.GetJobId())
	assert.Equal(t, w.org.String(), g.Grant.GetOrganizationId())
	assert.Equal(t, apptest.TestImage, g.Grant.GetSpec().GetImage())
	assert.EqualValues(t, 45, g.Grant.GetLeaseTtlSeconds())
	assert.Positive(t, g.Ref.GetFencingToken())
	assert.Equal(t, "leased", w.get(t, id).Job.State)

	start, err := r.Start(g)
	require.NoError(t, err)
	assert.Nil(t, start.GetSecrets(), "a job without secrets gets no bundle")
	assert.False(t, start.GetControl().GetCancelRequested())
	assert.Equal(t, "running", w.get(t, id).Job.State)

	ctrl, err := r.Heartbeat(g.Ref)
	require.NoError(t, err)
	assert.EqualValues(t, 45, ctrl.GetLeaseTtlSeconds())
	for i := range int32(2) {
		require.NoError(t, r.BeginStep(g.Ref, i, false))
		require.NoError(t, r.BeginStep(g.Ref, i, false), "BeginStep is safe to repeat")
		require.NoError(t, r.EndStep(g.Ref, i, 0))
		require.NoError(t, r.EndStep(g.Ref, i, 0), "EndStep is safe to repeat")
	}
	require.NoError(t, r.Complete(g, succeeded, 0, noReason))

	v := w.get(t, id)
	assert.Equal(t, "succeeded", v.Job.State)
	require.Len(t, v.Attempts, 1)
	assert.Equal(t, "succeeded", v.Attempts[0].State)
	assert.Equal(t, r.RunnerID, v.Attempts[0].RunnerId)
	require.Len(t, v.Attempts[0].Steps, 2)
	for _, s := range v.Attempts[0].Steps {
		assert.Equal(t, "succeeded", s.State)
	}
	var lease string
	require.NoError(t, w.env.Pool.QueryRow(context.Background(), `SELECT "State" FROM "Leases"`).Scan(&lease))
	assert.Equal(t, "released", lease)
}

func TestCompleteIsIdempotentByDigest(t *testing.T) {
	w := newWorld(t)
	w.job(t, apptest.SimpleSpec(), nil)
	r := w.runner(apptest.SeedRunnerSpec{})
	g := leaseNow(t, r)
	_, err := r.Start(g)
	require.NoError(t, err)

	require.NoError(t, r.Complete(g, succeeded, 0, noReason))
	require.NoError(t, r.Complete(g, succeeded, 0, noReason), "the same result again is OK")
	err = r.Complete(g, failed, 2, runnerv1.FailureReason_FAILURE_REASON_STEP_FAILED)
	requireRPC(t, err, connect.CodeAlreadyExists, "COMPLETION_CONFLICT")

	forged := apptest.Digest(g, failed, 2, noReason)
	err = r.CompleteWith(g, succeeded, 0, noReason, forged)
	requireRPC(t, err, connect.CodeInvalidArgument, "VALIDATION_FAILED: Digest")
	err = r.CompleteWith(g, succeeded, 0, noReason, []byte("short"))
	requireRPC(t, err, connect.CodeInvalidArgument, "VALIDATION_FAILED: Digest")
	assert.Len(t, runnersig.GrantDigest(nil), 32)
}

func TestCompleteRejectsImpossibleResults(t *testing.T) {
	w := newWorld(t)
	w.job(t, apptest.SimpleSpec(), nil)
	r := w.runner(apptest.SeedRunnerSpec{})
	g := leaseNow(t, r)

	err := r.Complete(g, succeeded, 0, noReason)
	requireRPC(t, err, connect.CodeFailedPrecondition, "ATTEMPT_NOT_RUNNING")
	err = r.Complete(g, runnerv1.AttemptResult_ATTEMPT_RESULT_UNSPECIFIED, 0, noReason)
	requireRPC(t, err, connect.CodeInvalidArgument, "VALIDATION_FAILED: Result")
	_, err = r.Start(g)
	require.NoError(t, err)
	err = r.Complete(g, succeeded, 1, noReason)
	requireRPC(t, err, connect.CodeInvalidArgument, "VALIDATION_FAILED: Result")
}
