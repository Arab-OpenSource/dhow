package e2e_test

import (
	"context"
	"encoding/base32"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Arab-OpenSource/dhow/apps/server/internal/App/Testing"
	i18n "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/I18n"
	mailer "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Mail"
	oidcmock "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Oidc/Mock"
)

const firefox = "Mozilla/5.0 (X11; Linux x86_64; rv:140.0) Gecko/20100101 Firefox/140.0"

func change(locale i18n.Locale, key string) string {
	return i18n.Translate(locale, "mail.alert.change."+key)
}

func to(messages []mailer.Message, email string) []mailer.Message {
	var out []mailer.Message
	for _, m := range messages {
		if strings.EqualFold(m.To, email) {
			out = append(out, m)
		}
	}
	return out
}

func subjects(messages []mailer.Message) []string {
	out := make([]string, 0, len(messages))
	for _, m := range messages {
		out = append(out, m.Subject)
	}
	return out
}

func expectedSubjects(locale i18n.Locale, keys ...string) []string {
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, strings.ReplaceAll(i18n.Translate(locale, "mail.alert.subject"), "{change}", change(locale, k)))
	}
	return out
}

func ok(t *testing.T, res apptest.Response) apptest.Response {
	t.Helper()
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	return res
}

func TestEveryCredentialChangeSendsOneAlertWithoutAnySecret(t *testing.T) {
	mail := &apptest.FakeMailer{}
	env, idp := apptest.NewWithIdP(t, apptest.OidcOptions{Pool: apptest.Options{Mailer: mail}})
	owner, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	dev, _ := env.SignedIn(org, "dev@example.com", "developer")
	require.Empty(t, env.SentMail(mail), "signing in sends no alert")

	newPassword := "N3w-passphrase-ZZTOP-changed"
	ok(t, owner.Do(apptest.Request{Method: http.MethodPut, Path: "/me/password", Body: map[string]string{"CurrentPassword": d.Password, "NewPassword": newPassword}}))
	enrolled := owner.EnableTotp(apptest.PasswordFactor(newPassword))
	codes := enrolled.RecoveryCodes
	require.GreaterOrEqual(t, len(codes), 8)
	factor := func(i int) map[string]any { return apptest.CodeFactor("recovery_code", codes[i]) }

	added := ok(t, owner.AddPasskey(passkey(t), "Laptop", factor(0), apptest.Override{}))
	var key struct{ Passkey struct{ Id string } }
	added.Decode(t, &key)
	ok(t, owner.Do(apptest.Request{Method: http.MethodPost, Path: "/me/passkeys/" + key.Passkey.Id + "/remove", Body: map[string]any{"CurrentFactor": factor(1)}}))

	linked := ok(t, owner.LinkOidc(idp, factor(2), oidcmock.Claims{Subject: "idp-user-1", Email: "sara@corp.example", EmailVerified: true, Name: "Sara"}))
	var identity struct{ Identity struct{ Id string } }
	linked.Decode(t, &identity)
	ok(t, owner.Do(apptest.Request{Method: http.MethodPost, Path: "/me/identities/" + identity.Identity.Id + "/remove", Body: map[string]any{"CurrentFactor": factor(3)}}))

	devTotp := dev.EnableTotp(apptest.PasswordFactor("a long password for dev@example.com"))
	ok(t, owner.Do(apptest.Request{
		Method: http.MethodPost, Path: apptest.OrgPath(org, "/members/"+dev.UserID+"/mfa-reset"), Body: map[string]any{"CurrentFactor": factor(4)},
	}))

	token := ok(t, owner.CreateAPIToken(org, "CI", []string{"org.read"}, 30, factor(5)))
	var created struct{ Value string }
	token.Decode(t, &created)
	require.NotEmpty(t, created.Value)

	regenerated := ok(t, owner.Do(apptest.Request{Method: http.MethodPost, Path: "/me/recovery-codes", Body: map[string]any{"CurrentFactor": factor(6)}}))
	var fresh struct{ RecoveryCodes []string }
	regenerated.Decode(t, &fresh)
	env.Clock.Advance(time.Minute)
	ok(t, owner.Do(apptest.Request{Method: http.MethodPost, Path: "/me/totp/disable", Body: map[string]any{
		"CurrentFactor": apptest.CodeFactor("totp", env.TotpCode(enrolled.Secret, 0)),
	}}))

	sent := env.SentMail(mail)
	assert.Equal(t, expectedSubjects(i18n.Ar,
		"password_changed", "totp_enabled", "passkey_added", "passkey_removed", "identity_linked", "identity_unlinked",
		"api_token_created", "recovery_codes_regenerated", "totp_disabled",
	), subjects(to(sent, d.Email)))
	assert.Equal(t, expectedSubjects(i18n.Ar, "totp_enabled", "mfa_reset"), subjects(to(sent, "dev@example.com")))
	assert.Len(t, sent, 11)

	b32 := base32.StdEncoding.WithPadding(base32.NoPadding)
	secrets := map[string]string{
		"old password": d.Password, "new password": newPassword, "marker": "ZZTOP", "API token": created.Value,
		"TOTP secret": b32.EncodeToString(enrolled.Secret), "developer TOTP secret": b32.EncodeToString(devTotp.Secret),
		"session cookie": owner.Cookie, "CSRF token": owner.CSRFToken,
	}
	for i, c := range append(append(append([]string(nil), codes...), fresh.RecoveryCodes...), devTotp.RecoveryCodes...) {
		secrets["recovery code "+strconv.Itoa(i)] = c
	}
	for _, m := range sent {
		for name, secret := range secrets {
			assert.NotContains(t, m.Subject+"\n"+m.Body, secret, "%s must not appear in an alert", name)
		}
	}
}

func passkey(t *testing.T) *apptest.Authenticator {
	t.Helper()
	a, err := apptest.NewAuthenticator()
	require.NoError(t, err)
	return a
}

func TestTheAlertSaysWhatChangedWhenAndFromWhereInTheUsersLanguage(t *testing.T) {
	mail := &apptest.FakeMailer{}
	env := apptest.NewWithOptions(t, apptest.Options{Mailer: mail})
	owner, d := env.SetupAndLogin()
	_, err := env.Pool.Exec(context.Background(), `UPDATE "Users" SET "Locale" = 'en', "Name" = 'Mona' WHERE "Email" = lower($1)`, d.Email)
	require.NoError(t, err)
	at := env.Clock.Now()

	ok(t, owner.Do(apptest.Request{
		Method: http.MethodPut, Path: "/me/password", RemoteAddr: "198.51.100.23:5000",
		Headers: map[string]string{"User-Agent": firefox, "Accept-Language": "ar"},
		Body:    map[string]string{"CurrentPassword": d.Password, "NewPassword": "another long passphrase here"},
	}))

	sent := env.SentMail(mail)
	require.Len(t, sent, 1)
	m := sent[0]
	assert.True(t, strings.EqualFold(d.Email, m.To))
	assert.Equal(t, "Security change on your Dhow account: Your password was changed", m.Subject, "the user's language, not the request's")
	for _, want := range []string{
		"Hello Mona", "Your password was changed.", at.UTC().Format("2006-01-02 15:04") + " (UTC)", "198.51.100.0/24",
		"Firefox · Linux", apptest.PublicOrigin + "/?stack=account",
	} {
		assert.Contains(t, m.Body, want)
	}
	assert.NotContains(t, m.Body, "198.51.100.23", "only the network is shown")
	assert.NotContains(t, m.Body, "{", "every placeholder is filled")
}

func TestAFailedAlertNeverStopsTheChange(t *testing.T) {
	mail := &apptest.FakeMailer{}
	env := apptest.NewWithOptions(t, apptest.Options{Mailer: mail})
	owner, d := env.SetupAndLogin()
	mail.FailNext(true)

	ok(t, owner.Do(apptest.Request{Method: http.MethodPut, Path: "/me/password", Body: map[string]string{"CurrentPassword": d.Password, "NewPassword": "another long passphrase here"}}))
	assert.Empty(t, env.SentMail(mail))
	assert.Equal(t, 1, env.AuditCount("auth.password_changed"), "the audit records the change in every case")
	assert.Contains(t, env.Logs.String(), "credential alert not sent")
	_, login := env.Login(d.Email, "another long passphrase here")
	assert.Equal(t, http.StatusOK, login.Status, "the new password is in place")
}

func TestWithoutEmailTheChangeIsOnlyAudited(t *testing.T) {
	env := apptest.New(t)
	owner, d := env.SetupAndLogin()
	ok(t, owner.Do(apptest.Request{Method: http.MethodPut, Path: "/me/password", Body: map[string]string{"CurrentPassword": d.Password, "NewPassword": "another long passphrase here"}}))
	env.Wired.Alerts.Wait()
	assert.Equal(t, 1, env.AuditCount("auth.password_changed"))
	assert.NotContains(t, env.Logs.String(), "credential alert")
}

func TestADisabledUserGetsNoAlert(t *testing.T) {
	mail := &apptest.FakeMailer{}
	env := apptest.NewWithOptions(t, apptest.Options{Mailer: mail})
	owner, d := env.SetupAndLogin()
	org := env.OrganizationID(d.OrgSlug)
	_, devID := env.SignedIn(org, "dev@example.com", "developer")
	env.SeedPasskey(devID)
	_, err := env.Pool.Exec(context.Background(), `UPDATE "Users" SET "Disabled" = true WHERE "Id" = $1`, devID)
	require.NoError(t, err)

	ok(t, owner.Do(apptest.Request{
		Method: http.MethodPost, Path: apptest.OrgPath(org, "/members/"+devID.String()+"/mfa-reset"),
		Body: map[string]any{"CurrentFactor": apptest.PasswordFactor(d.Password)},
	}))
	assert.Empty(t, to(env.SentMail(mail), "dev@example.com"))
}
