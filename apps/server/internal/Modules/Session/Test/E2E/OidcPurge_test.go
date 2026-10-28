package e2e_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	app "github.com/Khaymah/dhow/apps/server/internal/App"
	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
	config "github.com/Khaymah/dhow/apps/server/internal/Config"
)

func storedFlows(t *testing.T, env *apptest.Env) int {
	t.Helper()
	var n int
	require.NoError(t, env.Pool.QueryRow(context.Background(), `SELECT count(*) FROM "OidcFlows"`).Scan(&n))
	return n
}

func beginLogin(t *testing.T, env *apptest.Env) apptest.OidcStart {
	t.Helper()
	start := (&apptest.Client{Env: env}).BeginOidc("/oidc/authorize", map[string]string{"Provider": apptest.OidcProvider})
	require.Equal(t, http.StatusOK, start.Res.Status, string(start.Res.Raw))
	return start
}

func TestExpiredOidcFlowsAreDeletedAfterTheGracePeriod(t *testing.T) {
	env, idp := apptest.NewWithIdP(t, apptest.OidcOptions{})
	owner, _ := env.SetupAndLogin()
	stale := beginLogin(t, env)
	used := beginLogin(t, env)
	code, state := idp.Authorize(used.URL, sara())
	owner.FinishOidc("/oidc/callback", used.Cookie, code, state, nil)
	env.Clock.Advance(config.OidcFlowTTL + config.RetentionOidcFlowAfter + time.Second)
	fresh := beginLogin(t, env)
	require.Equal(t, 3, storedFlows(t, env))

	report, err := env.Wired.Retention.Run(context.Background())
	require.NoError(t, err)
	assert.True(t, report.Ran)
	assert.Equal(t, int64(2), report.Deleted["OidcFlows"], "the expired flow and the used one")
	assert.Equal(t, 1, storedFlows(t, env), "the rows are gone from the table, not only marked")

	code, state = idp.Authorize(fresh.URL, sara())
	res := (&apptest.Client{Env: env}).FinishOidc("/oidc/callback", fresh.Cookie, code, state, nil)
	assert.NotEqual(t, "OIDC_FLOW_INVALID", errorCode(res), "a live flow survives: %s", res.Raw)
	code, state = idp.Authorize(stale.URL, sara())
	res = (&apptest.Client{Env: env}).FinishOidc("/oidc/callback", stale.Cookie, code, state, nil)
	assert.Equal(t, "OIDC_FLOW_INVALID", errorCode(res))
}

func TestTheMaintenanceLoopPurgesAndStops(t *testing.T) {
	env, _ := apptest.NewWithIdP(t, apptest.OidcOptions{})
	beginLogin(t, env)
	env.Clock.Advance(config.OidcFlowTTL + config.RetentionOidcFlowAfter + time.Second)
	stop := app.StartMaintenance(context.Background(), env.Wired.Retention, 10*time.Millisecond)
	assert.Eventually(t, func() bool { return storedFlows(t, env) == 0 }, 5*time.Second, 20*time.Millisecond)
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
