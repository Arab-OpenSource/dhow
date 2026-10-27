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

type stateChange struct {
	name    string
	request apptest.Request
}

func stateChanges(orgID, password string) []stateChange {
	return []stateChange{
		{"logout", apptest.Request{Method: http.MethodDelete, Path: "/sessions/current"}},
		{"revoke others", apptest.Request{Method: http.MethodPost, Path: "/sessions/revoke-others"}},
		{"change password", apptest.Request{Method: http.MethodPut, Path: "/me/password", Body: map[string]string{
			"CurrentPassword": password, "NewPassword": "a brand new passphrase",
		}}},
		{"rename organization", apptest.Request{Method: http.MethodPatch, Path: "/organizations/" + orgID, Body: map[string]string{"Name": "Renamed"}}},
	}
}

func TestStateChangesNeedAValidCSRFTokenAndOrigin(t *testing.T) {
	env := apptest.New(t)
	client, d := env.SetupAndLogin()
	other, _ := env.Login(d.Email, d.Password)
	orgID := env.OrganizationID(d.OrgSlug).String()

	variants := []struct {
		name    string
		headers map[string]string
	}{
		{"no CSRF token", map[string]string{config.CSRFHeaderName: ""}},
		{"empty CSRF token", map[string]string{config.CSRFHeaderName: " "}},
		{"a made-up CSRF token", map[string]string{config.CSRFHeaderName: strings.Repeat("A", 43)}},
		{"the CSRF token of another session", map[string]string{config.CSRFHeaderName: other.CSRFToken}},
		{"a truncated CSRF token", map[string]string{config.CSRFHeaderName: client.CSRFToken[:20]}},
		{"a foreign origin", map[string]string{"Origin": "https://evil.example"}},
		{"a sub-domain of the origin", map[string]string{"Origin": "https://evil.dhow.example.com"}},
		{"no origin", map[string]string{"Origin": ""}},
		{"origin null", map[string]string{"Origin": "null"}},
		{"http scheme", map[string]string{"Origin": "http://dhow.example.com"}},
	}
	for _, sc := range stateChanges(orgID, d.Password) {
		for _, v := range variants {
			t.Run(sc.name+"/"+v.name, func(t *testing.T) {
				req := sc.request
				req.Headers = v.headers
				res := client.Do(req)
				assert.Equal(t, http.StatusForbidden, res.Status, string(res.Raw))
				require.NotNil(t, res.Error)
				assert.Equal(t, "CSRF_FAILED", res.Error.Code)
			})
		}
	}
	assert.Equal(t, http.StatusOK, client.Get("/me").Status, "no rejected request changed anything")
	assert.Equal(t, http.StatusOK, other.Get("/me").Status)
	var org struct{ Name string }
	client.Get("/organizations/"+orgID).Decode(t, &org)
	assert.Equal(t, d.OrgName, org.Name)
}

func TestStateChangesSucceedWithTheTokenFromLoginAndFromMe(t *testing.T) {
	env := apptest.New(t)
	client, d := env.SetupAndLogin()
	orgID := env.OrganizationID(d.OrgSlug).String()

	var me meData
	client.Get("/me").Decode(t, &me)
	client.CSRFToken = me.CsrfToken
	res := client.Do(apptest.Request{Method: http.MethodPatch, Path: "/organizations/" + orgID, Body: map[string]string{"Name": "Renamed"}})
	assert.Equal(t, http.StatusOK, res.Status, string(res.Raw))
}

func TestReadsNeedNoCSRFTokenButStillNeedTheSession(t *testing.T) {
	env := apptest.New(t)
	client, _ := env.SetupAndLogin()
	res := client.Do(apptest.Request{Method: http.MethodGet, Path: "/me", Headers: map[string]string{config.CSRFHeaderName: "", "Origin": ""}})
	assert.Equal(t, http.StatusOK, res.Status)
}

func TestLoginAndSetupCheckTheOriginButNeedNoToken(t *testing.T) {
	env := apptest.New(t)
	setupBody := func(token string) map[string]string {
		return map[string]string{
			"SetupToken": token, "OwnerName": "Mona", "Email": "mona@example.com", "Password": "correct horse battery staple",
			"OrganizationName": "Dhow", "OrganizationSlug": "dhow",
		}
	}
	token := env.SetupToken()
	for name, origin := range map[string]string{"foreign": "https://evil.example", "missing": "", "null": "null"} {
		res := env.Do(apptest.Request{Method: http.MethodPost, Path: "/setup", Headers: map[string]string{"Origin": origin}, Body: setupBody(token)})
		assert.Equal(t, http.StatusForbidden, res.Status, "setup with a %s origin", name)
		assert.Equal(t, "CSRF_FAILED", res.Error.Code)
	}
	assert.Equal(t, http.StatusOK, env.Do(apptest.Request{
		Method: http.MethodPost, Path: "/setup",
		Headers: map[string]string{"Origin": apptest.PublicOrigin}, Body: setupBody(token),
	}).Status, "the rejected attempts did not burn the token")

	login := map[string]string{"Email": "mona@example.com", "Password": "correct horse battery staple"}
	for name, origin := range map[string]string{"foreign": "https://evil.example", "missing": "", "null": "null"} {
		res := env.Do(apptest.Request{Method: http.MethodPost, Path: "/sessions", Headers: map[string]string{"Origin": origin}, Body: login})
		assert.Equal(t, http.StatusForbidden, res.Status, "login with a %s origin", name)
		assert.Equal(t, "CSRF_FAILED", res.Error.Code)
		assert.Empty(t, res.Header.Get("Set-Cookie"))
	}
	ok := env.Do(apptest.Request{Method: http.MethodPost, Path: "/sessions", Headers: map[string]string{"Origin": apptest.PublicOrigin}, Body: login})
	assert.Equal(t, http.StatusOK, ok.Status, "login needs the origin but no CSRF token")
}

func TestSessionCookieAttributes(t *testing.T) {
	env := apptest.New(t)
	client, login := env.SetupAndLoginResponse()
	cookies := login.Header.Values("Set-Cookie")
	require.Len(t, cookies, 1)
	c := cookies[0]
	assert.True(t, strings.HasPrefix(c, "__Host-dhow_session="), c)
	attrs := strings.Split(c, "; ")[1:]
	assert.ElementsMatch(t, []string{"Path=/", "Max-Age=1209600", "HttpOnly", "Secure", "SameSite=Strict"}, attrs, "no Domain, as the __Host- prefix requires")
	assert.Len(t, client.Cookie, 43, "32 random bytes, base64url")
}

func TestEveryLoginIssuesAFreshSessionID(t *testing.T) {
	env := apptest.New(t)
	first, d := env.SetupAndLogin()
	second, _ := env.Login(d.Email, d.Password)
	assert.NotEqual(t, first.Cookie, second.Cookie)
	assert.NotEqual(t, first.CSRFToken, second.CSRFToken)

	again := &apptest.Client{Env: env, Cookie: first.Cookie, CSRFToken: first.CSRFToken}
	res := again.Do(apptest.Request{Method: http.MethodPost, Path: "/sessions", Body: map[string]string{"Email": d.Email, "Password": d.Password}})
	require.Equal(t, http.StatusOK, res.Status, string(res.Raw))
	renewed := apptest.CookieValue(t, res.Header.Get("Set-Cookie"))
	assert.NotEqual(t, first.Cookie, renewed, "logging in while holding a cookie rotates it")
	assert.Equal(t, http.StatusUnauthorized, first.Get("/me").Status, "and the old one is revoked, not reused")
}
