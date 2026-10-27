package app_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	app "github.com/Arab-OpenSource/dhow/apps/server/internal/App"
	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
)

func listenAddr(srv *httptest.Server) string { return srv.Listener.Addr().String() }

func readyServer(t *testing.T, status int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/health/ready" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestReadyURL(t *testing.T) {
	tests := []struct {
		name string
		addr string
		want string
	}{
		{"loopback", "127.0.0.1:8080", "http://127.0.0.1:8080/api/v1/health/ready"},
		{"all ipv4 interfaces", "0.0.0.0:8080", "http://127.0.0.1:8080/api/v1/health/ready"},
		{"empty host", ":9000", "http://127.0.0.1:9000/api/v1/health/ready"},
		{"all ipv6 interfaces", "[::]:8080", "http://127.0.0.1:8080/api/v1/health/ready"},
		{"ipv6 loopback", "[::1]:8080", "http://[::1]:8080/api/v1/health/ready"},
		{"hostname", "localhost:8081", "http://localhost:8081/api/v1/health/ready"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := app.ReadyURL(tt.addr)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestReadyURLRejectsMissingPort(t *testing.T) {
	_, err := app.ReadyURL("localhost")
	require.Error(t, err)
}

func TestHealthcheckReady(t *testing.T) {
	srv := readyServer(t, http.StatusOK)
	require.NoError(t, app.Healthcheck(context.Background(), listenAddr(srv), time.Second))
}

func TestHealthcheckFailures(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{"service unavailable", http.StatusServiceUnavailable},
		{"server error", http.StatusInternalServerError},
		{"not found", http.StatusNotFound},
		{"no content", http.StatusNoContent},
		{"redirect is not followed", http.StatusFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := readyServer(t, tt.status)
			err := app.Healthcheck(context.Background(), listenAddr(srv), time.Second)
			require.ErrorIs(t, err, app.ErrNotReady)
		})
	}
}

func TestHealthcheckTimesOut(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-release }))
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(release) })

	start := time.Now()
	err := app.Healthcheck(context.Background(), listenAddr(srv), 100*time.Millisecond)
	require.Error(t, err)
	assert.Less(t, time.Since(start), 2*time.Second)
}

func TestHealthcheckConnectionRefused(t *testing.T) {
	srv := readyServer(t, http.StatusOK)
	addr := listenAddr(srv)
	srv.Close()
	require.Error(t, app.Healthcheck(context.Background(), addr, time.Second))
}

func TestRunHealthcheckReadsOnlyTheListenAddress(t *testing.T) {
	srv := readyServer(t, http.StatusOK)
	t.Setenv(config.EnvDatabaseURL, "")
	t.Setenv(config.EnvSecretKey, "")
	t.Setenv(config.EnvPublicURL, "")
	t.Setenv(config.EnvListenAddr, listenAddr(srv))
	require.NoError(t, app.RunHealthcheck(context.Background()))
}

func TestRunHealthcheckRejectsBadListenAddress(t *testing.T) {
	t.Setenv(config.EnvListenAddr, "8080")
	err := app.RunHealthcheck(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), config.EnvListenAddr)
}
