package e2e_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	runnerauth "github.com/Khaymah/dhow/apps/server/internal/Core/RunnerAuth"
)

type wireError struct {
	code    connect.Code
	message string
	meta    map[string][]string
	details int
}

func wireOf(t *testing.T, err error) wireError {
	t.Helper()
	ce := connectErr(t, err)
	meta := map[string][]string{}
	for k, v := range ce.Meta() {
		if k != "Content-Type" && k != "Content-Length" && k != "Date" && k != "X-Request-Id" && k != "Content-Language" {
			meta[k] = v
		}
	}
	return wireError{code: ce.Code(), message: ce.Message(), meta: meta, details: len(ce.Details())}
}

func TestEveryTokenFailureLooksTheSame(t *testing.T) {
	w := newWorld(t)
	otherOrg := w.env.SeedOrganization("Deleted Org", "deleted-org")
	expired, _ := w.env.CreateRegistrationToken(w.owner, w.org, apptest.RegistrationTokenSpec{ExpiresIn: time.Minute})
	used, _ := w.env.CreateRegistrationToken(w.owner, w.org, apptest.RegistrationTokenSpec{})
	w.env.RegisterRunner(used)
	revoked, revokedID := w.env.CreateRegistrationToken(w.owner, w.org, apptest.RegistrationTokenSpec{})
	res := w.owner.Do(apptest.Request{Method: http.MethodPost, Path: w.path("/runner-registration-tokens/" + revokedID + "/revoke")})
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))

	deletedOrgOwner := w.env.SeedUser(otherOrg, "other-owner@example.com", "a long password for other", "owner")
	deletedClient := w.env.SeedSession(deletedOrgOwner)
	ofDeletedOrg, _ := w.env.CreateRegistrationToken(deletedClient, otherOrg, apptest.RegistrationTokenSpec{})
	_, err := w.env.Pool.Exec(context.Background(), `UPDATE "Organizations" SET "IsDeleted" = true WHERE "Id" = $1`, otherOrg)
	require.NoError(t, err)

	unknown, _, err := runnerauth.NewRegistrationToken()
	require.NoError(t, err)
	w.env.Clock.Advance(time.Minute + time.Second)

	cases := map[string]string{
		"expired":                   expired,
		"used up":                   used,
		"revoked":                   revoked,
		"of a deleted organization": ofDeletedOrg,
		"never issued":              unknown,
		"malformed":                 "dhow_rrt_not-a-token",
		"an access token":           "dhow_rat_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		"empty":                     "",
		"with an upper-case prefix": "DHOW_RRT_" + used[len("dhow_rrt_"):],
	}
	var first *wireError
	for name, token := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := w.register(token, apptest.NewRunnerIdentity(t))
			got := wireOf(t, err)
			assert.Equal(t, connect.CodeUnauthenticated, got.code)
			assert.Equal(t, "INVALID_REGISTRATION_TOKEN", got.message)
			if first == nil {
				first = &got
				return
			}
			assert.Equal(t, *first, got, "every token failure must be indistinguishable")
		})
	}
	var count int
	require.NoError(t, w.env.Pool.QueryRow(context.Background(), `SELECT count(*) FROM "Runners"`).Scan(&count))
	assert.Equal(t, 1, count, "only the runner registered with the used token exists")
}

func TestATokenOnlyRegistersIntoItsOwnOrganization(t *testing.T) {
	w := newWorld(t)
	orgB := w.env.SeedOrganization("Org B", "org-b")
	ownerB := w.env.SeedSession(w.env.SeedUser(orgB, "owner-b@example.com", "a long password for owner b", "owner"))
	tokenB, tokenBID := w.env.CreateRegistrationToken(ownerB, orgB, apptest.RegistrationTokenSpec{})
	_, reg := w.env.RegisterRunner(tokenB)
	assert.Equal(t, orgB.String(), reg.GetOrganizationId())

	status, _ := w.runnerStatus(t, reg.GetRunnerId())
	assert.Equal(t, http.StatusNotFound, status, "a runner of organization B is invisible through organization A")
	for _, req := range []apptest.Request{
		{Method: http.MethodPatch, Path: w.path("/runners/" + reg.GetRunnerId()), Body: map[string]any{"Name": "stolen"}},
		{Method: http.MethodPost, Path: w.path("/runners/" + reg.GetRunnerId() + "/revoke")},
		{Method: http.MethodPost, Path: w.path("/runner-registration-tokens/" + tokenBID + "/revoke")},
	} {
		res := w.owner.Do(req)
		assert.Equal(t, http.StatusNotFound, res.Status, "%s %s: %s", req.Method, req.Path, res.Raw)
	}
	list := w.owner.Get(w.path("/runners"))
	var runners []map[string]any
	list.Decode(t, &runners)
	assert.Empty(t, runners)
	res := w.owner.Do(apptest.Request{Method: http.MethodPost, Path: "/organizations/" + orgB.String() + "/runner-registration-tokens", Body: map[string]any{}})
	assert.Equal(t, http.StatusNotFound, res.Status, "an owner of A cannot mint tokens for B")
}

func TestTokenSettingsAreBounded(t *testing.T) {
	w := newWorld(t)
	for _, body := range []map[string]any{
		{"MaxUses": 0},
		{"MaxUses": 51},
		{"ExpiresInSeconds": 59},
		{"ExpiresInSeconds": 86401},
		{"Labels": make([]string, 33)},
		{"Description": "line\nbreak"},
	} {
		res := w.owner.Do(apptest.Request{Method: http.MethodPost, Path: w.path("/runner-registration-tokens"), Body: body})
		assert.Contains(t, []int{http.StatusBadRequest, http.StatusUnprocessableEntity}, res.Status, "%v: %s", body, res.Raw)
	}
	res := w.owner.Do(apptest.Request{Method: http.MethodPost, Path: w.path("/runner-registration-tokens"), Body: map[string]any{
		"MaxUses": 50, "ExpiresInSeconds": 86400, "Description": "fleet", "Labels": []string{"b", "a", "a"},
	}})
	require.Equal(t, http.StatusCreated, res.Status, string(res.Raw))
	var data struct {
		Token, RegisterCommand string
		RegistrationToken      struct {
			MaxUses   int
			Labels    []string
			ExpiresAt time.Time
			CreatedAt time.Time
			Status    string
		}
	}
	res.Decode(t, &data)
	assert.Equal(t, 50, data.RegistrationToken.MaxUses)
	assert.Equal(t, []string{"a", "b"}, data.RegistrationToken.Labels)
	assert.Equal(t, 24*time.Hour, data.RegistrationToken.ExpiresAt.Sub(data.RegistrationToken.CreatedAt))
	assert.Equal(t, "active", data.RegistrationToken.Status)
	assert.Equal(t, "sudo -u dhow-runner dhow-runner register --url "+apptest.PublicOrigin+" --token-file -", data.RegisterCommand)
	assert.NotContains(t, data.RegisterCommand, data.Token, "the token never goes on a command line")
}

func TestTheRegisterCommandAddsDevOnlyOnADevServer(t *testing.T) {
	for _, tc := range []struct {
		name string
		dev  bool
		want string
	}{
		{"production", false, "sudo -u dhow-runner dhow-runner register --url " + apptest.PublicOrigin + " --token-file -"},
		{"dev", true, "sudo -u dhow-runner dhow-runner register --dev --url " + apptest.PublicOrigin + " --token-file -"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := apptest.NewWithOptions(t, apptest.Options{Profile: func(cfg *config.Config) { cfg.Dev = tc.dev }})
			owner, d := env.SetupAndLogin()
			res := owner.Do(apptest.Request{
				Method: http.MethodPost, Path: "/organizations/" + env.OrganizationID(d.OrgSlug).String() + "/runner-registration-tokens",
				Body: map[string]any{},
			})
			require.Equal(t, http.StatusCreated, res.Status, string(res.Raw))
			var data struct{ RegisterCommand string }
			res.Decode(t, &data)
			assert.Equal(t, tc.want, data.RegisterCommand)
		})
	}
}

func TestRevokingATokenIsIdempotentAndKeepsItsRunners(t *testing.T) {
	w := newWorld(t)
	token, tokenID := w.env.CreateRegistrationToken(w.owner, w.org, apptest.RegistrationTokenSpec{MaxUses: 2})
	id, reg := w.env.RegisterRunner(token)
	for range 2 {
		res := w.owner.Do(apptest.Request{Method: http.MethodPost, Path: w.path("/runner-registration-tokens/" + tokenID + "/revoke")})
		require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
		var tok struct{ Status string }
		res.Decode(t, &tok)
		assert.Equal(t, "revoked", tok.Status)
	}
	_, err := w.env.Authenticate(w.env.RunnerClient(apptest.RunnerClientOptions{}), id, reg.GetRunnerId())
	assert.NoError(t, err, "a runner registered before the revocation keeps working")
	res := w.owner.Do(apptest.Request{Method: http.MethodPost, Path: w.path("/runner-registration-tokens/00000000-0000-7000-8000-000000000009/revoke")})
	assert.Equal(t, http.StatusNotFound, res.Status)
}
