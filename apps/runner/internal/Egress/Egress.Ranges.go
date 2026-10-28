package egress

import (
	"net"
	"net/netip"
)

func blockedPrefixes() []netip.Prefix {
	out := make([]netip.Prefix, 0, 18)
	for _, p := range []string{
		"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16", "172.16.0.0/12",
		"192.0.0.0/24", "192.168.0.0/16", "198.18.0.0/15", "224.0.0.0/4", "240.0.0.0/4",
		"::1/128", "::/128", "fc00::/7", "fe80::/10", "ff00::/8", "64:ff9b::/96", "2001:db8::/32",
	} {
		out = append(out, netip.MustParsePrefix(p))
	}
	return out
}

func metadataAddrs() []netip.Addr {
	return []netip.Addr{
		netip.MustParseAddr("169.254.169.254"), netip.MustParseAddr("169.254.170.2"), netip.MustParseAddr("fd00:ec2::254"),
	}
}

func HostAddrs() ([]netip.Addr, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}
	out := make([]netip.Addr, 0, len(addrs))
	for _, a := range addrs {
		if p, err := netip.ParsePrefix(a.String()); err == nil {
			out = append(out, p.Addr().Unmap())
		}
	}
	return out, nil
}

type ranges struct {
	blocked []netip.Prefix
	host    []netip.Addr
	allowed []netip.Prefix
}

func (r ranges) denial(addr netip.Addr) string {
	addr = addr.Unmap().WithZone("")
	for _, m := range metadataAddrs() {
		if addr == m {
			return "a cloud metadata address"
		}
	}
	for _, p := range r.allowed {
		if p.Contains(addr) {
			return ""
		}
	}
	for _, p := range r.blocked {
		if p.Contains(addr) {
			return "a private, loopback or reserved address"
		}
	}
	for _, h := range r.host {
		if addr == h {
			return "an address of the runner host"
		}
	}
	return ""
}
