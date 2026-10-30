package egress

import (
	"net"
	"net/netip"

	"github.com/Khaymah/dhow/internal/netguard"
)

const reasonRunnerHost = "an address of the runner host"

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
	guard netguard.Policy
	host  []netip.Addr
}

func (r ranges) denial(addr netip.Addr) string {
	if why := r.guard.Denial(addr); why != "" {
		return why
	}
	if r.guard.Covers(addr) {
		return ""
	}
	addr = addr.Unmap().WithZone("")
	for _, h := range r.host {
		if addr == h {
			return reasonRunnerHost
		}
	}
	return ""
}
