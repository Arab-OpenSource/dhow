package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"
)

func discard() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestSecurityHeaders(t *testing.T) {
	tests := []struct {
		name     string
		hsts     bool
		wantHSTS bool
	}{
		{"https", true, true},
		{"http dev", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := SecurityHeaders(tt.hsts)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", http.NoBody))
			want := map[string]string{
				"Content-Security-Policy":      ContentSecurityPolicy,
				"X-Content-Type-Options":       "nosniff",
				"Referrer-Policy":              "no-referrer",
				"Cross-Origin-Opener-Policy":   "same-origin",
				"Cross-Origin-Resource-Policy": "same-origin",
				"X-Frame-Options":              "DENY",
			}
			for k, v := range want {
				if got := rec.Header().Get(k); got != v {
					t.Errorf("%s = %q, want %q", k, got, v)
				}
			}
			for _, d := range []string{"frame-ancestors 'none'", "base-uri 'none'", "object-src 'none'", "default-src 'self'"} {
				if !strings.Contains(rec.Header().Get("Content-Security-Policy"), d) {
					t.Errorf("CSP missing %s", d)
				}
			}
			if got := rec.Header().Get("Strict-Transport-Security") != ""; got != tt.wantHSTS {
				t.Errorf("HSTS present = %v, want %v", got, tt.wantHSTS)
			}
			if rec.Header().Get("Access-Control-Allow-Origin") != "" {
				t.Error("CORS header must never be set")
			}
		})
	}
}

func TestClientIP(t *testing.T) {
	trusted := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	tests := []struct {
		name   string
		remote string
		xff    []string
		want   string
	}{
		{"direct client ignores spoofed header", "203.0.113.9:4000", []string{"1.2.3.4"}, "203.0.113.9"},
		{"trusted proxy forwards client", "10.0.0.2:4000", []string{"198.51.100.7"}, "198.51.100.7"},
		{"spoof left of real client is ignored", "10.0.0.2:4000", []string{"1.2.3.4, 198.51.100.7"}, "198.51.100.7"},
		{"chain of trusted proxies", "10.0.0.2:4000", []string{"198.51.100.7, 10.1.1.1"}, "198.51.100.7"},
		{"multiple header lines", "10.0.0.2:4000", []string{"1.2.3.4", "198.51.100.7"}, "198.51.100.7"},
		{"garbage hop stops the chain", "10.0.0.2:4000", []string{"198.51.100.7, not-an-ip"}, "10.0.0.2"},
		{"trusted proxy without header", "10.0.0.2:4000", nil, "10.0.0.2"},
		{"ipv6 peer", "[2001:db8::1]:443", []string{"1.2.3.4"}, "2001:db8::1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got netip.Addr
			h := ClientIP(trusted)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				got = ClientIPFrom(r.Context())
			}))
			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			req.RemoteAddr = tt.remote
			for _, v := range tt.xff {
				req.Header.Add("X-Forwarded-For", v)
			}
			h.ServeHTTP(httptest.NewRecorder(), req)
			if got.String() != tt.want {
				t.Fatalf("client IP = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestClientIPNoTrustedProxies(t *testing.T) {
	var got netip.Addr
	h := ClientIP(nil)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { got = ClientIPFrom(r.Context()) }))
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.RemoteAddr = "10.0.0.2:1"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if got.String() != "10.0.0.2" {
		t.Fatalf("got %s", got)
	}
}

func TestBodyLimit(t *testing.T) {
	var readErr error
	h := BodyLimit(8)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, readErr = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))

	t.Run("declared too large", func(t *testing.T) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", strings.NewReader("0123456789")))
		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d", rec.Code)
		}
		assertErrorBody(t, rec, "payload_too_large")
	})
	t.Run("undeclared too large", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", io.NopCloser(strings.NewReader("0123456789")))
		req.ContentLength = -1
		h.ServeHTTP(httptest.NewRecorder(), req)
		var mbe *http.MaxBytesError
		if !errors.As(readErr, &mbe) {
			t.Fatalf("read error = %v, want *http.MaxBytesError", readErr)
		}
	})
	t.Run("within limit", func(t *testing.T) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", strings.NewReader("small")))
		if rec.Code != http.StatusNoContent || readErr != nil {
			t.Fatalf("status = %d err = %v", rec.Code, readErr)
		}
	})
}

func TestRecover(t *testing.T) {
	var logs bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&logs, nil))
	h := RequestID(Recover(log)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("db password is hunter2")
	})))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", http.NoBody))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
	assertErrorBody(t, rec, "internal")
	if strings.Contains(rec.Body.String(), "hunter2") {
		t.Fatal("panic value leaked to the client")
	}
	if !strings.Contains(logs.String(), rec.Header().Get("X-Request-ID")) {
		t.Fatal("panic log lacks the request ID")
	}
}

func TestRecoverReraisesAbort(t *testing.T) {
	h := Recover(discard())(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic(http.ErrAbortHandler) }))
	defer func() {
		if r := recover(); r != http.ErrAbortHandler { //nolint:errorlint // comparing the recovered sentinel value
			t.Fatalf("recovered %v, want ErrAbortHandler", r)
		}
	}()
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", http.NoBody))
}

func TestRequestIDIgnoresClientValue(t *testing.T) {
	var seen string
	h := RequestID(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { seen = RequestIDFrom(r.Context()) }))
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Header.Set("X-Request-ID", "attacker-chosen\nvalue")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if seen == "" || strings.Contains(seen, "attacker") || rec.Header().Get("X-Request-ID") != seen || len(seen) != 24 {
		t.Fatalf("request id = %q", seen)
	}
}

func TestTimeoutSetsDeadline(t *testing.T) {
	var ok bool
	h := Timeout(time.Second)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, ok = r.Context().Deadline()
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", http.NoBody))
	if !ok {
		t.Fatal("no deadline on request context")
	}
}

func TestAccessLogOmitsQuery(t *testing.T) {
	var logs bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&logs, nil))
	h := AccessLog(log)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) }))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/meta?token=abc123", http.NoBody))
	if strings.Contains(logs.String(), "abc123") {
		t.Fatal("query string logged")
	}
	if !strings.Contains(logs.String(), `"status":418`) {
		t.Fatalf("status not logged: %s", logs.String())
	}
}

func assertErrorBody(t *testing.T, rec *httptest.ResponseRecorder, code string) {
	t.Helper()
	var body ErrorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not the error schema: %v", err)
	}
	if body.Code != code || body.Message == "" {
		t.Fatalf("body = %+v, want code %q", body, code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q", ct)
	}
}
