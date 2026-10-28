package e2e_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
)

func authenticator(t *testing.T) *apptest.Authenticator {
	t.Helper()
	a, err := apptest.NewAuthenticator()
	require.NoError(t, err)
	return a
}

func withPasskey(t *testing.T) (*apptest.Env, *apptest.Client, apptest.SetupDetails, *apptest.Authenticator) {
	t.Helper()
	env := apptest.New(t)
	c, d := env.SetupAndLogin()
	a := authenticator(t)
	res := c.AddPasskey(a, "MacBook Touch ID", apptest.PasswordFactor(d.Password), apptest.Override{})
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	return env, c, d, a
}

type passkeyView struct {
	Id, Name, CreatedAt, LastUsedAt string
	Transports                      []string
	Synced                          bool
}

func TestRegisteringAPasskeyStoresItWithItsCounterAndTransports(t *testing.T) {
	env := apptest.New(t)
	c, d := env.SetupAndLogin()
	other, _ := env.Login(d.Email, d.Password)
	a := authenticator(t)
	a.SignCount = 7
	a.Synced = true

	noFactor := c.AddPasskey(a, "Key", apptest.PasswordFactor("wrong password entirely"), apptest.Override{})
	assert.Equal(t, "INVALID_CURRENT_FACTOR", noFactor.Error.Code)

	res := c.AddPasskey(a, "  MacBook Touch ID  ", apptest.PasswordFactor(d.Password), apptest.Override{})
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	var added struct {
		Passkey       passkeyView
		Revoked       int
		RecoveryCodes []string
	}
	res.Decode(t, &added)
	assert.Equal(t, "MacBook Touch ID", added.Passkey.Name)
	assert.ElementsMatch(t, []string{"internal", "hybrid"}, added.Passkey.Transports)
	assert.True(t, added.Passkey.Synced)
	assert.Equal(t, 1, added.Revoked)
	assert.Len(t, added.RecoveryCodes, 10)
	assert.Equal(t, http.StatusUnauthorized, other.Get("/me").Status)

	var stored struct {
		SignCount  int64
		Transports []string
	}
	require.NoError(t, env.Pool.QueryRow(context.Background(), `SELECT "SignCount", "Transports" FROM "Passkeys"`).Scan(&stored.SignCount, &stored.Transports))
	assert.Equal(t, int64(7), stored.SignCount)
	assert.ElementsMatch(t, []string{"internal", "hybrid"}, stored.Transports)

	var list []passkeyView
	c.Get("/me/passkeys").Decode(t, &list)
	require.Len(t, list, 1)
	assert.Equal(t, added.Passkey.Id, list[0].Id)
	assert.Equal(t, 1, env.AuditCount("auth.passkey_added"))
}

func TestPasswordlessSignInWithAPasskey(t *testing.T) {
	env, _, _, a := withPasskey(t)
	a.SignCount = 3
	c, res := env.PasskeyLogin(a, apptest.Override{})
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	var me meData
	c.Get("/me").Decode(t, &me)
	assert.Equal(t, "owner@example.com", me.User.Email)

	var count int64
	var lastUsed *time.Time
	require.NoError(t, env.Pool.QueryRow(context.Background(), `SELECT "SignCount", "LastUsedAt" FROM "Passkeys"`).Scan(&count, &lastUsed))
	assert.Equal(t, int64(3), count, "the new counter is stored")
	assert.NotNil(t, lastUsed)
	var method string
	require.NoError(t, env.Pool.QueryRow(context.Background(),
		`SELECT "Data"->>'Method' FROM "AuditEvents" WHERE "Action" = 'auth.login' ORDER BY "Seq" DESC LIMIT 1`).Scan(&method))
	assert.Equal(t, "passkey", method)
}

func TestAPasskeyIsAlsoASecondStep(t *testing.T) {
	env, _, d, a := withPasskey(t)
	step := env.PasswordStep(d.Email, d.Password, nil)
	require.Equal(t, http.StatusAccepted, step.Status)
	var pending struct{ Methods []string }
	step.Decode(t, &pending)
	assert.ElementsMatch(t, []string{"passkey", "recovery_code"}, pending.Methods)

	cookie := env.StartMfa(d.Email, d.Password)
	c, res := env.CompleteMfa(cookie, env.MfaPasskeyAssertion(cookie, a, apptest.Override{}), nil)
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	assert.Equal(t, http.StatusOK, c.Get("/me").Status)

	stale := env.StartMfa(d.Email, d.Password)
	_, res = env.CompleteMfa(stale, map[string]any{"Method": "passkey"}, nil)
	assert.Equal(t, "INVALID_MFA_CODE", res.Error.Code, "an assertion needs the options call first")
}

func TestPasskeyAssertionsFromTheWrongOriginOrRPIDAreRefused(t *testing.T) {
	env, _, d, a := withPasskey(t)
	tests := []struct {
		name string
		o    apptest.Override
	}{
		{"wrong origin", apptest.Override{Origin: "https://evil.example"}},
		{"look-alike origin", apptest.Override{Origin: "https://dhow.example.com.evil.example"}},
		{"plain http origin", apptest.Override{Origin: "http://dhow.example.com"}},
		{"wrong RP ID", apptest.Override{RPID: "evil.example"}},
		{"parent domain as RP ID", apptest.Override{RPID: "example.com"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, res := env.PasskeyLogin(a, tt.o)
			assert.Equal(t, http.StatusUnauthorized, res.Status, string(res.Raw))
			assert.Equal(t, "INVALID_CREDENTIALS", res.Error.Code)
			assert.Empty(t, res.Header.Values("Set-Cookie"))

			cookie := env.StartMfa(d.Email, d.Password)
			_, second := env.CompleteMfa(cookie, env.MfaPasskeyAssertion(cookie, a, tt.o), nil)
			assert.Equal(t, "INVALID_MFA_CODE", second.Error.Code)
			env.Clock.Advance(time.Hour)
		})
	}

	env.Clock.Advance(time.Hour)
	owner, ownerLogin := env.PasskeyLogin(a, apptest.Override{})
	require.Equal(t, http.StatusOK, ownerLogin.Status, string(ownerLogin.Raw))
	b := authenticator(t)
	res := owner.AddPasskey(b, "Wrong origin key", owner.PasskeyFactor(a), apptest.Override{Origin: "https://evil.example"})
	assert.Equal(t, http.StatusUnprocessableEntity, res.Status, string(res.Raw))
	assert.Equal(t, "PASSKEY_REJECTED", res.Error.Code)
	res = owner.AddPasskey(b, "Wrong RP key", owner.PasskeyFactor(a), apptest.Override{RPID: "evil.example"})
	assert.Equal(t, "PASSKEY_REJECTED", res.Error.Code)
}

func TestASignCountRegressionIsRefusedAndAudited(t *testing.T) {
	env, _, _, a := withPasskey(t)
	a.SignCount = 10
	_, res := env.PasskeyLogin(a, apptest.Override{})
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))

	a.SignCount = 4
	_, res = env.PasskeyLogin(a, apptest.Override{})
	assert.Equal(t, "INVALID_CREDENTIALS", res.Error.Code, "a counter that goes backwards suggests a cloned authenticator")
	a.SignCount = 10
	_, res = env.PasskeyLogin(a, apptest.Override{})
	assert.Equal(t, "INVALID_CREDENTIALS", res.Error.Code, "an equal counter is a regression too")
	assert.Equal(t, 2, env.AuditCount("auth.passkey_sign_count_regressed"))

	a.SignCount = 11
	_, res = env.PasskeyLogin(a, apptest.Override{})
	assert.Equal(t, http.StatusOK, res.Status, "a higher counter works again")
}

func TestPasskeyChallengesWorkOnceAndExpire(t *testing.T) {
	env, _, _, a := withPasskey(t)
	anonymous := &apptest.Client{Env: env}
	ch := anonymous.PasskeyChallenge("/sessions/passkey-options")
	credential, err := a.Assert(ch.Options, apptest.PublicOrigin, apptest.Override{})
	require.NoError(t, err)
	body := map[string]any{"ChallengeId": ch.ChallengeId, "Credential": credential}
	post := func() apptest.Response {
		return env.Do(apptest.Request{Method: http.MethodPost, Path: "/sessions/passkey", Headers: map[string]string{"Origin": apptest.PublicOrigin}, Body: body})
	}
	require.Equal(t, http.StatusOK, post().Status)
	assert.Equal(t, "INVALID_CREDENTIALS", post().Error.Code, "the same challenge never works twice")

	late := anonymous.PasskeyChallenge("/sessions/passkey-options")
	credential, err = a.Assert(late.Options, apptest.PublicOrigin, apptest.Override{})
	require.NoError(t, err)
	env.Clock.Advance(5*time.Minute + time.Second)
	res := env.Do(apptest.Request{
		Method: http.MethodPost, Path: "/sessions/passkey", Headers: map[string]string{"Origin": apptest.PublicOrigin},
		Body: map[string]any{"ChallengeId": late.ChallengeId, "Credential": credential},
	})
	assert.Equal(t, "INVALID_CREDENTIALS", res.Error.Code, "an expired challenge is refused")
}

func TestRemovingAPasskeyNeedsACurrentFactor(t *testing.T) {
	env, c, d, a := withPasskey(t)
	other, _ := env.PasskeyLogin(a, apptest.Override{})
	var list []passkeyView
	c.Get("/me/passkeys").Decode(t, &list)
	require.Len(t, list, 1)
	remove := func(id string, factor map[string]any) apptest.Response {
		return c.Do(apptest.Request{Method: http.MethodPost, Path: "/me/passkeys/" + id + "/remove", Body: map[string]any{"CurrentFactor": factor}})
	}

	res := remove(list[0].Id, apptest.PasswordFactor(d.Password))
	assert.Equal(t, "INVALID_CURRENT_FACTOR", res.Error.Code)
	res = remove("00000000-0000-7000-8000-00000000abcd", c.PasskeyFactor(a))
	assert.Equal(t, http.StatusNotFound, res.Status)

	res = remove(list[0].Id, c.PasskeyFactor(a))
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	assert.Equal(t, http.StatusUnauthorized, other.Get("/me").Status)
	var status struct {
		Enabled                bool
		RecoveryCodesRemaining int
	}
	c.Get("/me/mfa").Decode(t, &status)
	assert.False(t, status.Enabled)
	assert.Zero(t, status.RecoveryCodesRemaining)
	_, res = env.PasskeyLogin(a, apptest.Override{})
	assert.Equal(t, "INVALID_CREDENTIALS", res.Error.Code, "a removed passkey no longer signs in")
	assert.Equal(t, 1, env.AuditCount("auth.passkey_removed"))
}

func TestPasskeysBelongToTheirOwner(t *testing.T) {
	env, c, d, _ := withPasskey(t)
	var list []passkeyView
	c.Get("/me/passkeys").Decode(t, &list)
	require.Len(t, list, 1)
	env.SeedUser(env.OrganizationID(d.OrgSlug), "second@example.com", "second user passphrase", "admin")
	second, _ := env.Login("second@example.com", "second user passphrase")

	rename := second.Do(apptest.Request{Method: http.MethodPatch, Path: "/me/passkeys/" + list[0].Id, Body: map[string]string{"Name": "Mine now"}})
	assert.Equal(t, http.StatusNotFound, rename.Status)
	removal := second.Do(apptest.Request{
		Method: http.MethodPost, Path: "/me/passkeys/" + list[0].Id + "/remove",
		Body: map[string]any{"CurrentFactor": apptest.PasswordFactor("second user passphrase")},
	})
	assert.Equal(t, http.StatusNotFound, removal.Status)
	var mine []passkeyView
	second.Get("/me/passkeys").Decode(t, &mine)
	assert.Empty(t, mine)

	ok := c.Do(apptest.Request{Method: http.MethodPatch, Path: "/me/passkeys/" + list[0].Id, Body: map[string]string{"Name": "YubiKey 5C"}})
	require.Equal(t, http.StatusOK, ok.Status, string(ok.Raw))
	bad := c.Do(apptest.Request{Method: http.MethodPatch, Path: "/me/passkeys/" + list[0].Id, Body: map[string]string{"Name": " "}})
	assert.Equal(t, http.StatusUnprocessableEntity, bad.Status)

	a2 := authenticator(t)
	ch := c.PasskeyChallenge("/me/passkeys/options")
	credential, err := a2.Register(ch.Options, apptest.PublicOrigin, apptest.Override{})
	require.NoError(t, err)
	stolen := second.Do(apptest.Request{Method: http.MethodPost, Path: "/me/passkeys", Body: map[string]any{
		"ChallengeId": ch.ChallengeId, "Credential": credential, "Name": "Stolen", "CurrentFactor": apptest.PasswordFactor("second user passphrase"),
	}})
	assert.Equal(t, "ENROLLMENT_EXPIRED", stolen.Error.Code, "a registration challenge belongs to the session that asked for it")
}
