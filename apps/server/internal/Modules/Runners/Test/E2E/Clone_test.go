package e2e_test

import (
	"net/http"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
)

func TestACopiedIdentityDisablesTheRunner(t *testing.T) {
	w := newWorld(t)
	id, reg := w.registerOK(t, apptest.RegistrationTokenSpec{})
	hostA := w.env.RunnerClient(apptest.RunnerClientOptions{Remote: "198.51.100.1:1000"})
	hostB := w.env.RunnerClient(apptest.RunnerClientOptions{Remote: "198.51.100.2:1000"})

	_, err := w.env.Authenticate(hostA, id, reg.GetRunnerId())
	require.NoError(t, err)
	for i, host := range []*apptest.RunnerClient{hostB, hostA, hostB} {
		w.env.Clock.Advance(time.Minute)
		_, err := w.env.Authenticate(host, id, reg.GetRunnerId())
		require.NoError(t, err, "switch %d is still within the limit", i+1)
	}
	w.env.Clock.Advance(time.Minute)
	_, err = w.env.Authenticate(hostA, id, reg.GetRunnerId())
	requireRPCError(t, err, connect.CodePermissionDenied, "RUNNER_DISABLED")
	_, err = w.env.Authenticate(hostB, id, reg.GetRunnerId())
	requireRPCError(t, err, connect.CodePermissionDenied, "RUNNER_DISABLED")

	_, runner := w.runnerStatus(t, reg.GetRunnerId())
	assert.Equal(t, "disabled", runner["State"])
	assert.Equal(t, "clone_suspected", runner["DisabledReason"])

	events := w.owner.Get(w.path("/audit-events?Limit=100"))
	var list []struct{ Action, TargetId string }
	events.Decode(t, &list)
	found := 0
	for _, e := range list {
		if e.Action == "runner.disabled_clone_suspected" {
			found++
			assert.Equal(t, reg.GetRunnerId(), e.TargetId)
		}
	}
	assert.Equal(t, 1, found)

	res := w.owner.Do(apptest.Request{Method: http.MethodPatch, Path: w.path("/runners/" + reg.GetRunnerId()), Body: map[string]any{"Disabled": false}})
	require.Equal(t, http.StatusOK, res.Status)
	w.env.Clock.Advance(11 * time.Minute)
	_, err = w.env.Authenticate(hostA, id, reg.GetRunnerId())
	assert.NoError(t, err, "an administrator can enable the runner again")
}

func TestRenewingEveryTenMinutesIsNotAClone(t *testing.T) {
	w := newWorld(t)
	id, reg := w.registerOK(t, apptest.RegistrationTokenSpec{})
	rc := w.env.RunnerClient(apptest.RunnerClientOptions{})
	for i := range 12 {
		_, err := w.env.Authenticate(rc, id, reg.GetRunnerId())
		require.NoError(t, err, "renewal %d", i)
		w.env.Clock.Advance(10 * time.Minute)
	}
	for range 6 {
		w.env.Clock.Advance(16 * time.Minute)
		_, err := w.env.Authenticate(rc, id, reg.GetRunnerId())
		require.NoError(t, err, "restarting after the token expired is not a switch")
	}
	_, runner := w.runnerStatus(t, reg.GetRunnerId())
	assert.Equal(t, "active", runner["State"])
}
