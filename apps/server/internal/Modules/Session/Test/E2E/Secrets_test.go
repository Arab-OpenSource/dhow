package e2e_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Arab-OpenSource/dhow/apps/server/internal/App/Testing"
	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
)

func TestNoSecretReachesAnyResponseOrLogLine(t *testing.T) {
	env := apptest.New(t)
	d := apptest.DefaultSetup()
	d.Password = "S3cret-passphrase-ZZTOP-setup"
	token := env.SetupToken()
	d.Token = token

	var bodies []string
	record := func(res apptest.Response) apptest.Response {
		bodies = append(bodies, string(res.Raw))
		for k, v := range res.Header {
			if !strings.EqualFold(k, "Set-Cookie") {
				bodies = append(bodies, k+": "+strings.Join(v, ","))
			}
		}
		return res
	}
	require.Equal(t, http.StatusOK, record(env.Setup(d)).Status)

	wrong := record(env.Do(apptest.Request{
		Method: http.MethodPost, Path: "/sessions", Headers: map[string]string{"Origin": apptest.PublicOrigin},
		Body: map[string]string{"Email": d.Email, "Password": "Wr0ng-passphrase-ZZTOP-failed"},
	}))
	require.Equal(t, http.StatusUnauthorized, wrong.Status)

	client, login := env.Login(d.Email, d.Password)
	record(login)
	record(client.Get("/me"))
	record(client.Get("/sessions"))
	newPassword := "N3w-passphrase-ZZTOP-changed"
	record(client.Do(apptest.Request{Method: http.MethodPut, Path: "/me/password", Body: map[string]string{"CurrentPassword": d.Password, "NewPassword": newPassword}}))
	record(client.Do(apptest.Request{Method: http.MethodPut, Path: "/me/password", Body: map[string]string{"CurrentPassword": "Bad-current-ZZTOP-rejected", "NewPassword": newPassword}}))
	orgID := env.OrganizationID(d.OrgSlug).String()
	events := record(client.Get("/organizations/" + orgID + "/audit-events?Limit=100"))
	record(client.Get("/organizations/" + orgID + "/audit-events/verify"))
	record(client.Get("/organizations/" + orgID + "/members"))
	record(env.Logout(client))

	secrets := map[string]string{
		"setup password": "S3cret-passphrase", "failed password": "Wr0ng-passphrase", "new password": "N3w-passphrase",
		"rejected current password": "Bad-current", "setup token": token, "session cookie": client.Cookie,
		"password hash": "$argon2id", "the shared marker": "ZZTOP",
	}
	logs := env.Logs.String()
	require.NotEmpty(t, logs)
	for name, secret := range secrets {
		assert.NotContains(t, logs, secret, "%s must not appear in any log line", name)
		for _, body := range bodies {
			assert.NotContains(t, body, secret, "%s must not appear in any response", name)
		}
	}
	assert.NotContains(t, logs, client.CSRFToken, "the CSRF token is never logged")
	assert.Contains(t, events.Message, "audit.events.listed")
	assert.NotContains(t, string(events.Raw), `"Password`)
	assert.NotContains(t, string(events.Raw), "Hash")
}

func TestTheSessionCookieValueOnlyTravelsInSetCookie(t *testing.T) {
	env := apptest.New(t)
	client, login := env.SetupAndLoginResponse()
	assert.NotContains(t, string(login.Raw), client.Cookie)
	for k, v := range login.Header {
		if k != "Set-Cookie" {
			assert.NotContains(t, strings.Join(v, ","), client.Cookie, k)
		}
	}
	assert.Contains(t, login.Header.Get("Set-Cookie"), config.SessionCookieName+"="+client.Cookie)
	assert.Equal(t, "no-store", login.Header.Get("Cache-Control"), "auth responses are never cached")
}
