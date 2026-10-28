package e2e_test

import (
	"context"
	"net/http"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
)

func TestAnAttemptThatNeverStartsIsReleasedAfterTheStartDeadline(t *testing.T) {
	w := newWorld(t)
	id := w.job(t, apptest.SimpleSpec(), nil)
	r := w.runner(apptest.SeedRunnerSpec{})
	g := leaseNow(t, r)
	_, err := r.Heartbeat(g.Ref)
	require.NoError(t, err, "a runner may heartbeat while it pulls the image")
	assert.Zero(t, w.env.Reconcile().StartTimeouts)

	_, err = w.env.Pool.Exec(context.Background(), `UPDATE "JobAttempts" SET "LeasedAt" = now() - interval '11 minutes'`)
	require.NoError(t, err)
	_, err = r.Heartbeat(g.Ref)
	require.NoError(t, err, "the lease itself is still alive")
	assert.Equal(t, 1, w.env.Reconcile().StartTimeouts)

	v := w.get(t, id)
	assert.Equal(t, "queued", v.Job.State, "a start timeout is retried")
	assert.Equal(t, "lost", v.Attempts[0].State)
	assert.Equal(t, "START_TIMEOUT", v.Attempts[0].FailureReason)
	_, err = r.Heartbeat(g.Ref)
	requireRPC(t, err, connect.CodeFailedPrecondition, "LEASE_LOST")
}

func TestCancelingALeasedJobEndsItWhenTheRunnerNeverStarts(t *testing.T) {
	w := newWorld(t)
	id := w.job(t, apptest.SimpleSpec(), nil)
	r := w.runner(apptest.SeedRunnerSpec{})
	g := leaseNow(t, r)
	require.Equal(t, http.StatusOK, w.cancel(t, id).Status)
	assert.Equal(t, "leased", w.get(t, id).Job.State)
	assert.Zero(t, w.env.Reconcile().CanceledUnstarted, "the runner gets a grace period to report")

	_, err := w.env.Pool.Exec(context.Background(), `UPDATE "Jobs" SET "CancelRequestedAt" = now() - interval '2 minutes'`)
	require.NoError(t, err)
	assert.Equal(t, 1, w.env.Reconcile().CanceledUnstarted)
	v := w.get(t, id)
	assert.Equal(t, "canceled", v.Job.State)
	assert.Equal(t, "canceled", v.Attempts[0].State)
	_, err = r.Heartbeat(g.Ref)
	requireRPC(t, err, connect.CodeFailedPrecondition, "LEASE_LOST")
}
