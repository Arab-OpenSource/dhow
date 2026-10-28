package e2e_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
	config "github.com/Khaymah/dhow/apps/server/internal/Config"
)

type world struct {
	env      *apptest.Env
	org      uuid.UUID
	owner    *apptest.Client
	password string
}

func newWorld(t *testing.T) world {
	t.Helper()
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	return world{env: env, org: env.OrganizationID(d.OrgSlug), owner: owner, password: d.Password}
}

func (w world) get(token, path string) apptest.Response {
	return w.env.Bearer(token, apptest.Request{Method: http.MethodGet, Path: path})
}

func (w world) members() string { return apptest.OrgPath(w.org, "/members") }

func TestCreateAndUseAToken(t *testing.T) {
	w := newWorld(t)
	res := w.owner.CreateAPIToken(w.org, "Deploy bot", []string{"member.read", "org.read", "member.read"}, 30, apptest.PasswordFactor(w.password))
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	assert.Equal(t, "tokens.token.created", res.Message)
	var created struct {
		Token struct {
			Id, Name, Status string
			Scopes           []string
			ExpiresAt        time.Time
			Owner            struct{ Email string }
		}
		Value string
	}
	res.Decode(t, &created)
	assert.True(t, strings.HasPrefix(created.Value, config.APITokenPrefix))
	assert.Len(t, created.Value, len(config.APITokenPrefix)+43)
	assert.Equal(t, []string{"member.read", "org.read"}, created.Token.Scopes)
	assert.Equal(t, "active", created.Token.Status)
	assert.Equal(t, "owner@example.com", created.Token.Owner.Email)
	assert.WithinDuration(t, w.env.Clock.Now().Add(30*24*time.Hour), created.Token.ExpiresAt, time.Second)

	used := w.get(created.Value, w.members())
	require.Equal(t, http.StatusOK, used.Status, string(used.Raw))
	assert.Equal(t, "identity.members.listed", used.Message)

	for _, path := range []string{"/me/api-tokens", apptest.OrgPath(w.org, "/api-tokens")} {
		list := w.owner.Get(path)
		require.Equal(t, http.StatusOK, list.Status, string(list.Raw))
		assert.NotContains(t, string(list.Raw), created.Value, "%s never shows the value again", path)
		assert.Contains(t, string(list.Raw), created.Token.Id)
	}
	assert.NotContains(t, w.env.Logs.String(), created.Value, "the value never reaches the logs")
	assert.NotContains(t, w.env.Logs.String(), created.Value[len(config.APITokenPrefix):])
	var stored int
	require.NoError(t, w.env.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM "ApiTokens" WHERE position(convert_to($1, 'UTF8') in "TokenHash") > 0`, created.Value).Scan(&stored))
	assert.Zero(t, stored)
	var data string
	require.NoError(t, w.env.Pool.QueryRow(context.Background(), `SELECT string_agg("Data"::text, ' ') FROM "AuditEvents"`).Scan(&data))
	assert.NotContains(t, data, created.Value)
	assert.Equal(t, 1, w.env.AuditCount("auth.api_token_created"))
}

func TestCreateValidatesAndNeedsTheFactor(t *testing.T) {
	w := newWorld(t)
	tests := []struct {
		name   string
		body   func() apptest.Response
		status int
		code   string
	}{
		{"wrong factor", func() apptest.Response {
			return w.owner.CreateAPIToken(w.org, "x", []string{"org.read"}, 30, apptest.PasswordFactor("not the password"))
		}, http.StatusForbidden, "INVALID_CURRENT_FACTOR"},
		{"no expiry", func() apptest.Response {
			return w.owner.CreateAPIToken(w.org, "x", []string{"org.read"}, 0, apptest.PasswordFactor(w.password))
		}, http.StatusUnprocessableEntity, "VALIDATION_FAILED"},
		{"expiry past a year", func() apptest.Response {
			return w.owner.CreateAPIToken(w.org, "x", []string{"org.read"}, 367, apptest.PasswordFactor(w.password))
		}, http.StatusUnprocessableEntity, "VALIDATION_FAILED"},
		{"no scope", func() apptest.Response {
			return w.owner.CreateAPIToken(w.org, "x", []string{}, 30, apptest.PasswordFactor(w.password))
		}, http.StatusUnprocessableEntity, "VALIDATION_FAILED"},
		{"unknown scope", func() apptest.Response {
			return w.owner.CreateAPIToken(w.org, "x", []string{"org.transfer"}, 30, apptest.PasswordFactor(w.password))
		}, http.StatusUnprocessableEntity, "VALIDATION_FAILED"},
		{"empty name", func() apptest.Response {
			return w.owner.CreateAPIToken(w.org, "", []string{"org.read"}, 30, apptest.PasswordFactor(w.password))
		}, http.StatusUnprocessableEntity, "VALIDATION_FAILED"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := tt.body()
			assert.Equal(t, tt.status, res.Status, string(res.Raw))
			require.NotNil(t, res.Error)
			assert.Equal(t, tt.code, res.Error.Code)
		})
	}
	var n int
	require.NoError(t, w.env.Pool.QueryRow(context.Background(), `SELECT count(*) FROM "ApiTokens"`).Scan(&n))
	assert.Zero(t, n)
}

func TestATokenNeverExceedsItsCreator(t *testing.T) {
	w := newWorld(t)
	dev, _ := w.env.SignedIn(w.org, "dev@example.com", "developer")
	over := dev.CreateAPIToken(w.org, "too much", []string{"member.manage"}, 7, apptest.PasswordFactor("a long password for dev@example.com"))
	assert.Equal(t, http.StatusForbidden, over.Status, string(over.Raw))
	assert.Equal(t, "SCOPE_NOT_ALLOWED", over.Error.Code)

	admin, adminID := w.env.SignedIn(w.org, "admin@example.com", "admin")
	token := admin.MustCreateAPIToken(w.org, []string{"member.manage", "member.read"}, "a long password for admin@example.com")
	target := w.env.SeedUser(w.org, "target@example.com", "a long password for target", "viewer")
	change := func() apptest.Response {
		return w.env.Bearer(token.Value, apptest.Request{
			Method: http.MethodPatch, Path: apptest.OrgPath(w.org, "/members/"+target.String()), Body: map[string]string{"Role": "developer"},
		})
	}
	require.Equal(t, http.StatusOK, change().Status)

	require.Equal(t, http.StatusOK, w.owner.ChangeRole(w.org, adminID.String(), "developer", nil).Status)
	demoted := change()
	assert.Equal(t, http.StatusForbidden, demoted.Status, "the token follows its creator's role down: %s", demoted.Raw)
	assert.Equal(t, http.StatusOK, w.get(token.Value, w.members()).Status, "what the new role still allows keeps working")

	require.Equal(t, http.StatusOK, w.owner.ChangeRole(w.org, adminID.String(), "admin", nil).Status)
	assert.Equal(t, http.StatusOK, change().Status, "and back up, never past its own scopes")
	assert.Equal(t, http.StatusForbidden, w.env.Bearer(token.Value, apptest.Request{
		Method: http.MethodPatch, Path: apptest.OrgPath(w.org, ""), Body: map[string]string{"Name": "x"},
	}).Status, "an admin role does not give the token a scope it was not created with")
}

func TestATokenStopsWhenItsCreatorLeavesOrIsDisabled(t *testing.T) {
	w := newWorld(t)
	member, id := w.env.SignedIn(w.org, "m@example.com", "maintainer")
	token := member.MustCreateAPIToken(w.org, []string{"member.read"}, "a long password for m@example.com")
	require.Equal(t, http.StatusOK, w.get(token.Value, w.members()).Status)

	removed := w.owner.Do(apptest.Request{Method: http.MethodDelete, Path: apptest.OrgPath(w.org, "/members/"+id.String())})
	require.Equal(t, http.StatusOK, removed.Status, string(removed.Raw))
	assert.Equal(t, http.StatusUnauthorized, w.get(token.Value, w.members()).Status)
	_, err := w.env.Pool.Exec(context.Background(), `INSERT INTO "Memberships" ("UserId", "OrganizationId", "Role") VALUES ($1, $2, 'maintainer')`, id, w.org)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, w.get(token.Value, w.members()).Status, "joining again does not bring an old token back")
	member, login := w.env.Login("m@example.com", "a long password for m@example.com")
	require.Equal(t, http.StatusOK, login.Status)
	list := member.Get("/me/api-tokens")
	assert.Contains(t, string(list.Raw), `"Status":"inactive"`)

	other, otherID := w.env.SignedIn(w.org, "o@example.com", "viewer")
	second := other.MustCreateAPIToken(w.org, []string{"org.read"}, "a long password for o@example.com")
	_, err = w.env.Pool.Exec(context.Background(), `UPDATE "Users" SET "Disabled" = true WHERE "Id" = $1`, otherID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, w.get(second.Value, apptest.OrgPath(w.org, "")).Status)
}

func TestRevokedAndExpiredTokensStop(t *testing.T) {
	w := newWorld(t)
	token := w.owner.MustCreateAPIToken(w.org, []string{"org.read"}, w.password)
	org := apptest.OrgPath(w.org, "")
	require.Equal(t, http.StatusOK, w.get(token.Value, org).Status)

	revoked := w.owner.Do(apptest.Request{Method: http.MethodPost, Path: "/me/api-tokens/" + token.ID.String() + "/revoke"})
	require.Equal(t, http.StatusOK, revoked.Status, string(revoked.Raw))
	assert.Contains(t, string(revoked.Raw), `"Status":"revoked"`)
	assert.Equal(t, http.StatusUnauthorized, w.get(token.Value, org).Status)
	again := w.owner.Do(apptest.Request{Method: http.MethodPost, Path: "/me/api-tokens/" + token.ID.String() + "/revoke"})
	assert.Equal(t, http.StatusOK, again.Status, "revoking is idempotent")
	assert.Equal(t, 1, w.env.AuditCount("auth.api_token_revoked"))

	short := w.owner.CreateAPIToken(w.org, "short", []string{"org.read"}, 1, apptest.PasswordFactor(w.password))
	require.Equal(t, http.StatusOK, short.Status)
	var created struct{ Value string }
	short.Decode(t, &created)
	require.Equal(t, http.StatusOK, w.get(created.Value, org).Status)
	w.env.Clock.Advance(24 * time.Hour)
	assert.Equal(t, http.StatusUnauthorized, w.get(created.Value, org).Status)
}

func TestTokensStayInTheirOrganization(t *testing.T) {
	w := newWorld(t)
	other := w.env.SeedOrganization("Other", "other")
	ownerID := uuid.MustParse(w.owner.UserID)
	_, err := w.env.Pool.Exec(context.Background(), `INSERT INTO "Memberships" ("UserId", "OrganizationId", "Role") VALUES ($1, $2, 'owner')`, ownerID, other)
	require.NoError(t, err)
	token := w.owner.MustCreateAPIToken(w.org, []string{"org.read", "member.read"}, w.password)
	res := w.get(token.Value, apptest.OrgPath(other, ""))
	assert.Equal(t, http.StatusNotFound, res.Status, "a token for one organization sees nothing of another: %s", res.Raw)
	assert.Equal(t, http.StatusNotFound, w.get(token.Value, apptest.OrgPath(other, "/members")).Status)
}

func TestATokenCannotMakeItsCreatorLeave(t *testing.T) {
	w := newWorld(t)
	admin, adminID := w.env.SignedIn(w.org, "admin@example.com", "admin")
	token := admin.MustCreateAPIToken(w.org, []string{"member.manage", "member.read"}, "a long password for admin@example.com")
	res := w.env.Bearer(token.Value, apptest.Request{Method: http.MethodDelete, Path: apptest.OrgPath(w.org, "/members/"+adminID.String())})
	assert.Equal(t, http.StatusForbidden, res.Status, string(res.Raw))
	require.NotNil(t, res.Error)
	assert.Equal(t, "SESSION_REQUIRED", res.Error.Code)
	assert.Equal(t, http.StatusOK, w.get(token.Value, w.members()).Status, "the creator is still a member")
	target := w.env.SeedUser(w.org, "viewer@example.com", "a long password for viewer", "viewer")
	other := w.env.Bearer(token.Value, apptest.Request{Method: http.MethodDelete, Path: apptest.OrgPath(w.org, "/members/"+target.String())})
	assert.Equal(t, http.StatusOK, other.Status, "removing someone else stays a token action: %s", other.Raw)
}

func TestTokensAreRefusedOnBrowserOnlyRoutes(t *testing.T) {
	w := newWorld(t)
	token := w.owner.MustCreateAPIToken(w.org, []string{"org.read", "org.update", "member.read", "member.manage"}, w.password)
	target := w.env.SeedUser(w.org, "t@example.com", "a long password for t", "viewer")
	routes := []apptest.Request{
		{Method: http.MethodGet, Path: "/me"},
		{Method: http.MethodPut, Path: "/me/password", Body: map[string]string{"CurrentPassword": w.password, "NewPassword": "another long passphrase"}},
		{Method: http.MethodPost, Path: "/me/totp"},
		{Method: http.MethodGet, Path: "/me/mfa"},
		{Method: http.MethodPost, Path: "/sessions/revoke-others"},
		{Method: http.MethodGet, Path: "/me/api-tokens"},
		{Method: http.MethodGet, Path: "/me/identities"},
		{Method: http.MethodPost, Path: apptest.OrgPath(w.org, "/api-tokens"), Body: map[string]any{
			"Name": "x", "Scopes": []string{"org.read"}, "ExpiresInDays": 1, "CurrentFactor": apptest.PasswordFactor(w.password),
		}},
		{Method: http.MethodGet, Path: apptest.OrgPath(w.org, "/api-tokens")},
		{Method: http.MethodPost, Path: apptest.OrgPath(w.org, "/leave")},
		{Method: http.MethodPost, Path: apptest.OrgPath(w.org, "/transfer-ownership"), Body: map[string]any{
			"UserId": target, "CurrentFactor": apptest.PasswordFactor(w.password),
		}},
		{Method: http.MethodPost, Path: apptest.OrgPath(w.org, "/members/"+target.String()+"/mfa-reset"), Body: map[string]any{
			"CurrentFactor": apptest.PasswordFactor(w.password),
		}},
		{Method: http.MethodPost, Path: "/organizations", Body: map[string]string{"Name": "New", "Slug": "new-org"}},
		{Method: http.MethodPost, Path: apptest.OrgPath(w.org, "/jobs"), Body: map[string]any{}},
		{Method: http.MethodPost, Path: "/sessions", Body: map[string]string{"Email": "owner@example.com", "Password": w.password}},
	}
	for _, req := range routes {
		t.Run(req.Method+" "+req.Path, func(t *testing.T) {
			res := w.env.Bearer(token.Value, req)
			assert.Equal(t, http.StatusForbidden, res.Status, string(res.Raw))
			require.NotNil(t, res.Error)
			assert.Equal(t, "SESSION_REQUIRED", res.Error.Code)
		})
	}
	owner := w.env.Bearer(token.Value, apptest.Request{
		Method: http.MethodPatch, Path: apptest.OrgPath(w.org, "/members/"+target.String()),
		Body: map[string]any{"Role": "owner", "CurrentFactor": apptest.PasswordFactor(w.password)},
	})
	assert.Equal(t, http.StatusForbidden, owner.Status, "granting Owner needs a factor, and a token cannot confirm one: %s", owner.Raw)
	assert.Equal(t, "SESSION_REQUIRED", owner.Error.Code)
	_, login := w.env.Login("owner@example.com", w.password)
	assert.Equal(t, http.StatusOK, login.Status, "the password did not change")
}

func TestOnlyOwnersAndAdminsManageTheOrganizationTokens(t *testing.T) {
	w := newWorld(t)
	dev, _ := w.env.SignedIn(w.org, "dev@example.com", "developer")
	token := dev.MustCreateAPIToken(w.org, []string{"org.read"}, "a long password for dev@example.com")
	assert.Equal(t, http.StatusForbidden, dev.Get(apptest.OrgPath(w.org, "/api-tokens")).Status)
	other := dev.Do(apptest.Request{Method: http.MethodPost, Path: "/me/api-tokens/" + uuid.New().String() + "/revoke"})
	assert.Equal(t, http.StatusNotFound, other.Status)
	ownerToken := w.owner.MustCreateAPIToken(w.org, []string{"org.read"}, w.password)
	theirs := dev.Do(apptest.Request{Method: http.MethodPost, Path: "/me/api-tokens/" + ownerToken.ID.String() + "/revoke"})
	assert.Equal(t, http.StatusNotFound, theirs.Status, "another user's token looks absent")

	list := w.owner.Get(apptest.OrgPath(w.org, "/api-tokens"))
	require.Equal(t, http.StatusOK, list.Status)
	assert.Contains(t, string(list.Raw), "dev@example.com")
	res := w.owner.Do(apptest.Request{Method: http.MethodPost, Path: apptest.OrgPath(w.org, "/api-tokens/"+token.ID.String()+"/revoke")})
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	assert.Equal(t, http.StatusUnauthorized, w.get(token.Value, apptest.OrgPath(w.org, "")).Status)
}

func TestLastUsedIsThrottledAndActionsAreAuditedWithTheToken(t *testing.T) {
	w := newWorld(t)
	token := w.owner.MustCreateAPIToken(w.org, []string{"org.read", "org.update"}, w.password)
	lastUsed := func() *time.Time {
		var at *time.Time
		require.NoError(t, w.env.Pool.QueryRow(context.Background(), `SELECT "LastUsedAt" FROM "ApiTokens" WHERE "Id" = $1`, token.ID).Scan(&at))
		return at
	}
	assert.Nil(t, lastUsed())
	require.Equal(t, http.StatusOK, w.get(token.Value, apptest.OrgPath(w.org, "")).Status)
	first := lastUsed()
	require.NotNil(t, first)
	w.env.Clock.Advance(30 * time.Second)
	require.Equal(t, http.StatusOK, w.get(token.Value, apptest.OrgPath(w.org, "")).Status)
	assert.True(t, first.Equal(*lastUsed()), "at most one write a minute")
	w.env.Clock.Advance(31 * time.Second)
	require.Equal(t, http.StatusOK, w.get(token.Value, apptest.OrgPath(w.org, "")).Status)
	assert.True(t, lastUsed().After(*first))

	renamed := w.env.Bearer(token.Value, apptest.Request{Method: http.MethodPatch, Path: apptest.OrgPath(w.org, ""), Body: map[string]string{"Name": "Renamed by CI"}})
	require.Equal(t, http.StatusOK, renamed.Status, string(renamed.Raw))
	var via, viaID string
	require.NoError(t, w.env.Pool.QueryRow(context.Background(),
		`SELECT "Data"->>'Via', "Data"->>'ViaId' FROM "AuditEvents" WHERE "Action" = 'identity.organization.updated' ORDER BY "Seq" DESC LIMIT 1`).Scan(&via, &viaID))
	assert.Equal(t, "api_token", via)
	assert.Equal(t, token.ID.String(), viaID)
}

func TestTheActiveTokenLimit(t *testing.T) {
	w := newWorld(t)
	ownerID := uuid.MustParse(w.owner.UserID)
	for range config.APITokenMaxPerUser {
		w.env.SeedAPIToken(ownerID, w.org)
	}
	res := w.owner.CreateAPIToken(w.org, "one too many", []string{"org.read"}, 1, apptest.PasswordFactor(w.password))
	assert.Equal(t, http.StatusConflict, res.Status, string(res.Raw))
	assert.Equal(t, "API_TOKEN_LIMIT", res.Error.Code)
}
