package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"slices"
	"strings"
	"testing"

	middleware "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Middleware"
)

func clientIPOf(trusted []netip.Prefix, remote string, forwarded []string) netip.Addr {
	var got netip.Addr
	h := middleware.ClientIP(trusted)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = middleware.ClientIPFrom(r.Context())
	}))
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = remote
	for _, v := range forwarded {
		r.Header.Add("X-Forwarded-For", v)
	}
	h.ServeHTTP(httptest.NewRecorder(), r)
	return got
}

func hops(forwarded string) []netip.Addr {
	var out []netip.Addr
	for _, part := range strings.Split(forwarded, ",") {
		if a, err := netip.ParseAddr(strings.TrimSpace(part)); err == nil {
			out = append(out, a.Unmap())
		}
	}
	return out
}

func FuzzClientIP(f *testing.F) {
	f.Add("203.0.113.7:4444", "198.51.100.1")
	f.Add("10.0.0.5:4444", "198.51.100.1, 10.0.0.9")
	f.Add("10.0.0.5:4444", "garbage, 192.0.2.1")
	f.Add("[2001:db8::1]:443", "::ffff:192.0.2.1")
	f.Add("10.0.0.5:4444", "10.1.1.1, 10.2.2.2")
	f.Add("not-an-address", "")
	f.Fuzz(func(t *testing.T, remote, forwarded string) {
		trusted := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
		peer := clientIPOf(nil, remote, nil)
		if got := clientIPOf(nil, remote, []string{forwarded}); got != peer {
			t.Fatalf("without a trusted proxy X-Forwarded-For %q moved the client from %v to %v", forwarded, peer, got)
		}
		got := clientIPOf(trusted, remote, []string{forwarded})
		if !peer.IsValid() || !trusted[0].Contains(peer) {
			if got != peer {
				t.Fatalf("an untrusted peer %v let X-Forwarded-For %q choose %v", peer, forwarded, got)
			}
			return
		}
		if got != peer && !slices.Contains(hops(forwarded), got) {
			t.Fatalf("the client %v is neither the peer nor a hop of %q", got, forwarded)
		}
		if got.Is4In6() {
			t.Fatalf("the client %v is not unmapped", got)
		}
	})
}
