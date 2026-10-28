package e2e_test

import (
	"context"
	"net/http"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Arab-OpenSource/dhow/apps/server/internal/App/Testing"
)

func TestTheReconcilerLeavesALiveLeaseAlone(t *testing.T) {
	w := newWorld(t)
	id := w.job(t, apptest.SimpleSpec(), nil)
	r := w.runner(apptest.SeedRunnerSpec{})
	g := leaseNow(t, r)
	_, err := r.Start(g)
	require.NoError(t, err)
	_, err = w.env.Pool.Exec(context.Background(), `UPDATE "Leases" SET "ExpiresAt" = now() - interval '2 seconds'`)
	require.NoError(t, err)
	assert.Zero(t, w.env.Reconcile().Total(), "a lease inside the 5 second grace is not expired yet")
	assert.Equal(t, "running", w.get(t, id).Job.State)
}

func TestAJobPastItsDeadlineTimesOut(t *testing.T) {
	w := newWorld(t)
	id := w.job(t, apptest.SimpleSpec(), nil)
	r := w.runner(apptest.SeedRunnerSpec{})
	g := leaseNow(t, r)
	_, err := r.Start(g)
	require.NoError(t, err)
	_, err = w.env.Pool.Exec(context.Background(), `UPDATE "Jobs" SET "Deadline" = now() - interval '10 seconds'`)
	require.NoError(t, err)
	assert.Zero(t, w.env.Reconcile().TimedOut, "the runner gets 30 seconds to stop on its own")
	_, err = w.env.Pool.Exec(context.Background(), `UPDATE "Jobs" SET "Deadline" = now() - interval '31 seconds'`)
	require.NoError(t, err)
	assert.Equal(t, 1, w.env.Reconcile().TimedOut)
	v := w.get(t, id)
	assert.Equal(t, "timed_out", v.Job.State)
	assert.Equal(t, "timed_out", v.Attempts[0].State)
	_, err = r.Heartbeat(g.Ref)
	requireRPC(t, err, connect.CodeFailedPrecondition, "LEASE_LOST")
}

func TestAJobNobodyTakesFailsAtItsQueueDeadline(t *testing.T) {
	w := newWorld(t)
	id := w.job(t, apptest.SimpleSpec(), nil)
	_, err := w.env.Pool.Exec(context.Background(),
		`UPDATE "Jobs" SET "QueuedAt" = now() - interval '25 hours', "QueueDeadline" = now() - interval '1 hour'`)
	require.NoError(t, err)
	noLease(t, w.runner(apptest.SeedRunnerSpec{}))
	assert.Equal(t, 1, w.env.Reconcile().QueueTimeouts)
	v := w.get(t, id)
	assert.Equal(t, "failed", v.Job.State)
	assert.Equal(t, "QUEUE_TIMEOUT", v.Job.FailureReason)
}

func TestRevokingARunnerEndsItsLeasesAndRequeuesTheJob(t *testing.T) {
	w := newWorld(t)
	id := w.job(t, apptest.SimpleSpec(), nil)
	token, _ := w.env.CreateRegistrationToken(w.owner, w.org, apptest.RegistrationTokenSpec{})
	r := w.env.ConnectRunner(token)
	g := leaseNow(t, r)
	_, err := r.Start(g)
	require.NoError(t, err)

	res := w.owner.Do(apptest.Request{Method: http.MethodPost, Path: w.path("/runners/" + r.RunnerID + "/revoke")})
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	v := w.get(t, id)
	assert.Equal(t, "queued", v.Job.State, "the runner.revoked event ends the lease without waiting for it to expire")
	assert.Equal(t, "lost", v.Attempts[0].State)
	assert.Equal(t, "RUNNER_REVOKED", v.Attempts[0].FailureReason)

	_, err = r.Lease(context.Background(), 1, 0)
	requireRPC(t, err, connect.CodePermissionDenied, "RUNNER_REVOKED")
	_, err = r.Heartbeat(g.Ref)
	requireRPC(t, err, connect.CodePermissionDenied, "RUNNER_REVOKED")
	assert.Equal(t, id.String(), leaseNow(t, w.runner(apptest.SeedRunnerSpec{})).Grant.GetJobId())
}

func TestTheReconcilerEndsLeasesOfRevokedRunnersTheEventMissed(t *testing.T) {
	w := newWorld(t)
	id := w.job(t, apptest.SimpleSpec(), nil)
	r := w.runner(apptest.SeedRunnerSpec{})
	g := leaseNow(t, r)
	_, err := r.Start(g)
	require.NoError(t, err)
	_, err = w.env.Pool.Exec(context.Background(), `UPDATE "Runners" SET "State" = 'revoked', "RevokedAt" = now() WHERE "Id" = $1`, r.RunnerID)
	require.NoError(t, err)
	assert.Equal(t, 1, w.env.Reconcile().RevokedLeases)
	assert.Equal(t, "queued", w.get(t, id).Job.State)
}

func TestADisabledRunnerFinishesItsJobButGetsNoNewOne(t *testing.T) {
	w := newWorld(t)
	w.job(t, apptest.SimpleSpec(), nil)
	second := w.job(t, apptest.SimpleSpec(), nil)
	r := w.runner(apptest.SeedRunnerSpec{})
	g := leaseNow(t, r)
	_, err := r.Start(g)
	require.NoError(t, err)
	res := w.owner.Do(apptest.Request{Method: http.MethodPatch, Path: w.path("/runners/" + r.RunnerID), Body: map[string]any{"Disabled": true}})
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))

	noLease(t, r)
	_, err = r.Heartbeat(g.Ref)
	require.NoError(t, err)
	require.NoError(t, r.Complete(g, succeeded, 0, noReason))
	assert.Equal(t, "queued", w.get(t, second).Job.State)
}

func TestARunnerNeverHoldsMoreLeasesThanItsCapacity(t *testing.T) {
	w := newWorld(t)
	for range 3 {
		w.job(t, apptest.SimpleSpec(), nil)
	}
	r := w.runner(apptest.SeedRunnerSpec{Capacity: 2})
	leaseNow(t, r)
	leaseNow(t, r)
	noLease(t, r)
	g, err := r.Lease(context.Background(), 0, 0)
	require.NoError(t, err)
	assert.Nil(t, g, "a runner with no free slots gets nothing")
}

func TestLabelsAndIsolationDecideWhoTakesAJob(t *testing.T) {
	w := newWorld(t)
	gpu := apptest.SimpleSpec()
	gpu["Labels"] = []string{"gpu"}
	gpuJob := w.job(t, gpu, nil)
	sandboxed := apptest.SimpleSpec()
	sandboxed["Isolation"] = "gvisor"
	gvisorJob := w.job(t, sandboxed, nil)
	capability := apptest.SimpleSpec()
	capability["Labels"] = []string{"dhow.arch=arm64"}
	w.job(t, capability, nil)

	plain := w.runner(apptest.SeedRunnerSpec{})
	noLease(t, plain)
	gpuRunner := w.runner(apptest.SeedRunnerSpec{Labels: []string{"gpu", "linux"}})
	assert.Equal(t, gpuJob.String(), leaseNow(t, gpuRunner).Grant.GetJobId())
	noLease(t, gpuRunner)
	gvisor := w.runner(apptest.SeedRunnerSpec{Isolation: []string{"container", "gvisor"}})
	assert.Equal(t, gvisorJob.String(), leaseNow(t, gvisor).Grant.GetJobId())
	noLease(t, gvisor)
}
