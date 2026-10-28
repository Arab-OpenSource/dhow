package edge_test

import (
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	apptest "github.com/Arab-OpenSource/dhow/apps/server/internal/App/Testing"
	runnerv1 "github.com/Arab-OpenSource/dhow/packages/proto/gen/go/dhow/runner/v1"
)

const (
	succeeded = runnerv1.AttemptResult_ATTEMPT_RESULT_SUCCEEDED
	noReason  = runnerv1.FailureReason_FAILURE_REASON_UNSPECIFIED
)

type world struct {
	env     *apptest.Env
	owner   *apptest.Client
	ownerID uuid.UUID
	org     uuid.UUID
}

func setup(t *testing.T, env *apptest.Env) world {
	t.Helper()
	owner, d := env.SetupAndLogin()
	return world{env: env, owner: owner, ownerID: uuid.MustParse(owner.UserID), org: env.OrganizationID(d.OrgSlug)}
}

func newWorld(t *testing.T) world {
	t.Helper()
	return setup(t, apptest.New(t))
}

func (w world) runner() *apptest.TestRunner {
	return w.env.SeedConnectedRunner(w.org, w.ownerID, apptest.SeedRunnerSpec{})
}

func (w world) job(t *testing.T, spec apptest.JobSpec) uuid.UUID {
	t.Helper()
	return w.env.CreateJobOK(w.owner, w.org, spec, nil)
}

func rpcMessage(err error) string {
	var ce *connect.Error
	if errors.As(err, &ce) {
		return ce.Message()
	}
	if err != nil {
		return err.Error()
	}
	return ""
}

func count(t *testing.T, w world, sql string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, w.env.Pool.QueryRow(t.Context(), sql, args...).Scan(&n))
	return n
}
