package auth_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
	auth "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Auth"
	policy "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Policy"
)

const origin = "https://dhow.example.com"

type fakeResolver struct {
	sessions map[string]auth.Resolved
	err      error
}

func (f fakeResolver) Resolve(_ context.Context, hash []byte, _ time.Time) (auth.Resolved, error) {
	if f.err != nil {
		return auth.Resolved{}, f.err
	}
	if r, ok := f.sessions[string(hash)]; ok {
		return r, nil
	}
	return auth.Resolved{}, auth.ErrNoSession
}

type guardFixture struct {
	handler http.Handler
	cookies map[string]string
	csrf    map[string]string
	orgA    uuid.UUID
	orgB    uuid.UUID
	logs    *bytes.Buffer
	reached *int
}

func newGuardFixture(t *testing.T, resolverErr error) guardFixture {
	t.Helper()
	csrf := auth.NewCSRF([]byte("0123456789abcdef0123456789abcdef"))
	orgA, orgB := uuid.New(), uuid.New()
	f := guardFixture{orgA: orgA, orgB: orgB, cookies: map[string]string{}, csrf: map[string]string{}, logs: &bytes.Buffer{}}
	sessions := map[string]auth.Resolved{}
	for name, memberships := range map[string][]policy.Membership{
		"owner":    {{OrganizationID: orgA, Role: policy.RoleOwner}},
		"viewer":   {{OrganizationID: orgA, Role: policy.RoleViewer}},
		"outsider": {{OrganizationID: orgB, Role: policy.RoleOwner}},
		"instance": nil,
	} {
		cookie, hash, err := auth.NewSessionToken(config.SessionTokenBytes)
		require.NoError(t, err)
		token := csrf.Token(cookie)
		sessions[string(hash)] = auth.Resolved{
			SessionID: uuid.New(), UserID: uuid.New(), CsrfHash: csrf.Hash(token), Email: name + "@example.com", Memberships: memberships,
			InstanceOwner: name == "instance",
		}
		f.cookies[name], f.csrf[name] = cookie, token
	}
	u, err := url.Parse(origin)
	require.NoError(t, err)
	guard := auth.NewGuard(auth.GuardOptions{
		Resolver: fakeResolver{sessions: sessions, err: resolverErr}, CSRF: csrf, PublicURL: u,
		Log: slog.New(slog.NewJSONHandler(f.logs, nil)),
	})
	table := policy.NewTable(
		policy.Public(http.MethodPost, "/sessions"),
		policy.Public(http.MethodGet, "/meta"),
		policy.Authenticated(http.MethodGet, "/me", policy.SessionManageSelf),
		policy.Authenticated(http.MethodDelete, "/sessions/current", policy.SessionManageSelf),
		policy.Organization(http.MethodGet, "/organizations/{id}", policy.OrgRead, "id"),
		policy.Organization(http.MethodPatch, "/organizations/{id}", policy.OrgUpdate, "id"),
		policy.Instance(http.MethodPost, "/organizations", policy.OrgCreate),
	)
	reached := 0
	f.reached = &reached
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached++
		if p, found := auth.PrincipalFrom(r.Context()); found {
			w.Header().Set("X-User", p.Email)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	r := chi.NewRouter()
	r.Route(config.APIPrefix, func(api chi.Router) {
		api = api.With(guard.Middleware(table))
		api.Post("/sessions", ok)
		api.Get("/meta", ok)
		api.Get("/me", ok)
		api.Delete("/sessions/current", ok)
		api.Get("/organizations/{id}", ok)
		api.Patch("/organizations/{id}", ok)
		api.Post("/organizations", ok)
		api.Get("/secret", ok)
	})
	f.handler = r
	return f
}

type call struct {
	method, path string
	cookie       string
	origin       string
	csrf         string
}

func (f guardFixture) do(c call) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(context.Background(), c.method, config.APIPrefix+c.path, http.NoBody)
	if c.cookie != "" {
		req.Header.Set("Cookie", config.SessionCookieName+"="+c.cookie)
	}
	if c.origin != "" {
		req.Header.Set("Origin", c.origin)
	}
	if c.csrf != "" {
		req.Header.Set(config.CSRFHeaderName, c.csrf)
	}
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	return rec
}

func TestGuardDecisions(t *testing.T) {
	f := newGuardFixture(t, nil)
	org := f.orgA.String()
	tests := []struct {
		name     string
		call     call
		want     int
		wantCode string
	}{
		{"public GET needs nothing", call{method: "GET", path: "/meta"}, 204, ""},
		{"public POST with the right origin", call{method: "POST", path: "/sessions", origin: origin}, 204, ""},
		{"public POST with another origin", call{method: "POST", path: "/sessions", origin: "https://evil.example"}, 403, "CSRF_FAILED"},
		{"public POST without an origin", call{method: "POST", path: "/sessions"}, 403, "CSRF_FAILED"},
		{"public POST with origin null", call{method: "POST", path: "/sessions", origin: "null"}, 403, "CSRF_FAILED"},
		{"public POST with a look-alike origin", call{method: "POST", path: "/sessions", origin: origin + ".evil.example"}, 403, "CSRF_FAILED"},
		{"public POST with http instead of https", call{method: "POST", path: "/sessions", origin: "http://dhow.example.com"}, 403, "CSRF_FAILED"},
		{"public POST needs no CSRF token even with a stale cookie", call{method: "POST", path: "/sessions", origin: origin, cookie: "stale"}, 204, ""},

		{"anonymous GET /me", call{method: "GET", path: "/me"}, 401, "UNAUTHENTICATED"},
		{"garbage cookie", call{method: "GET", path: "/me", cookie: "not-a-token"}, 401, "UNAUTHENTICATED"},
		{"unknown session", call{method: "GET", path: "/me", cookie: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}, 401, "UNAUTHENTICATED"},
		{"valid session GET /me needs no CSRF token", call{method: "GET", path: "/me", cookie: f.cookies["viewer"]}, 204, ""},

		{"DELETE without a CSRF token", call{method: "DELETE", path: "/sessions/current", cookie: f.cookies["owner"], origin: origin}, 403, "CSRF_FAILED"},
		{"DELETE with a wrong CSRF token", call{method: "DELETE", path: "/sessions/current", cookie: f.cookies["owner"], origin: origin, csrf: "wrong"}, 403, "CSRF_FAILED"},
		{"DELETE with another session's CSRF token", call{method: "DELETE", path: "/sessions/current", cookie: f.cookies["owner"], origin: origin, csrf: f.csrf["viewer"]}, 403, "CSRF_FAILED"},
		{"DELETE with the right token but a wrong origin", call{method: "DELETE", path: "/sessions/current", cookie: f.cookies["owner"], origin: "https://evil.example", csrf: f.csrf["owner"]}, 403, "CSRF_FAILED"},
		{"DELETE with the right token and no origin", call{method: "DELETE", path: "/sessions/current", cookie: f.cookies["owner"], csrf: f.csrf["owner"]}, 403, "CSRF_FAILED"},
		{"DELETE with everything right", call{method: "DELETE", path: "/sessions/current", cookie: f.cookies["owner"], origin: origin, csrf: f.csrf["owner"]}, 204, ""},
		{"DELETE anonymous", call{method: "DELETE", path: "/sessions/current", origin: origin}, 401, "UNAUTHENTICATED"},

		{"owner reads the organization", call{method: "GET", path: "/organizations/" + org, cookie: f.cookies["owner"]}, 204, ""},
		{"viewer reads the organization", call{method: "GET", path: "/organizations/" + org, cookie: f.cookies["viewer"]}, 204, ""},
		{"viewer may not update it", call{method: "PATCH", path: "/organizations/" + org, cookie: f.cookies["viewer"], origin: origin, csrf: f.csrf["viewer"]}, 403, "FORBIDDEN"},
		{"owner updates it", call{method: "PATCH", path: "/organizations/" + org, cookie: f.cookies["owner"], origin: origin, csrf: f.csrf["owner"]}, 204, ""},
		{"a member of another organization sees 404", call{method: "GET", path: "/organizations/" + org, cookie: f.cookies["outsider"]}, 404, "NOT_FOUND"},
		{"and also 404 for an update", call{method: "PATCH", path: "/organizations/" + org, cookie: f.cookies["outsider"], origin: origin, csrf: f.csrf["outsider"]}, 404, "NOT_FOUND"},
		{"an organization that does not exist is 404", call{method: "GET", path: "/organizations/" + uuid.NewString(), cookie: f.cookies["owner"]}, 404, "NOT_FOUND"},
		{"a malformed id is 404", call{method: "GET", path: "/organizations/not-a-uuid", cookie: f.cookies["owner"]}, 404, "NOT_FOUND"},
		{"anonymous organization read", call{method: "GET", path: "/organizations/" + org}, 401, "UNAUTHENTICATED"},
		{"CSRF is checked before authorization", call{method: "PATCH", path: "/organizations/" + org, cookie: f.cookies["outsider"], origin: origin}, 403, "CSRF_FAILED"},

		{"the instance owner creates an organization", call{method: "POST", path: "/organizations", cookie: f.cookies["instance"], origin: origin, csrf: f.csrf["instance"]}, 204, ""},
		{"an organization owner may not by default", call{method: "POST", path: "/organizations", cookie: f.cookies["owner"], origin: origin, csrf: f.csrf["owner"]}, 403, "FORBIDDEN"},
		{"instance routes still need the CSRF token", call{method: "POST", path: "/organizations", cookie: f.cookies["instance"], origin: origin}, 403, "CSRF_FAILED"},
		{"anonymous instance route", call{method: "POST", path: "/organizations", origin: origin}, 401, "UNAUTHENTICATED"},

		{"a route without a declared policy is refused", call{method: "GET", path: "/secret", cookie: f.cookies["owner"]}, 403, "FORBIDDEN"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := *f.reached
			rec := f.do(tt.call)
			assert.Equal(t, tt.want, rec.Code, rec.Body.String())
			if tt.wantCode != "" {
				assert.Contains(t, rec.Body.String(), `"Code":"`+tt.wantCode+`"`)
				assert.Equal(t, before, *f.reached, "a refused request must never reach the handler")
			} else {
				assert.Equal(t, before+1, *f.reached)
			}
		})
	}
	assert.Contains(t, f.logs.String(), "route has no valid policy declaration")
}

func TestGuardExposesThePrincipal(t *testing.T) {
	f := newGuardFixture(t, nil)
	rec := f.do(call{method: "GET", path: "/me", cookie: f.cookies["viewer"]})
	assert.Equal(t, "viewer@example.com", rec.Header().Get("X-User"))
}

func TestGuardFailsClosedWhenTheStoreFails(t *testing.T) {
	f := newGuardFixture(t, errors.New("database is down"))
	rec := f.do(call{method: "GET", path: "/me", cookie: f.cookies["owner"]})
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.NotContains(t, rec.Body.String(), "database is down")
	assert.Zero(t, *f.reached)
}

func TestOriginOf(t *testing.T) {
	tests := map[string]string{
		"https://dhow.example.com":      "https://dhow.example.com",
		"https://DHOW.example.com":      "https://dhow.example.com",
		"https://dhow.example.com:443":  "https://dhow.example.com",
		"https://dhow.example.com:8443": "https://dhow.example.com:8443",
		"http://localhost:8080":         "http://localhost:8080",
		"http://localhost:80":           "http://localhost",
	}
	for in, want := range tests {
		u, err := url.Parse(in)
		require.NoError(t, err)
		assert.Equal(t, want, auth.OriginOf(u), in)
	}
}
