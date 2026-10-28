package ratelimit_test

import (
	"net/netip"
	"testing"
	"time"

	ratelimit "github.com/Khaymah/dhow/apps/server/internal/Core/RateLimit"
)

func FuzzNetwork(f *testing.F) {
	for _, seed := range []string{"203.0.113.7", "::ffff:203.0.113.7", "2001:db8::1", "fe80::1%eth0", "::", "0.0.0.0", "", "1.2.3", "2001:db8::/64"} {
		f.Add(seed)
	}
	l := ratelimit.Limit{Name: "ip-oidc", Max: 1, Window: time.Minute}
	f.Fuzz(func(t *testing.T, ip string) {
		n := ratelimit.NetworkOf(ip)
		if _, err := ratelimit.Key(l, n); err != nil {
			t.Fatalf("NetworkOf(%q) = %q, which makes no key: %v", ip, n, err)
		}
		if n == ratelimit.UnknownNetwork {
			return
		}
		if a, err := netip.ParseAddr(n); err == nil {
			if !a.Is4() {
				t.Fatalf("NetworkOf(%q) = %q, a bare address that is not IPv4", ip, n)
			}
			return
		}
		p, err := netip.ParsePrefix(n)
		if err != nil || p.Bits() != ratelimit.IPv6Prefix || p.Masked() != p || !p.Addr().Is6() {
			t.Fatalf("NetworkOf(%q) = %q, not a masked IPv6 /64", ip, n)
		}
		a, err := netip.ParseAddr(ip)
		if err == nil && !p.Contains(a.WithZone("")) {
			t.Fatalf("%q is not inside its own network %q", ip, n)
		}
	})
}
