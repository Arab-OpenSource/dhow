package e2e_test

import (
	"context"
	"net/http"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Arab-OpenSource/dhow/apps/server/internal/App/Testing"
)

func (w world) cancel(t *testing.T, id uuid.UUID) apptest.Response {
	t.Helper()
	return w.owner.Do(apptest.Request{Method: http.MethodPost, Path: w.path("/jobs/" + id.String() + "/cancel")})
}

func TestCancelingAQueuedJobIsImmediateAndIdempotent(t *testing.T) {
	w := newWorld(t)
	id := w.job(t, apptest.SimpleSpec(), nil)
	res := w.cancel(t, id)
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	var j struct{ State, FailureReason string }
	res.Decode(t, &j)
	assert.Equal(t, "canceled", j.State)
	assert.Equal(t, "CANCELED", j.FailureReason)
	assert.Equal(t, http.StatusOK, w.cancel(t, id).Status, "canceling again is fine")
	noLease(t, w.runner(apptest.SeedRunnerSpec{}))

	var events int
	require.NoError(t, w.env.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM "AuditEvents" WHERE "Action" = 'job.cancel_requested'`).Scan(&events))
	assert.Equal(t, 1, events, "only the first request is audited")
}

func TestCancelingARunningJobReachesTheRunner(t *testing.T) {
	w := newWorld(t)
	id := w.job(t, mutatingSpec(), nil)
	r := w.runner(apptest.SeedRunnerSpec{})
	g := leaseNow(t, r)
	_, err := r.Start(g)
	require.NoError(t, err)
	require.NoError(t, r.BeginStep(g.Ref, 0, false))

	require.Equal(t, http.StatusOK, w.cancel(t, id).Status)
	v := w.get(t, id)
	assert.Equal(t, "running", v.Job.State)
	assert.NotNil(t, v.Job.CancelRequestedAt)

	ctrl, err := r.Heartbeat(g.Ref)
	require.NoError(t, err)
	assert.True(t, ctrl.GetCancelRequested())
	err = r.BeginStep(g.Ref, 1, true)
	requireRPC(t, err, connect.CodeFailedPrecondition, "CANCEL_REQUESTED")
	assert.Nil(t, w.get(t, id).Attempts[0].MutationStartedAt, "no mutating step began after the request")
	require.NoError(t, r.EndStep(g.Ref, 0, 143))
	require.NoError(t, r.Complete(g, canceled, 143, noReason))
	v = w.get(t, id)
	assert.Equal(t, "canceled", v.Job.State)
	assert.Equal(t, "failed", v.Attempts[0].Steps[0].State)
}

func TestCancelBeforeStartKeepsTheSecretsBack(t *testing.T) {
	w := newWorld(t)
	id := w.job(t, apptest.SimpleSpec(), nil)
	r := w.runner(apptest.SeedRunnerSpec{})
	g := leaseNow(t, r)
	require.Equal(t, http.StatusOK, w.cancel(t, id).Status)
	_, err := r.Start(g)
	requireRPC(t, err, connect.CodeFailedPrecondition, "CANCEL_REQUESTED")
	require.NoError(t, r.Complete(g, canceled, 0, noReason))
	assert.Equal(t, "canceled", w.get(t, id).Job.State)
}

func TestACanceledJobWhoseRunnerVanishedEndsCanceledNotQueued(t *testing.T) {
	w := newWorld(t)
	id := w.job(t, apptest.SimpleSpec(), nil)
	r := w.runner(apptest.SeedRunnerSpec{})
	g := leaseNow(t, r)
	_, err := r.Start(g)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, w.cancel(t, id).Status)
	w.env.ExpireLease(g.Ref.GetLeaseId())
	w.env.Reconcile()
	v := w.get(t, id)
	assert.Equal(t, "canceled", v.Job.State)
	assert.Equal(t, "lost", v.Attempts[0].State)
}

func TestCancelingAFinishedJobIsAConflict(t *testing.T) {
	w := newWorld(t)
	id := w.job(t, apptest.SimpleSpec(), nil)
	r := w.runner(apptest.SeedRunnerSpec{})
	g := leaseNow(t, r)
	_, err := r.Start(g)
	require.NoError(t, err)
	require.NoError(t, r.Complete(g, succeeded, 0, noReason))
	res := w.cancel(t, id)
	assert.Equal(t, http.StatusConflict, res.Status)
	require.NotNil(t, res.Error)
	assert.Equal(t, "JOB_FINISHED", res.Error.Code)
	assert.Equal(t, http.StatusNotFound, w.cancel(t, uuid.New()).Status)
}
