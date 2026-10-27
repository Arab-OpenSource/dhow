package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/Arab-OpenSource/dhow/apps/server/internal/api"
)

const (
	leakyDBError    = `dial tcp 10.0.0.5:5432: password authentication failed for user "dhow" password=hunter2`
	leakyQueryError = `ERROR: relation "Instance" does not exist (SQLSTATE 42P01)`
)

type panicDB struct{}

func (panicDB) Ping(context.Context) error { panic(leakyDBError) }

var leakMarkers = []string{"hunter2", "10.0.0.5", "5432", "SQLSTATE", "relation", "dial tcp", "goroutine", "panic"}

func TestEnvelopeShape(t *testing.T) {
	tests := []struct {
		name        string
		deps        api.Deps
		method      string
		path        string
		wantStatus  int
		wantSuccess bool
		wantData    bool
		wantCode    string
	}{
		{"liveness", api.Deps{}, http.MethodGet, "/api/v1/health/live", http.StatusOK, true, true, ""},
		{"readiness ok", api.Deps{}, http.MethodGet, "/api/v1/health/ready", http.StatusOK, true, true, ""},
		{
			"readiness unavailable",
			api.Deps{DB: fakeDB{err: errors.New(leakyDBError)}},
			http.MethodGet, "/api/v1/health/ready", http.StatusServiceUnavailable, false, true, "unavailable",
		},
		{"meta", api.Deps{}, http.MethodGet, "/api/v1/meta", http.StatusOK, true, true, ""},
		{
			"meta internal error",
			api.Deps{Queries: fakeQueries{err: errors.New(leakyQueryError)}},
			http.MethodGet, "/api/v1/meta", http.StatusInternalServerError, false, false, "internal",
		},
		{
			"recovered panic",
			api.Deps{DB: panicDB{}},
			http.MethodGet, "/api/v1/health/ready", http.StatusInternalServerError, false, false, "internal",
		},
		{"api not found", api.Deps{}, http.MethodGet, "/api/v1/nope", http.StatusNotFound, false, false, "not_found"},
		{"api version not found", api.Deps{}, http.MethodGet, "/api/v2/meta", http.StatusNotFound, false, false, "not_found"},
		{"web not found", api.Deps{}, http.MethodGet, "/dashboard", http.StatusNotFound, false, false, "not_found"},
		{"method not allowed", api.Deps{}, http.MethodDelete, "/api/v1/meta", http.StatusMethodNotAllowed, false, false, "method_not_allowed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, newRouter(t, tt.deps), tt.method, tt.path)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
				t.Fatalf("Content-Type = %q", ct)
			}
			var raw map[string]json.RawMessage
			if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
				t.Fatalf("body is not a JSON object: %q", rec.Body.String())
			}
			if len(raw) != 3 {
				t.Fatalf("envelope must have exactly Success, Data and Error: %s", rec.Body.String())
			}
			for _, k := range []string{"Success", "Data", "Error"} {
				if _, ok := raw[k]; !ok {
					t.Fatalf("envelope missing %s: %s", k, rec.Body.String())
				}
			}
			var success bool
			if err := json.Unmarshal(raw["Success"], &success); err != nil || success != tt.wantSuccess {
				t.Fatalf("Success = %s, want %v", raw["Success"], tt.wantSuccess)
			}
			if hasData := !bytes.Equal(raw["Data"], []byte("null")); hasData != tt.wantData {
				t.Fatalf("Data = %s, want present=%v", raw["Data"], tt.wantData)
			}
			assertErrorField(t, raw["Error"], tt.wantCode)
		})
	}
}

func assertErrorField(t *testing.T, raw json.RawMessage, wantCode string) {
	t.Helper()
	if wantCode == "" {
		if !bytes.Equal(raw, []byte("null")) {
			t.Fatalf("Error = %s, want null", raw)
		}
		return
	}
	var detail map[string]string
	if err := json.Unmarshal(raw, &detail); err != nil {
		t.Fatalf("Error is not an object of strings: %s", raw)
	}
	if len(detail) != 2 || detail["Code"] != wantCode || detail["Message"] == "" {
		t.Fatalf("Error = %s, want Code %q and a Message only", raw, wantCode)
	}
}

func TestErrorBodiesLeakNothing(t *testing.T) {
	tests := []struct {
		name string
		deps api.Deps
		path string
	}{
		{"readiness database error", api.Deps{DB: fakeDB{err: errors.New(leakyDBError)}}, "/api/v1/health/ready"},
		{"readiness migration error", api.Deps{Migrations: fakeMig{err: errors.New(leakyDBError)}}, "/api/v1/health/ready"},
		{"meta query error", api.Deps{Queries: fakeQueries{err: errors.New(leakyQueryError)}}, "/api/v1/meta"},
		{"recovered panic", api.Deps{DB: panicDB{}}, "/api/v1/health/ready"},
		{"not found", api.Deps{}, "/api/v1/nope"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, newRouter(t, tt.deps), http.MethodGet, tt.path)
			body := strings.ToLower(rec.Body.String())
			for _, m := range leakMarkers {
				if strings.Contains(body, strings.ToLower(m)) {
					t.Errorf("response leaks %q: %s", m, rec.Body.String())
				}
			}
		})
	}
}
