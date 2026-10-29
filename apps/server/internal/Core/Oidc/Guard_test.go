package oidcauth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	oidcauth "github.com/Khaymah/dhow/apps/server/internal/Core/Oidc"
)

func TestTheProxyNeverCarriesARequestToABlockedAddress(t *testing.T) {
	var reached atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(proxy.Close)
	proxyURL, err := url.Parse(proxy.URL)
	require.NoError(t, err)
	viaProxy := func(*http.Request) (*url.URL, error) { return proxyURL, nil }
	lookup := func(_ context.Context, host string) ([]netip.Addr, error) {
		switch host {
		case "metadata.attacker.test":
			return []netip.Addr{netip.MustParseAddr("169.254.169.254")}, nil
		case "nat64.attacker.test":
			return []netip.Addr{netip.MustParseAddr("64:ff9b::a9fe:a9fe")}, nil
		default:
			return []netip.Addr{netip.MustParseAddr("203.0.113.10")}, nil
		}
	}
	client := &http.Client{Transport: oidcauth.GuardedTransportWith(viaProxy, lookup), Timeout: 5 * time.Second}
	tests := []struct {
		url     string
		blocked bool
	}{
		{"http://169.254.169.254/latest/meta-data/", true},
		{"http://[fe80::1]/", true},
		{"http://168.63.129.16/machine", true},
		{"http://metadata.attacker.test/", true},
		{"http://nat64.attacker.test/", true},
		{"http://idp.example.test/.well-known/openid-configuration", false},
	}
	for _, tt := range tests {
		before := reached.Load()
		res, err := client.Get(tt.url)
		if res != nil {
			_ = res.Body.Close()
		}
		if tt.blocked {
			require.Error(t, err, tt.url)
			assert.Contains(t, err.Error(), "link-local or a cloud metadata", tt.url)
			assert.Equal(t, before, reached.Load(), "the proxy never sees %s", tt.url)
			continue
		}
		require.NoError(t, err, tt.url)
		assert.Equal(t, before+1, reached.Load(), "an allowed host still goes through the proxy")
	}
}

func TestAFailedProbeExplainsWithoutTheResponseBody(t *testing.T) {
	const marker = "INTERNAL-BODY-MARKER-91f3"
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(marker))
	}))
	t.Cleanup(srv.Close)
	r, err := oidcauth.NewRegistry(nil, oidcauth.Options{Transport: srv.Client().Transport})
	require.NoError(t, err)
	for _, kind := range []string{config.OIDCKindOIDC, config.OIDCKindGitHub} {
		res := r.Probe(context.Background(), kind, srv.URL)
		require.False(t, res.Passed())
		require.Error(t, res.Err)
		assert.NotEmpty(t, res.Reason())
		assert.NotContains(t, res.Reason(), marker)
		assert.False(t, strings.Contains(res.Reason(), srv.URL), "the reason names no address")
	}
}
