package edge_test

import (
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Arab-OpenSource/dhow/apps/server/internal/App/Testing"
	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
	auth "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Auth"
)

func login(env *apptest.Env, email, password string) apptest.Response {
	env.T.Helper()
	return env.Do(apptest.Request{
		Method: http.MethodPost, Path: "/sessions", Headers: map[string]string{"Origin": apptest.PublicOrigin},
		Body: map[string]string{"Email": email, "Password": password},
	})
}

func TestEveryFailedLoginLooksTheSame(t *testing.T) {
	env := apptest.New(t)
	d := apptest.DefaultSetup()
	require.Equal(t, http.StatusOK, env.Setup(d).Status)
	orgID := env.OrganizationID(d.OrgSlug)
	env.SeedUser(orgID, "disabled@example.com", "disabled user password", "viewer")
	_, err := env.Pool.Exec(t.Context(), `UPDATE "Users" SET "Disabled" = true WHERE "Email" = 'disabled@example.com'`)
	require.NoError(t, err)

	cases := map[string]apptest.Response{
		"unknown user":               login(env, "nobody@example.com", "some long password"),
		"wrong password":             login(env, d.Email, "some long password"),
		"empty password":             login(env, d.Email, ""),
		"empty email":                login(env, "", "some long password"),
		"disabled user":              login(env, "disabled@example.com", "disabled user password"),
		"malformed email":            login(env, "a@b@c", "some long password"),
		"very long email":            login(env, strings.Repeat("a", 400)+"@example.com", "some long password"),
		"very long password":         login(env, d.Email, strings.Repeat("p", 3000)),
		"email with surrounding gap": login(env, "  nobody@example.com  ", "some long password"),
		"mixed case unknown":         login(env, "NoBody@Example.com", "some long password"),
	}
	var want string
	names := make([]string, 0, len(cases))
	for name := range cases {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		res := cases[name]
		assert.Equal(t, http.StatusUnauthorized, res.Status, name)
		require.NotNil(t, res.Error, name)
		assert.Equal(t, "INVALID_CREDENTIALS", res.Error.Code, name)
		assert.Empty(t, res.Header.Get("Set-Cookie"), name)
		assert.Empty(t, res.Header.Get("Retry-After"), name)
		body := string(res.Raw)
		if want == "" {
			want = body
		}
		assert.Equal(t, want, body, "%s must produce the same body as every other failure", name)
	}
}

func TestUnknownAccountsStillPayForAPasswordHash(t *testing.T) {
	pool := newPool(t)
	params := auth.DefaultParams()
	env := apptest.NewWithPoolAndParams(t, pool, params)
	d := apptest.DefaultSetup()
	require.Equal(t, http.StatusOK, env.Setup(d).Status)

	measure := func(email string) time.Duration {
		best := time.Hour
		for i := range 4 {
			start := time.Now()
			res := env.Do(apptest.Request{
				Method: http.MethodPost, Path: "/sessions", RemoteAddr: "198.51.100." + string(rune('1'+i)) + ":1",
				Headers: map[string]string{"Origin": apptest.PublicOrigin},
				Body:    map[string]string{"Email": email, "Password": "definitely not the password"},
			})
			elapsed := time.Since(start)
			require.Equal(t, http.StatusUnauthorized, res.Status)
			best = min(best, elapsed)
		}
		return best
	}
	known := measure(d.Email)
	unknown := measure("nobody@example.com")
	malformed := measure("not-an-email")
	t.Logf("known=%s unknown=%s malformed=%s", known, unknown, malformed)

	assert.Greater(t, unknown, known/2, "an unknown account must take about as long as a wrong password")
	assert.Greater(t, malformed, known/2, "so must a malformed email")
	assert.Greater(t, unknown, 5*time.Millisecond, "the unknown-account path really runs Argon2id with the OWASP parameters")
	assert.Less(t, unknown, known*3)
}

func TestBodyLimits(t *testing.T) {
	env := apptest.New(t)
	d := apptest.DefaultSetup()
	require.Equal(t, http.StatusOK, env.Setup(d).Status)
	client, res := env.Login(d.Email, d.Password)
	require.Equal(t, http.StatusOK, res.Status)

	big := `{"Email":"` + strings.Repeat("a", config.AuthBodyMaxBytes) + `","Password":"x"}`
	justUnder := `{"Email":"a@example.com","Password":"` + strings.Repeat("p", 4000) + `"}`
	tests := []struct {
		name string
		do   func() apptest.Response
		want int
		code string
	}{
		{"login body over the limit", func() apptest.Response {
			return env.Do(apptest.Request{Method: http.MethodPost, Path: "/sessions", Body: big, Headers: map[string]string{"Origin": apptest.PublicOrigin}})
		}, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE"},
		{"setup body over the limit", func() apptest.Response {
			return env.Do(apptest.Request{Method: http.MethodPost, Path: "/setup", Body: big, Headers: map[string]string{"Origin": apptest.PublicOrigin}})
		}, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE"},
		{"password change body over the limit", func() apptest.Response {
			return client.Do(apptest.Request{Method: http.MethodPut, Path: "/me/password", Body: big})
		}, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE"},
		{"a large but allowed body is just a failed login", func() apptest.Response {
			return env.Do(apptest.Request{Method: http.MethodPost, Path: "/sessions", Body: justUnder, Headers: map[string]string{"Origin": apptest.PublicOrigin}})
		}, http.StatusUnauthorized, "INVALID_CREDENTIALS"},
		{"invalid JSON", func() apptest.Response {
			return env.Do(apptest.Request{Method: http.MethodPost, Path: "/sessions", Body: `{"Email":`, Headers: map[string]string{"Origin": apptest.PublicOrigin}})
		}, http.StatusBadRequest, "BAD_REQUEST"},
		{"empty body", func() apptest.Response {
			return env.Do(apptest.Request{Method: http.MethodPost, Path: "/sessions", Headers: map[string]string{"Origin": apptest.PublicOrigin}})
		}, http.StatusBadRequest, "BAD_REQUEST"},
		{"wrong JSON type", func() apptest.Response {
			return env.Do(apptest.Request{Method: http.MethodPost, Path: "/sessions", Body: `{"Email":1,"Password":[]}`, Headers: map[string]string{"Origin": apptest.PublicOrigin}})
		}, http.StatusBadRequest, "BAD_REQUEST"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := tt.do()
			assert.Equal(t, tt.want, res.Status, string(res.Raw))
			require.NotNil(t, res.Error)
			assert.Equal(t, tt.code, res.Error.Code)
		})
	}
}

func TestMethodsAndPathsOutsideTheSpec(t *testing.T) {
	env := apptest.New(t)
	tests := []struct {
		name   string
		method string
		path   string
		want   int
	}{
		{"GET on login", http.MethodGet, "/sessions/revoke-others", http.StatusMethodNotAllowed},
		{"PUT on setup", http.MethodPut, "/setup", http.StatusMethodNotAllowed},
		{"unknown route", http.MethodGet, "/users", http.StatusNotFound},
		{"unknown nested route", http.MethodGet, "/organizations", http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := env.Do(apptest.Request{Method: tt.method, Path: tt.path})
			assert.Equal(t, tt.want, res.Status)
			assert.False(t, res.Success)
		})
	}
}
