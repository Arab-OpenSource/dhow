package netguard

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"
)

const (
	ReasonMetadata = "a cloud metadata address"
	ReasonPrivate  = "a private, loopback or reserved address"
	maxAllowed     = 64
	minBitsV4      = 8
	minBitsV6      = 16
)

var (
	ErrBlocked   = errors.New("the address is not reachable from here")
	ErrNoAddress = errors.New("the host has no address")
	ErrCIDR      = errors.New("an allowed range must be a CIDR such as 10.20.0.0/16")
)

func BlockedPrefixes() []netip.Prefix {
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

func MetadataAddrs() []netip.Addr {
	out := make([]netip.Addr, 0, 10)
	for _, a := range []string{
		"169.254.169.254", "169.254.170.2", "169.254.170.23", "fd00:ec2::254", "fd00:ec2::23",
		"168.63.129.16", "100.100.100.200", "192.0.0.192", "fd20:ce::254", "fd00:c1::a9fe:a9fe",
	} {
		out = append(out, netip.MustParseAddr(a))
	}
	return out
}

type Policy struct {
	Allowed []netip.Prefix
}

func (p Policy) Denial(addr netip.Addr) string {
	if !addr.IsValid() {
		return ReasonPrivate
	}
	addr = addr.Unmap().WithZone("")
	for _, m := range MetadataAddrs() {
		if addr == m {
			return ReasonMetadata
		}
	}
	if p.Covers(addr) {
		return ""
	}
	if addr.IsUnspecified() || addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() || addr.IsMulticast() {
		return ReasonPrivate
	}
	for _, blocked := range BlockedPrefixes() {
		if blocked.Contains(addr) {
			return ReasonPrivate
		}
	}
	return ""
}

func (p Policy) Covers(addr netip.Addr) bool {
	addr = addr.Unmap().WithZone("")
	for _, allowed := range p.Allowed {
		if allowed.Contains(addr) {
			return true
		}
	}
	return false
}

func (p Policy) Check(addr netip.Addr) error {
	if why := p.Denial(addr); why != "" {
		return fmt.Errorf("%w: %s", ErrBlocked, why)
	}
	return nil
}

func ParseCIDRs(list string) ([]netip.Prefix, error) {
	fields := strings.FieldsFunc(list, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' || r == '\n' })
	if len(fields) > maxAllowed {
		return nil, fmt.Errorf("%w: at most %d ranges", ErrCIDR, maxAllowed)
	}
	out := make([]netip.Prefix, 0, len(fields))
	for _, f := range fields {
		p, err := netip.ParsePrefix(f)
		if err != nil || p != p.Masked() {
			return nil, fmt.Errorf("%w, not %q", ErrCIDR, f)
		}
		if p.Addr().Is4In6() {
			return nil, fmt.Errorf("%w, not an IPv4-mapped range %q", ErrCIDR, f)
		}
		if (p.Addr().Is4() && p.Bits() < minBitsV4) || (p.Addr().Is6() && p.Bits() < minBitsV6) {
			return nil, fmt.Errorf("%w no wider than /%d for IPv4 or /%d for IPv6, not %q", ErrCIDR, minBitsV4, minBitsV6, f)
		}
		out = append(out, p)
	}
	return out, nil
}
