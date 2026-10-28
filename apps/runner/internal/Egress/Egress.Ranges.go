package egress

import (
	"net"
	"net/netip"
)

func blockedPrefixes() []netip.Prefix {
	out := make([]netip.Prefix, 0, 24)
	for _, p := range []string{
		"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16", "172.16.0.0/12",
		"192.0.0.0/24", "192.168.0.0/16", "198.18.0.0/15", "224.0.0.0/4", "240.0.0.0/4",
		"::1/128", "::/96", "fc00::/7", "fe80::/10", "fec0::/10", "ff00::/8", "64:ff9b::/96", "64:ff9b:1::/48",
		"100::/64", "2001::/32", "2001:db8::/32", "2002::/16",
	} {
		out = append(out, netip.MustParsePrefix(p))
	}
	return out
}

func metadataAddrs() []netip.Addr {
	out := make([]netip.Addr, 0, 10)
	for _, a := range []string{
		"169.254.169.254", "169.254.170.2", "169.254.170.23", "fd00:ec2::254", "fd00:ec2::23",
		"168.63.129.16", "100.100.100.200", "192.0.0.192", "fd20:ce::254", "fd00:c1::a9fe:a9fe",
	} {
		out = append(out, netip.MustParseAddr(a))
	}
	return out
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
