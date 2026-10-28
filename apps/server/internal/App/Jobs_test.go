package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	app "github.com/Khaymah/dhow/apps/server/internal/App"
	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
	support "github.com/Khaymah/dhow/apps/server/internal/Modules/System/Test/Support"
)

func TestStartJobsReconcilesBeforeServingAndStopsCleanly(t *testing.T) {
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	id := env.CreateJobOK(owner, org, apptest.SimpleSpec(), nil)
	r := env.SeedConnectedRunner(org, mustUUID(owner.UserID), apptest.SeedRunnerSpec{})
	g := r.MustLease(0)
	env.ExpireLease(g.Ref.GetLeaseId())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stop, err := app.StartJobs(ctx, env.Wired.Jobs, support.DiscardLogger())
	require.NoError(t, err)
	state, _ := env.JobState(id)
	assert.Equal(t, "queued", state, "the boot pass ran before StartJobs returned")

	done := make(chan struct{})
	go func() {
		stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the background job tasks did not stop")
	}
}

func TestStartJobsFailsClosedWhenTheDatabaseIsGone(t *testing.T) {
	env := apptest.New(t)
	env.Pool.Close()
	_, err := app.StartJobs(context.Background(), env.Wired.Jobs, support.DiscardLogger())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prepare job log partitions before serving", "partitions come first, then the reconciler")
}
