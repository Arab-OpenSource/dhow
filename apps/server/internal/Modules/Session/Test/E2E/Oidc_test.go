package e2e_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Arab-OpenSource/dhow/apps/server/internal/App/Testing"
	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
	oidcmock "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Oidc/Mock"
)

const totpStep = 30 * time.Second

func sara() oidcmock.Claims {
	return oidcmock.Claims{Subject: "idp-user-1", Email: "sara@corp.example", EmailVerified: true, Name: "Sara"}
}

func ownerWithIdP(t *testing.T, o apptest.OidcOptions) (*apptest.Env, *oidcmock.IdP, *apptest.Client, apptest.SetupDetails) {
	t.Helper()
	env, idp := apptest.NewWithIdP(t, o)
	owner, d := env.SetupAndLogin()
	return env, idp, owner, d
}

func identityCount(t *testing.T, env *apptest.Env) int {
	t.Helper()
	var n int
	require.NoError(t, env.Pool.QueryRow(context.Background(), `SELECT count(*) FROM "UserIdentities" WHERE NOT "IsDeleted"`).Scan(&n))
	return n
}

func TestOidcProvidersArePublicAndCarryNoSecret(t *testing.T) {
	env, _ := apptest.NewWithIdP(t, apptest.OidcOptions{})
	res := env.Do(apptest.Request{Method: http.MethodGet, Path: "/oidc/providers"})
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	assert.JSONEq(t, `[{"Id":"corp","Name":"Corp SSO","Kind":"oidc"}]`, string(res.Data))
	assert.NotContains(t, string(res.Raw), oidcmock.ClientSecret)

	plain := apptest.New(t)
	none := plain.Do(apptest.Request{Method: http.MethodGet, Path: "/oidc/providers"})
	require.Equal(t, http.StatusOK, none.Status)
	assert.JSONEq(t, `[]`, string(none.Data))
}

func TestOidcLinkThenSignIn(t *testing.T) {
	env, idp, owner, d := ownerWithIdP(t, apptest.OidcOptions{})
	other, _ := env.Login(d.Email, d.Password)

	linked := owner.LinkOidc(idp, apptest.PasswordFactor(d.Password), sara())
	require.Equal(t, http.StatusOK, linked.Status, string(linked.Raw))
	assert.Equal(t, "oidc.identity.linked", linked.Message)
	var change struct {
		Identity struct{ Id, Provider, ProviderName, Email string }
		Revoked  int
	}
	linked.Decode(t, &change)
	assert.Equal(t, "corp", change.Identity.Provider)
	assert.Equal(t, "Corp SSO", change.Identity.ProviderName)
	assert.Equal(t, "sara@corp.example", change.Identity.Email)
	assert.Equal(t, 1, change.Revoked)
	assert.False(t, env.SessionAlive(other), "linking a sign-in method signs out the other sessions")
	_, raw, ok := apptest.SetCookie(linked.Header, config.OidcCookieName)
	require.True(t, ok)
	assert.Contains(t, raw, "Max-Age=0")

	list := owner.Get("/me/identities")
	require.Equal(t, http.StatusOK, list.Status)
	var identities []struct{ Id, Provider string }
	list.Decode(t, &identities)
	require.Len(t, identities, 1)

	signedIn, res := env.OidcLogin(idp, sara())
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	assert.Equal(t, "session.login.succeeded", res.Message)
	_, flowRaw, ok := apptest.SetCookie(res.Header, config.OidcCookieName)
	require.True(t, ok, "the callback clears the flow cookie")
	assert.Contains(t, flowRaw, "Max-Age=0")
	me := signedIn.Me()
	assert.Equal(t, "owner@example.com", me.User.Email)
	assert.Equal(t, 1, env.AuditCount("auth.identity_linked"))
	var method string
	require.NoError(t, env.Pool.QueryRow(context.Background(),
		`SELECT "Data"->>'Method' FROM "AuditEvents" WHERE "Action" = 'auth.login' ORDER BY "Seq" DESC LIMIT 1`).Scan(&method))
	assert.Equal(t, "oidc:corp", method)
}

func TestOidcSignInRotatesTheSessionTheBrowserCarried(t *testing.T) {
	env, idp, owner, d := ownerWithIdP(t, apptest.OidcOptions{})
	require.Equal(t, http.StatusOK, owner.LinkOidc(idp, apptest.PasswordFactor(d.Password), sara()).Status)

	start := owner.BeginOidc("/oidc/authorize", map[string]string{"Provider": apptest.OidcProvider})
	require.Equal(t, http.StatusOK, start.Res.Status, string(start.Res.Raw))
	code, state := idp.Authorize(start.URL, sara())
	res := owner.FinishOidc("/oidc/callback", start.Cookie, code, state, nil)
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	fresh, _, ok := apptest.SetCookie(res.Header, config.SessionCookieName)
	require.True(t, ok)
	assert.NotEqual(t, owner.Cookie, fresh)
	assert.False(t, env.SessionAlive(owner), "the session the request carried is revoked")
}

func TestOidcSignInGoesThroughTheSecondStep(t *testing.T) {
	env, idp, owner, d := ownerWithIdP(t, apptest.OidcOptions{})
	enrollment := owner.EnableTotp(apptest.PasswordFactor(d.Password))
	nextCode := func() map[string]any {
		env.Clock.Advance(totpStep)
		return apptest.CodeFactor("totp", env.TotpCode(enrollment.Secret, 0))
	}
	linked := owner.LinkOidc(idp, nextCode(), sara())
	require.Equal(t, http.StatusOK, linked.Status, string(linked.Raw))

	anonymous, pending := env.OidcLogin(idp, sara())
	require.Equal(t, http.StatusAccepted, pending.Status, string(pending.Raw))
	assert.Empty(t, anonymous.Cookie, "no session before the second step")
	_, _, ok := apptest.SetCookie(pending.Header, config.SessionCookieName)
	assert.False(t, ok)
	challenge, _, ok := apptest.SetCookie(pending.Header, config.MfaCookieName)
	require.True(t, ok)
	_, flowRaw, ok := apptest.SetCookie(pending.Header, config.OidcCookieName)
	require.True(t, ok)
	assert.Contains(t, flowRaw, "Max-Age=0")
	var data struct{ Methods []string }
	pending.Decode(t, &data)
	assert.Contains(t, data.Methods, "totp")

	wrong, bad := env.CompleteMfa(challenge, apptest.CodeFactor("totp", "000000"), nil)
	assert.Equal(t, http.StatusUnauthorized, bad.Status)
	assert.Empty(t, wrong.Cookie)
	signedIn, done := env.CompleteMfa(challenge, nextCode(), nil)
	require.Equal(t, http.StatusOK, done.Status, string(done.Raw))
	assert.NotEmpty(t, signedIn.Cookie)
	assert.Equal(t, "owner@example.com", signedIn.Me().User.Email)
}

func TestOidcNeverTakesOverAnAccountByEmail(t *testing.T) {
	env, idp, _, d := ownerWithIdP(t, apptest.OidcOptions{})
	claims := oidcmock.Claims{Subject: "attacker", Email: "owner@example.com", EmailVerified: true}
	anonymous, res := env.OidcLogin(idp, claims)
	assert.Equal(t, http.StatusUnauthorized, res.Status, string(res.Raw))
	require.NotNil(t, res.Error)
	assert.Equal(t, "OIDC_NOT_LINKED", res.Error.Code)
	assert.Empty(t, anonymous.Cookie)
	_, _, ok := apptest.SetCookie(res.Header, config.SessionCookieName)
	assert.False(t, ok)
	assert.Zero(t, identityCount(t, env), "no identity is linked on sign-in")
	assert.Equal(t, 1, env.AuditCount("auth.oidc_login_failed"))
	var orgEvents int
	require.NoError(t, env.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM "AuditEvents" WHERE "Action" = 'auth.oidc_login_failed' AND "OrganizationId" IS NOT NULL`).Scan(&orgEvents))
	assert.Zero(t, orgEvents, "an unknown identity is recorded on the instance chain")
	_, login := env.Login(d.Email, d.Password)
	assert.Equal(t, http.StatusOK, login.Status, "the real owner is unaffected")
}

func TestGitHubLinksByNumericID(t *testing.T) {
	env, idp, owner, d := ownerWithIdP(t, apptest.OidcOptions{GitHub: true})
	octo := oidcmock.Claims{Subject: "583231", Name: "Octo", GitHubEmails: []oidcmock.GitHubEmail{{Email: "octo@example.com", Primary: true, Verified: true}}}
	require.Equal(t, http.StatusOK, owner.LinkOidc(idp, apptest.PasswordFactor(d.Password), octo).Status)
	var issuer, subject string
	require.NoError(t, env.Pool.QueryRow(context.Background(), `SELECT "Issuer", "Subject" FROM "UserIdentities"`).Scan(&issuer, &subject))
	assert.Equal(t, idp.Issuer(), issuer)
	assert.Equal(t, "583231", subject)

	_, res := env.OidcLogin(idp, octo)
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	renamed := octo
	renamed.Subject = "583232"
	_, other := env.OidcLogin(idp, renamed)
	assert.Equal(t, http.StatusUnauthorized, other.Status)
}

func TestUnlinkNeedsTheFactorAndStopsSignIn(t *testing.T) {
	env, idp, owner, d := ownerWithIdP(t, apptest.OidcOptions{})
	linked := owner.LinkOidc(idp, apptest.PasswordFactor(d.Password), sara())
	require.Equal(t, http.StatusOK, linked.Status)
	var change struct{ Identity struct{ Id string } }
	linked.Decode(t, &change)
	path := "/me/identities/" + change.Identity.Id + "/remove"

	wrong := owner.Do(apptest.Request{Method: http.MethodPost, Path: path, Body: map[string]any{"CurrentFactor": apptest.PasswordFactor("not the password")}})
	assert.Equal(t, http.StatusForbidden, wrong.Status)
	assert.Equal(t, "INVALID_CURRENT_FACTOR", wrong.Error.Code)

	stranger, _ := env.SignedIn(env.OrganizationID(d.OrgSlug), "stranger@example.com", "viewer")
	theirs := stranger.Do(apptest.Request{Method: http.MethodPost, Path: path, Body: map[string]any{"CurrentFactor": apptest.PasswordFactor("a long password for stranger@example.com")}})
	assert.Equal(t, http.StatusNotFound, theirs.Status, "another user's identity looks absent")

	res := owner.Do(apptest.Request{Method: http.MethodPost, Path: path, Body: map[string]any{"CurrentFactor": apptest.PasswordFactor(d.Password)}})
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	assert.Zero(t, identityCount(t, env))
	assert.Equal(t, 1, env.AuditCount("auth.identity_unlinked"))
	_, login := env.OidcLogin(idp, sara())
	assert.Equal(t, http.StatusUnauthorized, login.Status)
	assert.Equal(t, "OIDC_NOT_LINKED", login.Error.Code)
}

func TestTheLastSignInMethodCannotBeUnlinked(t *testing.T) {
	env, idp, owner, d := ownerWithIdP(t, apptest.OidcOptions{})
	enrollment := owner.EnableTotp(apptest.PasswordFactor(d.Password))
	env.Clock.Advance(totpStep)
	linked := owner.LinkOidc(idp, apptest.CodeFactor("totp", env.TotpCode(enrollment.Secret, 0)), sara())
	require.Equal(t, http.StatusOK, linked.Status, string(linked.Raw))
	var change struct{ Identity struct{ Id string } }
	linked.Decode(t, &change)
	_, err := env.Pool.Exec(context.Background(), `UPDATE "Users" SET "PasswordHash" = '' WHERE "Email" = 'owner@example.com'`)
	require.NoError(t, err)
	env.Clock.Advance(totpStep)
	res := owner.Do(apptest.Request{
		Method: http.MethodPost, Path: "/me/identities/" + change.Identity.Id + "/remove",
		Body: map[string]any{"CurrentFactor": apptest.CodeFactor("totp", env.TotpCode(enrollment.Secret, 0))},
	})
	assert.Equal(t, http.StatusConflict, res.Status, string(res.Raw))
	require.NotNil(t, res.Error)
	assert.Equal(t, "LAST_SIGN_IN_METHOD", res.Error.Code)
	assert.Equal(t, 1, identityCount(t, env), "the identity stays linked")
}
