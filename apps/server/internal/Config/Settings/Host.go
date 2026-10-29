package settingspec

import (
	"net/netip"
	"slices"
	"strings"
)

const (
	RuleSMTPHostBlocked = "smtpHostBlocked"
	RuleSMTPPlainRemote = "smtpPlainRemote"
	RulePasswordAgain   = "passwordAgain"
)

var metadataNames = []string{
	"metadata", "metadata.google.internal", "metadata.goog", "metadata.azure.com", "metadata.azure.internal",
	"instance-data", "instance-data.ec2.internal", "metadata.tencentyun.com", "metadata.platformequinix.com",
}

var metadataAddrs = []netip.Addr{
	netip.MustParseAddr("100.100.100.200"),
	netip.MustParseAddr("fd00:ec2::254"),
	netip.MustParseAddr("168.63.129.16"),
}

var blockedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("169.254.0.0/16"),
	netip.MustParsePrefix("fe80::/10"),
	netip.MustParsePrefix("0.0.0.0/8"),
}

var nat64 = netip.MustParsePrefix("64:ff9b::/96")

func normalHost(host string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
}

func BlockedHost(host string) bool {
	h := normalHost(strings.Trim(host, "[]"))
	if slices.Contains(metadataNames, h) {
		return true
	}
	addr, err := netip.ParseAddr(h)
	if err != nil {
		return false
	}
	return BlockedAddr(addr)
}

func BlockedAddr(addr netip.Addr) bool {
	addr = addr.WithZone("").Unmap()
	if addr.Is6() && nat64.Contains(addr) {
		raw := addr.As16()
		addr = netip.AddrFrom4([4]byte{raw[12], raw[13], raw[14], raw[15]})
	}
	if addr.IsUnspecified() || slices.Contains(metadataAddrs, addr) {
		return true
	}
	for _, p := range blockedPrefixes {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

func LoopbackHost(host string) bool {
	h := normalHost(host)
	if h == "localhost" || strings.HasSuffix(h, ".localhost") {
		return true
	}
	addr, err := netip.ParseAddr(strings.Trim(h, "[]"))
	return err == nil && addr.Unmap().IsLoopback()
}
