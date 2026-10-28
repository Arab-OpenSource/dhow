package edge_test

import (
	"context"
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
)

func TestRaceHundredRunnersLeaseOneJob(t *testing.T) {
	w := newWorld(t)
	id := w.job(t, apptest.SimpleSpec())
	const runners = 100
	pool := make([]*apptest.TestRunner, runners)
	for i := range pool {
		pool[i] = w.runner()
	}
	grants := make([]*apptest.Grant, runners)
	errs := make([]error, runners)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i, r := range pool {
		wg.Go(func() {
			<-start
			grants[i], errs[i] = r.Lease(context.Background(), 1, 0)
		})
	}
	close(start)
	wg.Wait()
	won := 0
	for i := range pool {
		require.NoError(t, errs[i])
		if grants[i] != nil {
			won++
			assert.Equal(t, id.String(), grants[i].Grant.GetJobId())
		}
	}
	assert.Equal(t, 1, won, "exactly one runner holds the job")
	assert.Equal(t, 1, count(t, w, `SELECT count(*) FROM "JobAttempts"`))
	assert.Equal(t, 1, count(t, w, `SELECT count(*) FROM "Leases" WHERE "State" = 'active'`))
	assert.Equal(t, 1, count(t, w, `SELECT "AttemptCount" FROM "Jobs" WHERE "Id" = $1`, id))
}

func TestRaceManyRunnersManyJobsRespectCapacity(t *testing.T) {
	w := newWorld(t)
	const jobs, runners, rounds = 12, 8, 3
	for range jobs {
		w.job(t, apptest.SimpleSpec())
	}
	pool := make([]*apptest.TestRunner, runners)
	for i := range pool {
		pool[i] = w.env.SeedConnectedRunner(w.org, w.ownerID, apptest.SeedRunnerSpec{Capacity: 1})
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	leased := map[string]int{}
	for _, r := range pool {
		wg.Go(func() {
			for range rounds {
				g, err := r.Lease(context.Background(), 1, 0)
				assert.NoError(t, err)
				if g != nil {
					mu.Lock()
					leased[g.Grant.GetJobId()]++
					mu.Unlock()
				}
			}
		})
	}
	wg.Wait()
	assert.Len(t, leased, runners, "every runner holds exactly one job at capacity 1")
	for job, n := range leased {
		assert.Equal(t, 1, n, "job %s was leased twice", job)
	}
	assert.Zero(t, count(t, w, `SELECT count(*) FROM (SELECT "RunnerId" FROM "Leases" WHERE "State" = 'active' GROUP BY 1 HAVING count(*) > 1) x`))
}

func TestRaceCompleteHeartbeatAndReconcilerAgreeOnOneOutcome(t *testing.T) {
	tests := []struct {
		name       string
		expire     bool
		wantJob    string
		wantLease  string
		completeOK bool
	}{
		{"a live lease completes", false, "succeeded", "released", true},
		{"an expired lease is reconciled", true, "queued", "expired", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newWorld(t)
			id := w.job(t, apptest.SimpleSpec())
			r := w.runner()
			g := r.MustLease(0)
			_, err := r.Start(g)
			require.NoError(t, err)
			if tt.expire {
				w.env.ExpireLease(g.Ref.GetLeaseId())
			}
			var wg sync.WaitGroup
			var completeErr error
			wg.Go(func() { completeErr = r.Complete(g, succeeded, 0, noReason) })
			for range 3 {
				wg.Go(func() {
					_, herr := r.Heartbeat(g.Ref)
					if herr != nil {
						assert.Equal(t, "LEASE_LOST", rpcMessage(herr))
					}
				})
				wg.Go(func() {
					_, rerr := w.env.Wired.Jobs.Reconcile(context.Background())
					assert.NoError(t, rerr)
				})
			}
			wg.Wait()
			if tt.completeOK {
				require.NoError(t, completeErr)
			} else {
				assert.Equal(t, "LEASE_LOST", rpcMessage(completeErr))
			}
			state, _ := w.env.JobState(id)
			assert.Equal(t, tt.wantJob, state)
			assert.Equal(t, 1, count(t, w, `SELECT count(*) FROM "Leases" WHERE "State" = $1`, tt.wantLease))
			assert.Equal(t, 1, count(t, w, `SELECT count(*) FROM "JobAttempts"`))
		})
	}
}

func TestRaceCancelDuringBeginStep(t *testing.T) {
	w := newWorld(t)
	spec := apptest.SimpleSpec(map[string]any{"Name": "build", "Run": "make"}, map[string]any{"Name": "deploy", "Run": "ship", "Mutating": true})
	id := w.job(t, spec)
	r := w.runner()
	g := r.MustLease(0)
	_, err := r.Start(g)
	require.NoError(t, err)
	require.NoError(t, r.BeginStep(g.Ref, 0, false))
	require.NoError(t, r.EndStep(g.Ref, 0, 0))

	var wg sync.WaitGroup
	var beginErr error
	wg.Go(func() { beginErr = r.BeginStep(g.Ref, 1, true) })
	wg.Go(func() {
		res := w.owner.Do(apptest.Request{Method: http.MethodPost, Path: "/organizations/" + w.org.String() + "/jobs/" + id.String() + "/cancel"})
		assert.Equal(t, http.StatusOK, res.Status)
	})
	wg.Wait()
	mutated := count(t, w, `SELECT count(*) FROM "JobAttempts" WHERE "MutationStartedAt" IS NOT NULL`)
	if beginErr == nil {
		assert.Equal(t, 1, mutated, "the step began before the cancel request")
	} else {
		assert.Equal(t, "CANCEL_REQUESTED", rpcMessage(beginErr))
		assert.Zero(t, mutated, "a refused step never counts as a mutation")
	}
	assert.Equal(t, "CANCEL_REQUESTED", rpcMessage(r.BeginStep(g.Ref, 1, true)), "after the request every step is refused")
}
