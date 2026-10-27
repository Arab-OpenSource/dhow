package e2e_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Arab-OpenSource/dhow/apps/server/internal/App/Testing"
)

type event struct {
	Id, CreatedAt, OrganizationId, ActorUserId, Action, TargetType, TargetId, Ip string
	Data                                                                         map[string]string
}

type verifyResult struct {
	Valid         bool
	Checked       int
	FirstBrokenId string
}

func listEvents(t *testing.T, c *apptest.Client, orgID string, query string) ([]event, apptest.Response) {
	t.Helper()
	res := c.Get("/organizations/" + orgID + "/audit-events" + query)
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	var events []event
	res.Decode(t, &events)
	return events, res
}

func TestSecurityEventsAreRecorded(t *testing.T) {
	env := apptest.New(t)
	d := apptest.DefaultSetup()
	require.Equal(t, http.StatusOK, env.Setup(d).Status)
	orgID := env.OrganizationID(d.OrgSlug).String()

	failed := env.Do(apptest.Request{
		Method: http.MethodPost, Path: "/sessions", Headers: map[string]string{"Origin": apptest.PublicOrigin},
		Body: map[string]string{"Email": d.Email, "Password": "Wr0ng-secret-passphrase"},
	})
	require.Equal(t, http.StatusUnauthorized, failed.Status)
	client, login := env.Login(d.Email, d.Password)
	require.Equal(t, http.StatusOK, login.Status)
	changed := client.Do(apptest.Request{Method: http.MethodPut, Path: "/me/password", Body: map[string]string{
		"CurrentPassword": d.Password, "NewPassword": "N3w-secret-passphrase",
	}})
	require.Equal(t, http.StatusOK, changed.Status)
	other, _ := env.Login(d.Email, "N3w-secret-passphrase")
	require.Equal(t, http.StatusOK, client.Do(apptest.Request{Method: http.MethodPost, Path: "/sessions/revoke-others"}).Status)
	require.Equal(t, http.StatusUnauthorized, other.Get("/me").Status)
	require.Equal(t, http.StatusOK, env.Logout(client).Status)

	owner, _ := env.Login(d.Email, "N3w-secret-passphrase")
	events, res := listEvents(t, owner, orgID, "?Limit=100")
	require.NotNil(t, res.Meta)
	assert.Equal(t, len(events), res.Meta.Total)

	actions := make([]string, 0, len(events))
	for _, e := range events {
		actions = append(actions, e.Action)
	}
	assert.Equal(t, []string{
		"auth.login", "auth.logout", "auth.sessions_revoked_others", "auth.login", "auth.password_changed", "auth.login",
		"auth.login_failed", "system.instance.setupCompleted",
	}, actions, "newest first")

	byAction := map[string]event{}
	for _, e := range events {
		byAction[e.Action] = e
	}
	failedEvent := byAction["auth.login_failed"]
	assert.Equal(t, "203.0.113.7", failedEvent.Ip)
	assert.Equal(t, orgID, failedEvent.OrganizationId)
	assert.NotEmpty(t, failedEvent.ActorUserId, "a failure against a real account names it")
	assert.Equal(t, []string{"EmailDigest"}, keys(failedEvent.Data))
	assert.NotContains(t, string(res.Raw), "Wr0ng-secret", "the password that was tried is never recorded")
	assert.NotContains(t, string(res.Raw), "N3w-secret")
	assert.NotContains(t, string(res.Raw), d.Password)
	assert.NotContains(t, string(res.Raw), "owner@example.com", "failed logins keep a digest, not the typed email")

	revoked := byAction["auth.sessions_revoked_others"]
	assert.Equal(t, map[string]string{"Revoked": "1"}, revoked.Data)
	assert.Equal(t, "user", byAction["auth.login_failed"].TargetType)
	assert.Equal(t, "session", byAction["auth.login"].TargetType)
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestFailedLoginsForUnknownAccountsLandInTheInstanceChain(t *testing.T) {
	env := apptest.New(t)
	client, d := env.SetupAndLogin()
	orgID := env.OrganizationID(d.OrgSlug).String()
	res := env.Do(apptest.Request{
		Method: http.MethodPost, Path: "/sessions", Headers: map[string]string{"Origin": apptest.PublicOrigin},
		Body: map[string]string{"Email": "typo-victim@example.com", "Password": "Wr0ng-secret-passphrase"},
	})
	require.Equal(t, http.StatusUnauthorized, res.Status)

	var n int
	var actor *string
	require.NoError(t, env.Pool.QueryRow(context.Background(),
		`SELECT count(*), max("ActorUserId"::text) FROM "AuditEvents" WHERE "OrganizationId" IS NULL AND "Action" = 'auth.login_failed'`).Scan(&n, &actor))
	assert.Equal(t, 1, n)
	assert.Nil(t, actor)
	events, _ := listEvents(t, client, orgID, "")
	for _, e := range events {
		assert.NotEqual(t, "auth.login_failed", e.Action, "an organization never sees attempts against accounts that are not its members")
	}
}

func TestVerifyChainEndpoint(t *testing.T) {
	env := apptest.New(t)
	client, d := env.SetupAndLogin()
	orgID := env.OrganizationID(d.OrgSlug).String()
	for range 3 {
		env.Login(d.Email, d.Password)
	}

	res := client.Get("/organizations/" + orgID + "/audit-events/verify")
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	var ok verifyResult
	res.Decode(t, &ok)
	assert.True(t, ok.Valid)
	assert.Equal(t, 5, ok.Checked)
	assert.Empty(t, ok.FirstBrokenId)

	events, _ := listEvents(t, client, orgID, "")
	target := events[2]
	ctx := context.Background()
	tx, err := env.Pool.Begin(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `ALTER TABLE "AuditEvents" DISABLE TRIGGER "TR_AuditEvents_AppendOnly"`)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `UPDATE "AuditEvents" SET "Ip" = '198.51.100.250' WHERE "Id" = $1`, target.Id)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))

	res = client.Get("/organizations/" + orgID + "/audit-events/verify")
	var broken verifyResult
	res.Decode(t, &broken)
	assert.False(t, broken.Valid)
	assert.Equal(t, target.Id, broken.FirstBrokenId, "the response names the first event that no longer matches")
	assert.Equal(t, 3, broken.Checked)
}

func TestListPagination(t *testing.T) {
	env := apptest.New(t)
	client, d := env.SetupAndLogin()
	orgID := env.OrganizationID(d.OrgSlug).String()
	for range 4 {
		env.Login(d.Email, d.Password)
	}
	all, res := listEvents(t, client, orgID, "?Limit=100")
	require.Len(t, all, 6)
	assert.Equal(t, 6, res.Meta.Total)
	assert.Equal(t, 100, res.Meta.Limit)

	first, res := listEvents(t, client, orgID, "?Limit=4&Page=1")
	assert.Equal(t, all[:4], first)
	assert.Equal(t, 1, res.Meta.Page)
	second, res := listEvents(t, client, orgID, "?Limit=4&Page=2")
	assert.Equal(t, all[4:], second)
	assert.Equal(t, 2, res.Meta.Page)
	assert.Equal(t, 6, res.Meta.Total)
	beyond, _ := listEvents(t, client, orgID, "?Limit=4&Page=3")
	assert.Empty(t, beyond)
}

func TestListParameterEdges(t *testing.T) {
	env := apptest.New(t)
	client, d := env.SetupAndLogin()
	orgID := env.OrganizationID(d.OrgSlug).String()
	tests := []struct {
		name      string
		query     string
		status    int
		wantLimit int
	}{
		{"defaults", "", 200, 25},
		{"a limit above the cap is capped", "?Limit=100000", 200, 100},
		{"a zero page falls back to the first", "?Page=0", 200, 25},
		{"a negative limit falls back", "?Limit=-5", 200, 25},
		{"a non-numeric page is a bad request", "?Page=abc", 400, 0},
		{"a non-numeric limit is a bad request", "?Limit=ten", 400, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := client.Get("/organizations/" + orgID + "/audit-events" + tt.query)
			assert.Equal(t, tt.status, res.Status, string(res.Raw))
			if tt.status == http.StatusOK {
				assert.Equal(t, tt.wantLimit, res.Meta.Limit)
			}
		})
	}
}

func TestOnlyOwnersAndAdminsReadTheAuditLog(t *testing.T) {
	env := apptest.New(t)
	_, d := env.SetupAndLogin()
	orgID := env.OrganizationID(d.OrgSlug)
	for role, want := range map[string]int{"admin": 200, "maintainer": 403, "developer": 403, "viewer": 403} {
		email := role + "@example.com"
		env.SeedUser(orgID, email, "a long password for "+role, role)
		c, res := env.Login(email, "a long password for "+role)
		require.Equal(t, http.StatusOK, res.Status)
		assert.Equal(t, want, c.Get("/organizations/"+orgID.String()+"/audit-events").Status, role)
		assert.Equal(t, want, c.Get("/organizations/"+orgID.String()+"/audit-events/verify").Status, role)
	}
}
