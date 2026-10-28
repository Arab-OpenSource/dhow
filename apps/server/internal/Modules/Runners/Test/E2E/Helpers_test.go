package e2e_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
	runnerv1 "github.com/Khaymah/dhow/packages/proto/gen/go/dhow/runner/v1"
)

type world struct {
	env   *apptest.Env
	owner *apptest.Client
	org   uuid.UUID
}

func newWorld(t *testing.T) world {
	t.Helper()
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	return world{env: env, owner: owner, org: env.OrganizationID(d.OrgSlug)}
}

func (w world) path(rest string) string { return "/organizations/" + w.org.String() + rest }

func (w world) register(token string, id *apptest.RunnerIdentity) (*runnerv1.RegisterResponse, error) {
	res, err := w.env.RunnerClient(apptest.RunnerClientOptions{}).Registration.Register(context.Background(), connect.NewRequest(id.RegisterRequest(token)))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (w world) registerOK(t *testing.T, spec apptest.RegistrationTokenSpec) (*apptest.RunnerIdentity, *runnerv1.RegisterResponse) {
	t.Helper()
	token, _ := w.env.CreateRegistrationToken(w.owner, w.org, spec)
	return w.env.RegisterRunner(token)
}

func connectErr(t *testing.T, err error) *connect.Error {
	t.Helper()
	var ce *connect.Error
	require.True(t, errors.As(err, &ce), "want a connect error, got %v", err)
	return ce
}

func requireRPCError(t *testing.T, err error, code connect.Code, message string) {
	t.Helper()
	ce := connectErr(t, err)
	require.Equal(t, code, ce.Code(), ce.Message())
	require.Equal(t, message, ce.Message())
}

func (w world) runnerStatus(t *testing.T, id string) (int, map[string]any) {
	t.Helper()
	res := w.owner.Get(w.path("/runners/" + id))
	var data map[string]any
	if res.Status == http.StatusOK {
		res.Decode(t, &data)
	}
	return res.Status, data
}
