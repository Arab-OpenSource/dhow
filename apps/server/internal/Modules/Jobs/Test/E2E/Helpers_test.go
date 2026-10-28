package e2e_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	apptest "github.com/Arab-OpenSource/dhow/apps/server/internal/App/Testing"
)

type world struct {
	env     *apptest.Env
	owner   *apptest.Client
	ownerID uuid.UUID
	org     uuid.UUID
}

func newWorld(t *testing.T) world {
	t.Helper()
	return setup(t, apptest.New(t))
}

func newWorldWithSecrets(t *testing.T, values map[string]string) world {
	t.Helper()
	return setup(t, apptest.NewWithSecrets(t, apptest.NewFakeSecrets(values)))
}

func setup(t *testing.T, env *apptest.Env) world {
	t.Helper()
	owner, d := env.SetupAndLogin()
	return world{env: env, owner: owner, ownerID: uuid.MustParse(owner.UserID), org: env.OrganizationID(d.OrgSlug)}
}

func (w world) path(rest string) string { return "/organizations/" + w.org.String() + rest }

func (w world) runner(spec apptest.SeedRunnerSpec) *apptest.TestRunner {
	return w.env.SeedConnectedRunner(w.org, w.ownerID, spec)
}

func (w world) job(t *testing.T, spec apptest.JobSpec, extra map[string]any) uuid.UUID {
	t.Helper()
	return w.env.CreateJobOK(w.owner, w.org, spec, extra)
}

type jobView struct {
	Job struct {
		Id, State, FailureReason, QueuedReason, SpecDigest string
		AttemptCount                                       int
		CancelRequestedAt                                  *string
	}
	Spec     map[string]any
	Attempts []struct {
		Id, State, FailureReason, RunnerId string
		Number                             int
		MutationStartedAt                  *string
		Steps                              []struct {
			Index    int
			State    string
			Mutating bool
			ExitCode *int
		}
	}
}

func (w world) get(t *testing.T, id uuid.UUID) jobView {
	t.Helper()
	res := w.owner.Get(w.path("/jobs/" + id.String()))
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	var v jobView
	res.Decode(t, &v)
	return v
}

func requireRPC(t *testing.T, err error, code connect.Code, message string) {
	t.Helper()
	var ce *connect.Error
	require.True(t, errors.As(err, &ce), "want a connect error, got %v", err)
	require.Equal(t, code, ce.Code(), ce.Message())
	require.Equal(t, message, ce.Message())
}

func leaseNow(t *testing.T, r *apptest.TestRunner) *apptest.Grant {
	t.Helper()
	g, err := r.Lease(context.Background(), 1, 0)
	require.NoError(t, err)
	require.NotNil(t, g, "expected a lease")
	return g
}

func noLease(t *testing.T, r *apptest.TestRunner) {
	t.Helper()
	g, err := r.Lease(context.Background(), 1, 0)
	require.NoError(t, err)
	require.Nil(t, g, "expected no lease")
}
