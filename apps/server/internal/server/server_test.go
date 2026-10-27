package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/Arab-OpenSource/dhow/apps/server/internal/api"
	"github.com/Arab-OpenSource/dhow/apps/server/internal/api/gen"
	"github.com/Arab-OpenSource/dhow/apps/server/internal/config"
	"github.com/Arab-OpenSource/dhow/apps/server/internal/db/sqlc"
	"github.com/Arab-OpenSource/dhow/apps/server/internal/web"
	"github.com/Arab-OpenSource/dhow/internal/buildinfo"
)

type fakeDB struct{ err error }

func (f fakeDB) Ping(context.Context) error { return f.err }

type fakeMig struct {
	current bool
	err     error
}

func (f fakeMig) Current(context.Context) (bool, error) { return f.current, f.err }

type fakeQueries struct {
	inst sqlc.Instance
	err  error
}

func (f fakeQueries) GetInstance(context.Context) (sqlc.Instance, error) { return f.inst, f.err }

func newRouter(t *testing.T, d api.Deps) http.Handler {
	t.Helper()
	if d.Log == nil {
		d.Log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if d.DB == nil {
		d.DB = fakeDB{}
	}
	if d.Migrations == nil {
		d.Migrations = fakeMig{current: true}
	}
	if d.Queries == nil {
		d.Queries = fakeQueries{}
	}
	u, err := url.Parse("https://dhow.example.com")
	if err != nil {
		t.Fatal(err)
	}
	return Router(Options{
		Config: config.Config{PublicURL: u, ListenAddr: config.DefaultListenAddr},
		API:    api.New(d),
		Web:    web.Handler(),
		Log:    d.Log,
	})
}

func do(t *testing.T, h http.Handler, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, http.NoBody))
	return rec
}

func TestLiveness(t *testing.T) {
	rec := do(t, newRouter(t, api.Deps{DB: fakeDB{err: errors.New("down")}}), http.MethodGet, "/api/v1/health/live")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; liveness must not depend on the database", rec.Code)
	}
}

func TestReadiness(t *testing.T) {
	tests := []struct {
		name       string
		deps       api.Deps
		wantStatus int
		wantChecks map[gen.HealthCheckName]bool
	}{
		{"ready", api.Deps{}, http.StatusOK, map[gen.HealthCheckName]bool{gen.Database: true, gen.Migrations: true}},
		{
			"database down",
			api.Deps{DB: fakeDB{err: errors.New("dial tcp 10.0.0.5:5432: password=hunter2")}},
			http.StatusServiceUnavailable,
			map[gen.HealthCheckName]bool{gen.Database: false, gen.Migrations: false},
		},
		{
			"pending migrations",
			api.Deps{Migrations: fakeMig{current: false}},
			http.StatusServiceUnavailable,
			map[gen.HealthCheckName]bool{gen.Database: true, gen.Migrations: false},
		},
		{
			"migration status error",
			api.Deps{Migrations: fakeMig{err: errors.New("boom")}},
			http.StatusServiceUnavailable,
			map[gen.HealthCheckName]bool{gen.Database: true, gen.Migrations: false},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, newRouter(t, tt.deps), http.MethodGet, "/api/v1/health/ready")
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if strings.Contains(rec.Body.String(), "hunter2") || strings.Contains(rec.Body.String(), "10.0.0.5") {
				t.Fatal("readiness leaks internal error details")
			}
			var body gen.HealthResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Data == nil {
				t.Fatal("Data is null")
			}
			if body.Success != (tt.wantStatus == http.StatusOK) {
				t.Errorf("Success = %v for status %d", body.Success, tt.wantStatus)
			}
			for _, c := range body.Data.Checks {
				if c.Ok != tt.wantChecks[c.Name] {
					t.Errorf("check %s ok = %v, want %v", c.Name, c.Ok, tt.wantChecks[c.Name])
				}
			}
		})
	}
}

func TestMeta(t *testing.T) {
	build := buildinfo.Info{Version: "v0.1.0", Commit: "abc123"}
	t.Run("setup required", func(t *testing.T) {
		rec := do(t, newRouter(t, api.Deps{Build: build}), http.MethodGet, "/api/v1/meta")
		m := decodeMeta(t, rec)
		if rec.Code != http.StatusOK || !m.SetupRequired || m.Version != "v0.1.0" || m.Commit != "abc123" {
			t.Fatalf("status=%d meta=%+v", rec.Code, m)
		}
	})
	t.Run("setup done", func(t *testing.T) {
		q := fakeQueries{inst: sqlc.Instance{SetupCompletedAt: pgtype.Timestamptz{Valid: true}}}
		rec := do(t, newRouter(t, api.Deps{Queries: q}), http.MethodGet, "/api/v1/meta")
		if decodeMeta(t, rec).SetupRequired {
			t.Fatal("SetupRequired = true after setup")
		}
	})
	t.Run("query error is generic", func(t *testing.T) {
		q := fakeQueries{err: errors.New(`relation "Instance" does not exist`)}
		rec := do(t, newRouter(t, api.Deps{Queries: q}), http.MethodGet, "/api/v1/meta")
		if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "relation") {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		assertCode(t, rec, "internal")
	})
}

func TestRoutingAndHeaders(t *testing.T) {
	h := newRouter(t, api.Deps{})
	tests := []struct {
		name, method, path string
		wantStatus         int
		wantCode           string
	}{
		{"unknown api route", http.MethodGet, "/api/v1/nope", http.StatusNotFound, "not_found"},
		{"unknown api version", http.MethodGet, "/api/v9/meta", http.StatusNotFound, "not_found"},
		{"web root until the UI is embedded", http.MethodGet, "/", http.StatusNotFound, "not_found"},
		{"wrong method", http.MethodPost, "/api/v1/meta", http.StatusMethodNotAllowed, "method_not_allowed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, h, tt.method, tt.path)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			assertCode(t, rec, tt.wantCode)
			if rec.Header().Get("Content-Security-Policy") == "" || rec.Header().Get("Strict-Transport-Security") == "" {
				t.Fatal("security headers missing")
			}
			if rec.Header().Get("X-Request-ID") == "" {
				t.Fatal("request id missing")
			}
		})
	}

	rec := do(t, h, http.MethodGet, "/api/v1/meta")
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("API Cache-Control = %q", rec.Header().Get("Cache-Control"))
	}
}

func TestBodyLimitApplies(t *testing.T) {
	h := newRouter(t, api.Deps{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/meta", strings.NewReader(strings.Repeat("a", MaxBodyBytes+1)))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d", rec.Code)
	}
	assertCode(t, rec, "payload_too_large")
}

func TestServerTimeouts(t *testing.T) {
	u, err := url.Parse("https://dhow.example.com")
	if err != nil {
		t.Fatal(err)
	}
	srv := New(Options{
		Config: config.Config{PublicURL: u, ListenAddr: "127.0.0.1:0"},
		API:    api.New(api.Deps{}),
		Web:    web.Handler(),
		Log:    slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if srv.ReadHeaderTimeout == 0 || srv.ReadTimeout == 0 || srv.WriteTimeout == 0 || srv.IdleTimeout == 0 || srv.MaxHeaderBytes == 0 {
		t.Fatalf("server timeouts not set: %+v", srv)
	}
}

func decodeMeta(t *testing.T, rec *httptest.ResponseRecorder) gen.Meta {
	t.Helper()
	var body gen.MetaResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.Success || body.Data == nil || body.Error != nil {
		t.Fatalf("not a success envelope: %s", rec.Body.String())
	}
	return *body.Data
}

func assertCode(t *testing.T, rec *httptest.ResponseRecorder, code string) {
	t.Helper()
	var body gen.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("not an error body: %q", rec.Body.String())
	}
	if body.Success || body.Data != nil || body.Error == nil || body.Error.Code != code {
		t.Fatalf("body = %s, want error code %q", rec.Body.String(), code)
	}
}
