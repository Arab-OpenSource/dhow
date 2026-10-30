package netguard

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"syscall"
	"time"
)

const (
	DefaultTimeout   = 10 * time.Second
	MaxResponseBytes = 5 << 20
	maxAddresses     = 16
)

var (
	ErrRedirect = errors.New("the server answered with a redirect, and redirects are not followed")
	ErrInsecure = errors.New("only https is allowed")
	ErrTooLarge = errors.New("the response is too large")
)

type Lookup func(ctx context.Context, host string) ([]netip.Addr, error)

type dialFunc func(ctx context.Context, network, address string) (net.Conn, error)

type Dialer struct {
	Policy  Policy
	Lookup  Lookup
	Timeout time.Duration
	dial    dialFunc
}

func systemLookup(ctx context.Context, host string) ([]netip.Addr, error) {
	return net.DefaultResolver.LookupNetIP(ctx, "ip", host)
}

func (d Dialer) timeout() time.Duration {
	if d.Timeout > 0 {
		return d.Timeout
	}
	return DefaultTimeout
}

func (d Dialer) Resolve(ctx context.Context, host string) ([]netip.Addr, error) {
	host = strings.TrimSuffix(strings.Trim(host, "[]"), ".")
	if addr, err := netip.ParseAddr(host); err == nil {
		if err := d.Policy.Check(addr); err != nil {
			return nil, err
		}
		return []netip.Addr{addr.Unmap()}, nil
	}
	lookup := d.Lookup
	if lookup == nil {
		lookup = systemLookup
	}
	addrs, err := lookup(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("resolve the host: %w", err)
	}
	if len(addrs) == 0 {
		return nil, ErrNoAddress
	}
	if len(addrs) > maxAddresses {
		addrs = addrs[:maxAddresses]
	}
	out := make([]netip.Addr, 0, len(addrs))
	for _, a := range addrs {
		if err := d.Policy.Check(a); err != nil {
			return nil, err
		}
		out = append(out, a.Unmap())
	}
	return out, nil
}

func (d Dialer) Control(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return fmt.Errorf("%w: the dialed address is not an IP", ErrBlocked)
	}
	return d.Policy.Check(addr)
}

func (d Dialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	addrs, err := d.Resolve(ctx, host)
	if err != nil {
		return nil, err
	}
	dial := d.dial
	if dial == nil {
		dial = (&net.Dialer{Timeout: d.timeout(), Control: d.Control}).DialContext
	}
	var errs []error
	for _, a := range addrs {
		conn, err := dial(ctx, network, net.JoinHostPort(a.String(), port))
		if err == nil {
			return conn, nil
		}
		errs = append(errs, err)
	}
	return nil, errors.Join(errs...)
}

type ClientOptions struct {
	AllowHTTP bool
	TLS       *tls.Config
}

type schemeGuard struct {
	next      http.RoundTripper
	allowHTTP bool
}

func (g schemeGuard) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Scheme != "https" && (!g.allowHTTP || r.URL.Scheme != "http") {
		return nil, ErrInsecure
	}
	return g.next.RoundTrip(r)
}

func (d Dialer) HTTPClient(o ClientOptions) *http.Client {
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if o.TLS != nil {
		tlsConfig = o.TLS.Clone()
	}
	transport := &http.Transport{
		Proxy:                 nil,
		DialContext:           d.DialContext,
		TLSClientConfig:       tlsConfig,
		TLSHandshakeTimeout:   d.timeout(),
		ResponseHeaderTimeout: d.timeout(),
		MaxIdleConns:          4,
		IdleConnTimeout:       30 * time.Second,
		ForceAttemptHTTP2:     true,
	}
	return &http.Client{
		Transport: schemeGuard{next: transport, allowHTTP: o.AllowHTTP},
		Timeout:   d.timeout(),
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return ErrRedirect
		},
	}
}

func ReadLimited(r io.Reader, limit int64) ([]byte, error) {
	if limit <= 0 || limit > MaxResponseBytes {
		limit = MaxResponseBytes
	}
	body, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, ErrTooLarge
	}
	return body, nil
}
