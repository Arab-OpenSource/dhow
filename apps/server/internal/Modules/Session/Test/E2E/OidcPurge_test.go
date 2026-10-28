package e2e_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	app "github.com/Arab-OpenSource/dhow/apps/server/internal/App"
	apptest "github.com/Arab-OpenSource/dhow/apps/server/internal/App/Testing"
	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
)

func liveFlows(t *testing.T, env *apptest.Env) int {
	t.Helper()
	var n int
	require.NoError(t, env.Pool.QueryRow(context.Background(), `SELECT count(*) FROM "OidcFlows" WHERE NOT "IsDeleted"`).Scan(&n))
	return n
}

func beginLogin(t *testing.T, env *apptest.Env) apptest.OidcStart {
	t.Helper()
	start := (&apptest.Client{Env: env}).BeginOidc("/oidc/authorize", map[string]string{"Provider": apptest.OidcProvider})
	require.Equal(t, http.StatusOK, start.Res.Status, string(start.Res.Raw))
	return start
}

func TestExpiredAndUsedOidcFlowsArePurged(t *testing.T) {
	env, idp := apptest.NewWithIdP(t, apptest.OidcOptions{})
	owner, _ := env.SetupAndLogin()
	stale := beginLogin(t, env)
	env.Clock.Advance(config.OidcFlowTTL + time.Second)
	used := beginLogin(t, env)
	code, state := idp.Authorize(used.URL, sara())
	owner.FinishOidc("/oidc/callback", used.Cookie, code, state, nil)
	fresh := beginLogin(t, env)
	require.Equal(t, 3, liveFlows(t, env))

	n, err := env.Wired.Options.Session.PurgeOidcFlows(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(2), n, "the expired flow and the used one")
	assert.Equal(t, 1, liveFlows(t, env))

	code, state = idp.Authorize(fresh.URL, sara())
	res := (&apptest.Client{Env: env}).FinishOidc("/oidc/callback", fresh.Cookie, code, state, nil)
	assert.NotEqual(t, "OIDC_FLOW_INVALID", errorCode(res), "a live flow survives the purge: %s", res.Raw)
	code, state = idp.Authorize(stale.URL, sara())
	res = (&apptest.Client{Env: env}).FinishOidc("/oidc/callback", stale.Cookie, code, state, nil)
	assert.Equal(t, "OIDC_FLOW_INVALID", errorCode(res))
}

func TestTheMaintenanceLoopPurgesAndStops(t *testing.T) {
	env, _ := apptest.NewWithIdP(t, apptest.OidcOptions{})
	beginLogin(t, env)
	env.Clock.Advance(config.OidcFlowTTL + time.Second)
	stop := app.StartMaintenance(context.Background(), env.Wired.Options.Session, 10*time.Millisecond)
	assert.Eventually(t, func() bool { return liveFlows(t, env) == 0 }, 5*time.Second, 20*time.Millisecond)
	done := make(chan struct{})
	go func() {
		stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the maintenance loop did not stop")
	}
}

func errorCode(res apptest.Response) string {
	if res.Error == nil {
		return ""
	}
	return res.Error.Code
}
