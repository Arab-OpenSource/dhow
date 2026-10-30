package netguard

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func addrs(list ...string) []netip.Addr {
	out := make([]netip.Addr, 0, len(list))
	for _, a := range list {
		out = append(out, netip.MustParseAddr(a))
	}
	return out
}

func TestDenial(t *testing.T) {
	strict := Policy{}
	open := Policy{Allowed: []netip.Prefix{netip.MustParsePrefix("10.20.0.0/16"), netip.MustParsePrefix("127.0.0.0/8"), netip.MustParsePrefix("169.254.0.0/16")}}
	tests := []struct {
		addr       string
		strictWhy  string
		allowedWhy string
	}{
		{"127.0.0.1", ReasonPrivate, ""},
		{"10.20.3.4", ReasonPrivate, ""},
		{"10.21.0.1", ReasonPrivate, ReasonPrivate},
		{"192.168.1.1", ReasonPrivate, ReasonPrivate},
		{"172.16.0.1", ReasonPrivate, ReasonPrivate},
		{"100.64.0.1", ReasonPrivate, ReasonPrivate},
		{"0.0.0.0", ReasonPrivate, ReasonPrivate},
		{"224.0.0.1", ReasonPrivate, ReasonPrivate},
		{"169.254.1.1", ReasonPrivate, ""},
		{"169.254.169.254", ReasonMetadata, ReasonMetadata},
		{"100.100.100.200", ReasonMetadata, ReasonMetadata},
		{"fd00:ec2::254", ReasonMetadata, ReasonMetadata},
		{"::1", ReasonPrivate, ReasonPrivate},
		{"::", ReasonPrivate, ReasonPrivate},
		{"::ffff:127.0.0.1", ReasonPrivate, ""},
		{"::ffff:169.254.169.254", ReasonMetadata, ReasonMetadata},
		{"::127.0.0.1", ReasonPrivate, ReasonPrivate},
		{"64:ff9b::7f00:1", ReasonPrivate, ReasonPrivate},
		{"2002:7f00:1::", ReasonPrivate, ReasonPrivate},
		{"fe80::1", ReasonPrivate, ReasonPrivate},
		{"fc00::1", ReasonPrivate, ReasonPrivate},
		{"203.0.113.10", "", ""},
		{"2606:4700::1111", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			a := netip.MustParseAddr(tt.addr)
			assert.Equal(t, tt.strictWhy, strict.Denial(a))
			assert.Equal(t, tt.allowedWhy, open.Denial(a))
			if tt.strictWhy != "" {
				require.ErrorIs(t, strict.Check(a), ErrBlocked)
			}
		})
	}
	assert.Equal(t, ReasonPrivate, strict.Denial(netip.Addr{}))
}

func TestParseCIDRs(t *testing.T) {
	got, err := ParseCIDRs("10.20.0.0/16, 192.168.5.0/24\nfd12:3456::/32")
	require.NoError(t, err)
	assert.Len(t, got, 3)
	empty, err := ParseCIDRs("  ")
	require.NoError(t, err)
	assert.Empty(t, empty)
	for _, bad := range []string{"10.20.0.1/16", "10.0.0.0", "0.0.0.0/0", "::/0", "::ffff:10.0.0.0/104", "nope", "10.0.0.0/7"} {
		_, err := ParseCIDRs(bad)
		require.ErrorIs(t, err, ErrCIDR, bad)
	}
	_, err = ParseCIDRs(strings.Repeat("10.0.0.0/8,", maxAllowed+1))
	require.ErrorIs(t, err, ErrCIDR)
}

func fixedLookup(m map[string][]netip.Addr) Lookup {
	return func(_ context.Context, host string) ([]netip.Addr, error) {
		if a, ok := m[host]; ok {
			return a, nil
		}
		return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
	}
}

func TestResolveRefusesAnyBlockedAnswer(t *testing.T) {
	d := Dialer{Lookup: fixedLookup(map[string][]netip.Addr{
		"public.example.com":  addrs("203.0.113.10"),
		"mixed.example.com":   addrs("203.0.113.10", "127.0.0.1"),
		"meta.example.com":    addrs("169.254.169.254"),
		"mapped.example.com":  addrs("::ffff:10.0.0.1"),
		"nothing.example.com": nil,
	})}
	got, err := d.Resolve(t.Context(), "public.example.com")
	require.NoError(t, err)
	assert.Equal(t, addrs("203.0.113.10"), got)
	for _, host := range []string{"mixed.example.com", "meta.example.com", "mapped.example.com", "127.0.0.1", "[::1]", "169.254.169.254"} {
		_, err := d.Resolve(t.Context(), host)
		require.ErrorIs(t, err, ErrBlocked, host)
	}
	_, err = d.Resolve(t.Context(), "nothing.example.com")
	require.ErrorIs(t, err, ErrNoAddress)
	_, err = d.Resolve(t.Context(), "missing.example.com")
	require.Error(t, err)
}

func TestDialConnectsToTheCheckedAddressSoRebindingDoesNotPass(t *testing.T) {
	var lookups atomic.Int32
	rebinding := func(_ context.Context, _ string) ([]netip.Addr, error) {
		if lookups.Add(1) == 1 {
			return addrs("203.0.113.10"), nil
		}
		return addrs("127.0.0.1"), nil
	}
	var dialed []string
	d := Dialer{Lookup: rebinding, dial: func(_ context.Context, _, address string) (net.Conn, error) {
		dialed = append(dialed, address)
		return nil, errors.New("refused in the test")
	}}
	_, err := d.DialContext(t.Context(), "tcp", "rebind.example.com:22")
	require.Error(t, err)
	assert.Equal(t, int32(1), lookups.Load())
	assert.Equal(t, []string{"203.0.113.10:22"}, dialed)

	_, err = d.DialContext(t.Context(), "tcp", "rebind.example.com:22")
	require.ErrorIs(t, err, ErrBlocked)
	assert.Len(t, dialed, 1)
}

func TestControlRefusesABlockedAddressAtConnectTime(t *testing.T) {
	d := Dialer{}
	require.ErrorIs(t, d.Control("tcp", "127.0.0.1:22", nil), ErrBlocked)
	require.ErrorIs(t, d.Control("tcp", "[fd00:ec2::254]:80", nil), ErrBlocked)
	require.ErrorIs(t, d.Control("tcp", "example.com:22", nil), ErrBlocked)
	require.NoError(t, d.Control("tcp", "203.0.113.10:22", nil))
}

func TestDialToLoopbackNeedsAnAllowedRange(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { require.NoError(t, ln.Close()) }()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()
	_, port, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)
	lookup := fixedLookup(map[string][]netip.Addr{"git.example.test": addrs("127.0.0.1")})

	_, err = Dialer{Lookup: lookup}.DialContext(t.Context(), "tcp", "git.example.test:"+port)
	require.ErrorIs(t, err, ErrBlocked)

	allowed := Dialer{Lookup: lookup, Policy: Policy{Allowed: []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")}}}
	conn, err := allowed.DialContext(t.Context(), "tcp", "git.example.test:"+port)
	require.NoError(t, err)
	require.NoError(t, conn.Close())
}

func tlsServer(t *testing.T, h http.HandlerFunc) (*httptest.Server, string, *tls.Config) {
	t.Helper()
	srv := httptest.NewTLSServer(h)
	t.Cleanup(srv.Close)
	_, port, err := net.SplitHostPort(strings.TrimPrefix(srv.URL, "https://"))
	require.NoError(t, err)
	transport, ok := srv.Client().Transport.(*http.Transport)
	require.True(t, ok)
	return srv, "https://example.com:" + port, transport.TLSClientConfig
}

func TestHTTPClientRefusesRedirectsBlockedHostsAndPlainHTTP(t *testing.T) {
	_, base, tlsConfig := tlsServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "http://169.254.169.254/latest/meta-data", http.StatusFound)
			return
		}
		_, _ = w.Write([]byte("ok"))
	})
	lookup := fixedLookup(map[string][]netip.Addr{"example.com": addrs("127.0.0.1")})
	open := Dialer{Lookup: lookup, Policy: Policy{Allowed: []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")}}}
	client := open.HTTPClient(ClientOptions{TLS: tlsConfig})

	get := func(c *http.Client, url string) (string, error) {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, http.NoBody)
		require.NoError(t, err)
		res, err := c.Do(req)
		if err != nil {
			return "", err
		}
		defer func() { _ = res.Body.Close() }()
		body, err := ReadLimited(res.Body, 0)
		return string(body), err
	}
	body, err := get(client, base+"/")
	require.NoError(t, err)
	assert.Equal(t, "ok", body)

	_, err = get(client, base+"/redirect")
	require.ErrorIs(t, err, ErrRedirect)

	_, err = get(Dialer{Lookup: lookup}.HTTPClient(ClientOptions{TLS: tlsConfig}), base+"/")
	require.ErrorIs(t, err, ErrBlocked)

	_, err = get(client, "http://example.com/")
	require.ErrorIs(t, err, ErrInsecure)
}

func TestReadLimited(t *testing.T) {
	got, err := ReadLimited(strings.NewReader("abc"), 3)
	require.NoError(t, err)
	assert.Equal(t, "abc", string(got))
	_, err = ReadLimited(strings.NewReader("abcd"), 3)
	require.ErrorIs(t, err, ErrTooLarge)
	_, err = ReadLimited(strings.NewReader(strings.Repeat("x", MaxResponseBytes+1)), 0)
	require.ErrorIs(t, err, ErrTooLarge)
}
