package e2e_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
	runnerv1 "github.com/Khaymah/dhow/packages/proto/gen/go/dhow/runner/v1"
)

func oldRefCalls(r *apptest.TestRunner, g *apptest.Grant) map[string]func() error {
	return map[string]func() error{
		"Start":     func() error { _, err := r.Start(g); return err },
		"Heartbeat": func() error { _, err := r.Heartbeat(g.Ref); return err },
		"BeginStep": func() error { return r.BeginStep(g.Ref, 0, false) },
		"EndStep":   func() error { return r.EndStep(g.Ref, 0, 0) },
		"Complete":  func() error { return r.Complete(g, succeeded, 0, noReason) },
	}
}

func TestAnOldFencingTokenIsRefusedAfterTheJobIsLeasedAgain(t *testing.T) {
	w := newWorld(t)
	id := w.job(t, apptest.SimpleSpec(), nil)
	first := w.runner(apptest.SeedRunnerSpec{})
	second := w.runner(apptest.SeedRunnerSpec{})

	old := leaseNow(t, first)
	_, err := first.Start(old)
	require.NoError(t, err)
	w.env.ExpireLease(old.Ref.GetLeaseId())
	res := w.env.Reconcile()
	assert.Equal(t, 1, res.ExpiredLeases)
	assert.Equal(t, "queued", w.get(t, id).Job.State)

	next := leaseNow(t, second)
	assert.Greater(t, next.Ref.GetFencingToken(), old.Ref.GetFencingToken())
	_, err = second.Start(next)
	require.NoError(t, err)

	for name, call := range oldRefCalls(first, old) {
		t.Run(name, func(t *testing.T) {
			requireRPC(t, call(), connect.CodeFailedPrecondition, "LEASE_LOST")
		})
	}
	stale := &apptest.Grant{Signed: next.Signed, Grant: next.Grant, Ref: &runnerv1.LeaseRef{LeaseId: next.Ref.GetLeaseId(), FencingToken: old.Ref.GetFencingToken()}}
	_, err = second.Heartbeat(stale.Ref)
	requireRPC(t, err, connect.CodeFailedPrecondition, "LEASE_LOST")

	_, err = second.Heartbeat(next.Ref)
	require.NoError(t, err, "the current holder is not affected")
	v := w.get(t, id)
	require.Len(t, v.Attempts, 2)
	assert.Equal(t, "lost", v.Attempts[0].State)
	assert.Equal(t, "LEASE_LOST", v.Attempts[0].FailureReason)
	assert.Equal(t, "running", v.Attempts[1].State)
}

func TestStartHappensOnceAndNeedsProof(t *testing.T) {
	w := newWorld(t)
	w.job(t, apptest.SimpleSpec(), nil)
	r := w.runner(apptest.SeedRunnerSpec{})
	g := leaseNow(t, r)

	_, err := r.Client.Lease.Start(context.Background(), connect.NewRequest(&runnerv1.StartRequest{Lease: g.Ref}))
	requireRPC(t, err, connect.CodeUnauthenticated, "INVALID_RUNNER_CREDENTIALS")
	other := w.runner(apptest.SeedRunnerSpec{})
	_, err = r.Client.Lease.Start(context.Background(), connect.NewRequest(&runnerv1.StartRequest{Lease: g.Ref, PopSignature: other.Proof(g)}))
	requireRPC(t, err, connect.CodeUnauthenticated, "INVALID_RUNNER_CREDENTIALS")

	_, err = r.Start(g)
	require.NoError(t, err)
	_, err = r.Start(g)
	requireRPC(t, err, connect.CodeFailedPrecondition, "ATTEMPT_ALREADY_STARTED")
}

func TestARunnerCannotTouchALeaseOfAnotherRunner(t *testing.T) {
	w := newWorld(t)
	w.job(t, apptest.SimpleSpec(), nil)
	holder := w.runner(apptest.SeedRunnerSpec{})
	g := leaseNow(t, holder)
	_, err := holder.Start(g)
	require.NoError(t, err)

	other := w.env.SeedOrganization("Other", "other")
	foreign := w.env.SeedConnectedRunner(other, w.ownerID, apptest.SeedRunnerSpec{})
	sameOrg := w.runner(apptest.SeedRunnerSpec{})
	for name, r := range map[string]*apptest.TestRunner{"runner of another organization": foreign, "another runner of the organization": sameOrg} {
		for call, fn := range oldRefCalls(r, g) {
			t.Run(name+"/"+call, func(t *testing.T) {
				requireRPC(t, fn(), connect.CodeFailedPrecondition, "LEASE_LOST")
			})
		}
	}
	_, err = holder.Heartbeat(g.Ref)
	assert.NoError(t, err)
}

func TestARunnerOfAnotherOrganizationNeverLeasesTheJob(t *testing.T) {
	w := newWorld(t)
	id := w.job(t, apptest.SimpleSpec(), nil)
	other := w.env.SeedOrganization("Other", "other")
	foreign := w.env.SeedConnectedRunner(other, w.ownerID, apptest.SeedRunnerSpec{})
	noLease(t, foreign)
	g, err := foreign.Lease(context.Background(), 1, 1)
	require.NoError(t, err)
	assert.Nil(t, g)
	assert.Equal(t, "queued", w.get(t, id).Job.State)
	assert.Equal(t, id.String(), leaseNow(t, w.runner(apptest.SeedRunnerSpec{})).Grant.GetJobId())
}

func TestMalformedLeaseRefsAreLost(t *testing.T) {
	w := newWorld(t)
	r := w.runner(apptest.SeedRunnerSpec{})
	for name, ref := range map[string]*runnerv1.LeaseRef{
		"empty": {}, "not a uuid": {LeaseId: "x", FencingToken: 1}, "zero fencing": {LeaseId: "00000000-0000-7000-8000-000000000001"},
		"unknown lease": {LeaseId: "00000000-0000-7000-8000-000000000001", FencingToken: 1},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := r.Heartbeat(ref)
			requireRPC(t, err, connect.CodeFailedPrecondition, "LEASE_LOST")
		})
	}
}
