package e2e_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Arab-OpenSource/dhow/apps/server/internal/App/Testing"
	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
	oidcmock "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Oidc/Mock"
)

func assertCode(t *testing.T, res apptest.Response, status int, code string) {
	t.Helper()
	assert.Equal(t, status, res.Status, string(res.Raw))
	require.NotNil(t, res.Error, string(res.Raw))
	assert.Equal(t, code, res.Error.Code)
	_, _, ok := apptest.SetCookie(res.Header, config.SessionCookieName)
	assert.False(t, ok, "a failed callback never sets a session")
}

func linkedEnv(t *testing.T, o apptest.OidcOptions) (*apptest.Env, *oidcmock.IdP, *apptest.Client, apptest.SetupDetails) {
	t.Helper()
	env, idp, owner, d := ownerWithIdP(t, o)
	require.Equal(t, http.StatusOK, owner.LinkOidc(idp, apptest.PasswordFactor(d.Password), sara()).Status)
	return env, idp, owner, d
}

func begin(t *testing.T, env *apptest.Env) (*apptest.Client, apptest.OidcStart) {
	t.Helper()
	anonymous := &apptest.Client{Env: env}
	start := anonymous.BeginOidc("/oidc/authorize", map[string]string{"Provider": apptest.OidcProvider})
	require.Equal(t, http.StatusOK, start.Res.Status, string(start.Res.Raw))
	return anonymous, start
}

func TestOidcCallbackRejectsAWrongStateAndBurnsTheFlow(t *testing.T) {
	env, idp, _, _ := linkedEnv(t, apptest.OidcOptions{})
	anonymous, start := begin(t, env)
	code, state := idp.Authorize(start.URL, sara())
	exchanges := idp.Count("token")
	assertCode(t, anonymous.FinishOidc("/oidc/callback", start.Cookie, code, strings.Repeat("A", len(state)), nil), http.StatusBadRequest, "OIDC_FLOW_INVALID")
	assertCode(t, anonymous.FinishOidc("/oidc/callback", start.Cookie, code, state, nil), http.StatusBadRequest, "OIDC_FLOW_INVALID")
	assert.Equal(t, exchanges, idp.Count("token"), "nothing reaches the provider without the right state")
}

func TestOidcCallbackNeedsTheFlowCookieOfTheSameBrowser(t *testing.T) {
	env, idp, _, _ := linkedEnv(t, apptest.OidcOptions{})
	tests := []struct {
		name    string
		cookie  func(apptest.OidcStart) string
		headers map[string]string
	}{
		{"no flow cookie", func(apptest.OidcStart) string { return "" }, nil},
		{"forged flow cookie", func(apptest.OidcStart) string { return strings.Repeat("A", 43) }, nil},
		{"another browser", func(s apptest.OidcStart) string { return s.Cookie }, map[string]string{"User-Agent": "another-browser"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			anonymous, start := begin(t, env)
			code, state := idp.Authorize(start.URL, sara())
			assertCode(t, anonymous.FinishOidc("/oidc/callback", tt.cookie(start), code, state, tt.headers), http.StatusBadRequest, "OIDC_FLOW_INVALID")
		})
	}
}

func TestOidcCallbackWorksOnceAndCodesDoNotMoveBetweenFlows(t *testing.T) {
	env, idp, _, _ := linkedEnv(t, apptest.OidcOptions{})
	anonymous, start := begin(t, env)
	code, state := idp.Authorize(start.URL, sara())
	first := anonymous.FinishOidc("/oidc/callback", start.Cookie, code, state, nil)
	require.Equal(t, http.StatusOK, first.Status, string(first.Raw))
	assertCode(t, anonymous.FinishOidc("/oidc/callback", start.Cookie, code, state, nil), http.StatusBadRequest, "OIDC_FLOW_INVALID")

	again, second := begin(t, env)
	_, secondState := idp.Authorize(second.URL, sara())
	assertCode(t, again.FinishOidc("/oidc/callback", second.Cookie, code, secondState, nil), http.StatusUnauthorized, "OIDC_SIGN_IN_FAILED")
}

func TestOidcCallbackRejectsAnotherProviderOrAnExpiredFlow(t *testing.T) {
	env, idp, _, _ := linkedEnv(t, apptest.OidcOptions{})
	anonymous, start := begin(t, env)
	code, state := idp.Authorize(start.URL, sara())
	res := anonymous.Do(apptest.Request{
		Method: http.MethodPost, Path: "/oidc/callback", Headers: map[string]string{"Cookie": config.OidcCookieName + "=" + start.Cookie},
		Body: map[string]string{"Provider": "other", "Code": code, "State": state},
	})
	assertCode(t, res, http.StatusBadRequest, "OIDC_FLOW_INVALID")

	late, expired := begin(t, env)
	lateCode, lateState := idp.Authorize(expired.URL, sara())
	env.Clock.Advance(config.OidcFlowTTL + time.Second)
	assertCode(t, late.FinishOidc("/oidc/callback", expired.Cookie, lateCode, lateState, nil), http.StatusBadRequest, "OIDC_FLOW_INVALID")
}

func TestOidcRejectsTokensThatFailVerification(t *testing.T) {
	other := "another-nonce"
	tests := []struct {
		name   string
		mutate func(*oidcmock.Claims)
	}{
		{"issuer mismatch", func(c *oidcmock.Claims) { c.Issuer = "https://evil.example" }},
		{"audience mismatch", func(c *oidcmock.Claims) { c.Audience = []string{"another-client"} }},
		{"expired id_token", func(c *oidcmock.Claims) { c.ExpiresIn = -time.Minute }},
		{"wrong nonce", func(c *oidcmock.Claims) { c.Nonce = &other }},
		{"signed with another key", func(c *oidcmock.Claims) { c.SignWithOther = true }},
	}
	env, idp, _, _ := linkedEnv(t, apptest.OidcOptions{})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := sara()
			tt.mutate(&claims)
			anonymous, start := begin(t, env)
			code, state := idp.Authorize(start.URL, claims)
			assertCode(t, anonymous.FinishOidc("/oidc/callback", start.Cookie, code, state, nil), http.StatusUnauthorized, "OIDC_SIGN_IN_FAILED")
		})
	}
	var reasons int
	require.NoError(t, env.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM "AuditEvents" WHERE "Action" = 'auth.oidc_login_failed' AND "Data"->>'Reason' = 'rejected'`).Scan(&reasons))
	assert.Equal(t, len(tests), reasons)
}

func TestOidcAllowedDomains(t *testing.T) {
	env, idp, owner, d := ownerWithIdP(t, apptest.OidcOptions{Domains: []string{"corp.example"}})
	personal := oidcmock.Claims{Subject: "p", Email: "sara@gmail.com", EmailVerified: true}
	unverified := oidcmock.Claims{Subject: "u", Email: "sara@corp.example"}
	lookalike := oidcmock.Claims{Subject: "l", Email: "sara@corp.example.evil.com", EmailVerified: true}
	for name, claims := range map[string]oidcmock.Claims{"personal": personal, "unverified": unverified, "lookalike": lookalike} {
		t.Run(name, func(t *testing.T) {
			res := owner.LinkOidc(idp, apptest.PasswordFactor(d.Password), claims)
			assert.Equal(t, http.StatusForbidden, res.Status, string(res.Raw))
			assert.Equal(t, "OIDC_DOMAIN_NOT_ALLOWED", res.Error.Code)
			_, login := env.OidcLogin(idp, claims)
			assertCode(t, login, http.StatusForbidden, "OIDC_DOMAIN_NOT_ALLOWED")
		})
	}
	assert.Zero(t, identityCount(t, env))
	linked := owner.LinkOidc(idp, apptest.PasswordFactor(d.Password), sara())
	require.Equal(t, http.StatusOK, linked.Status, string(linked.Raw))
	_, login := env.OidcLogin(idp, sara())
	assert.Equal(t, http.StatusOK, login.Status)
}

func TestOidcLinkConflicts(t *testing.T) {
	env, idp, owner, d := linkedEnv(t, apptest.OidcOptions{})
	again := owner.BeginOidc("/me/identities/authorize", map[string]any{"Provider": apptest.OidcProvider, "CurrentFactor": apptest.PasswordFactor(d.Password)})
	assert.Equal(t, http.StatusConflict, again.Res.Status)
	assert.Equal(t, "OIDC_ALREADY_LINKED", again.Res.Error.Code)

	other, _ := env.SignedIn(env.OrganizationID(d.OrgSlug), "other@example.com", "developer")
	res := other.LinkOidc(idp, apptest.PasswordFactor("a long password for other@example.com"), sara())
	assert.Equal(t, http.StatusConflict, res.Status, string(res.Raw))
	assert.Equal(t, "OIDC_IDENTITY_IN_USE", res.Error.Code)
	assert.Equal(t, 1, identityCount(t, env))
}

func TestOidcFlowsStayWithTheirPurposeAndSession(t *testing.T) {
	env, idp, owner, d := ownerWithIdP(t, apptest.OidcOptions{})
	second, _ := env.Login(d.Email, d.Password)

	link := owner.BeginOidc("/me/identities/authorize", map[string]any{"Provider": apptest.OidcProvider, "CurrentFactor": apptest.PasswordFactor(d.Password)})
	require.Equal(t, http.StatusOK, link.Res.Status, string(link.Res.Raw))
	code, state := idp.Authorize(link.URL, sara())
	res := second.FinishOidc("/me/identities/callback", link.Cookie, code, state, nil)
	assert.Equal(t, http.StatusBadRequest, res.Status, "a link flow belongs to the session that started it")

	link = owner.BeginOidc("/me/identities/authorize", map[string]any{"Provider": apptest.OidcProvider, "CurrentFactor": apptest.PasswordFactor(d.Password)})
	code, state = idp.Authorize(link.URL, sara())
	anonymous := &apptest.Client{Env: env}
	assertCode(t, anonymous.FinishOidc("/oidc/callback", link.Cookie, code, state, nil), http.StatusBadRequest, "OIDC_FLOW_INVALID")

	_, login := begin(t, env)
	code, state = idp.Authorize(login.URL, sara())
	res = owner.FinishOidc("/me/identities/callback", login.Cookie, code, state, nil)
	assert.Equal(t, http.StatusBadRequest, res.Status, "a sign-in flow cannot link")
	assert.Zero(t, identityCount(t, env))
}

func TestOidcLinkNeedsTheCurrentFactor(t *testing.T) {
	env, _, owner, _ := ownerWithIdP(t, apptest.OidcOptions{})
	start := owner.BeginOidc("/me/identities/authorize", map[string]any{"Provider": apptest.OidcProvider, "CurrentFactor": apptest.PasswordFactor("not the password")})
	assert.Equal(t, http.StatusForbidden, start.Res.Status)
	assert.Equal(t, "INVALID_CURRENT_FACTOR", start.Res.Error.Code)
	_, _, ok := apptest.SetCookie(start.Res.Header, config.OidcCookieName)
	assert.False(t, ok)
	var flows int
	require.NoError(t, env.Pool.QueryRow(context.Background(), `SELECT count(*) FROM "OidcFlows"`).Scan(&flows))
	assert.Zero(t, flows)
}

func TestOidcRefusesDisabledAccountsAndUnknownProviders(t *testing.T) {
	env, idp, _, _ := linkedEnv(t, apptest.OidcOptions{})
	_, err := env.Pool.Exec(context.Background(), `UPDATE "Users" SET "Disabled" = true WHERE "Email" = 'owner@example.com'`)
	require.NoError(t, err)
	_, res := env.OidcLogin(idp, sara())
	assertCode(t, res, http.StatusUnauthorized, "OIDC_SIGN_IN_FAILED")

	unknown := (&apptest.Client{Env: env}).BeginOidc("/oidc/authorize", map[string]string{"Provider": "nobody"})
	assert.Equal(t, http.StatusNotFound, unknown.Res.Status)
}

func TestOidcProviderOutageIsAServiceError(t *testing.T) {
	env, idp := apptest.NewWithIdP(t, apptest.OidcOptions{})
	idp.DiscoveredIssuer = "https://other.example"
	start := (&apptest.Client{Env: env}).BeginOidc("/oidc/authorize", map[string]string{"Provider": apptest.OidcProvider})
	assert.Equal(t, http.StatusServiceUnavailable, start.Res.Status, string(start.Res.Raw))
	assert.Equal(t, "OIDC_PROVIDER_UNAVAILABLE", start.Res.Error.Code)
}

func TestOidcSecretsNeverReachLogsOrResponses(t *testing.T) {
	env, idp, owner, d := ownerWithIdP(t, apptest.OidcOptions{})
	link := owner.BeginOidc("/me/identities/authorize", map[string]any{"Provider": apptest.OidcProvider, "CurrentFactor": apptest.PasswordFactor(d.Password)})
	code, state := idp.Authorize(link.URL, sara())
	linked := owner.FinishOidc("/me/identities/callback", link.Cookie, code, state, nil)
	require.Equal(t, http.StatusOK, linked.Status)
	anonymous, start := begin(t, env)
	loginCode, loginState := idp.Authorize(start.URL, sara())
	res := anonymous.FinishOidc("/oidc/callback", start.Cookie, loginCode, loginState, nil)
	require.Equal(t, http.StatusOK, res.Status)

	logs := env.Logs.String()
	for name, secret := range map[string]string{
		"client secret": oidcmock.ClientSecret, "link code": code, "link state": state, "flow cookie": link.Cookie,
		"login code": loginCode, "login state": loginState, "login flow cookie": start.Cookie,
	} {
		assert.NotContains(t, logs, secret, name)
		assert.NotContains(t, string(res.Raw), secret, name)
		assert.NotContains(t, string(linked.Raw), secret, name)
	}
	var data string
	require.NoError(t, env.Pool.QueryRow(context.Background(), `SELECT string_agg("Data"::text, ' ') FROM "AuditEvents"`).Scan(&data))
	assert.NotContains(t, data, code)
	assert.NotContains(t, data, loginState)
}
