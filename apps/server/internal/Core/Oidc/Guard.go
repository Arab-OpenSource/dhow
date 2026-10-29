package oidcauth

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"syscall"
	"time"

	settingspec "github.com/Khaymah/dhow/apps/server/internal/Config/Settings"
)

const (
	dialTimeout   = 10 * time.Second
	dialKeepAlive = 30 * time.Second
)

var (
	errBlockedAddress = errors.New("the address is link-local or a cloud metadata service")
	errNoAddress      = errors.New("the host has no address")
)

type lookupFunc func(ctx context.Context, host string) ([]netip.Addr, error)

type proxyFunc func(*http.Request) (*url.URL, error)

func systemLookup(ctx context.Context, host string) ([]netip.Addr, error) {
	return net.DefaultResolver.LookupNetIP(ctx, "ip", host)
}

func refuseBlocked(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	if settingspec.BlockedHost(host) {
		return errBlockedAddress
	}
	return nil
}

func checkHost(ctx context.Context, host string, lookup lookupFunc) error {
	if settingspec.BlockedHost(host) {
		return errBlockedAddress
	}
	if _, err := netip.ParseAddr(strings.Trim(host, "[]")); err == nil {
		return nil
	}
	addrs, err := lookup(ctx, host)
	if err != nil {
		return err
	}
	if len(addrs) == 0 {
		return errNoAddress
	}
	for _, a := range addrs {
		if settingspec.BlockedAddr(a) {
			return errBlockedAddress
		}
	}
	return nil
}

func guardProxy(next proxyFunc, lookup lookupFunc) proxyFunc {
	return func(r *http.Request) (*url.URL, error) {
		if err := checkHost(r.Context(), r.URL.Hostname(), lookup); err != nil {
			return nil, err
		}
		if next == nil {
			return nil, nil
		}
		return next(r)
	}
}

func guardedTransportWith(proxy proxyFunc, lookup lookupFunc) http.RoundTripper {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return http.DefaultTransport
	}
	t := base.Clone()
	t.Proxy = guardProxy(proxy, lookup)
	t.DialContext = (&net.Dialer{Timeout: dialTimeout, KeepAlive: dialKeepAlive, Control: refuseBlocked}).DialContext
	return t
}

func guardedTransport() http.RoundTripper {
	return guardedTransportWith(http.ProxyFromEnvironment, systemLookup)
}

func reasonOf(err error) string {
	var netErr net.Error
	var dnsErr *net.DNSError
	switch {
	case errors.Is(err, errBlockedAddress):
		return "the address is link-local or a cloud metadata service"
	case errors.Is(err, errInsecureEndpoint):
		return "an endpoint does not use https"
	case errors.Is(err, errNoKeys):
		return "the key set has no keys"
	case errors.Is(err, errTooLarge):
		return "the response is too large"
	case errors.Is(err, errNoRedirects):
		return "the provider answered with a redirect"
	case errors.Is(err, context.DeadlineExceeded):
		return "timed out"
	case errors.As(err, &dnsErr):
		return "the host name does not resolve"
	case errors.As(err, &netErr):
		return "the provider cannot be reached"
	default:
		return "the provider answered with something unexpected"
	}
}
