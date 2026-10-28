package egress

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"sync"
	"time"

	"github.com/Khaymah/dhow/internal/jobspec"
)

const (
	DefaultMaxConns      = 256
	DefaultIdle          = 5 * time.Minute
	DefaultDialTimeout   = 10 * time.Second
	DefaultHeaderTimeout = 30 * time.Second
	maxHeaderBytes       = 16 << 10
)

var ErrNoRules = errors.New("the job has no egress allowlist, so it gets no proxy")

type Resolver interface {
	LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
}

type Decision struct {
	Host    string
	Port    int
	Allowed bool
	Reason  string
}

type Options struct {
	Allow         []string
	AllowPrivate  []netip.Prefix
	HostAddrs     []netip.Addr
	Resolver      Resolver
	Dial          func(ctx context.Context, network, address string) (net.Conn, error)
	Report        func(Decision)
	MaxConns      int
	Idle          time.Duration
	DialTimeout   time.Duration
	HeaderTimeout time.Duration
}

type Proxy struct {
	rules  []jobspec.EgressRule
	ranges ranges
	o      Options
}

func New(o Options) (*Proxy, error) {
	if len(o.Allow) == 0 {
		return nil, ErrNoRules
	}
	rules := make([]jobspec.EgressRule, 0, len(o.Allow))
	for _, entry := range o.Allow {
		r, err := jobspec.ParseEgress(entry)
		if err != nil {
			return nil, fmt.Errorf("egress entry %q: %w", entry, err)
		}
		rules = append(rules, r)
	}
	if o.Resolver == nil {
		o.Resolver = net.DefaultResolver
	}
	if o.Dial == nil {
		var d net.Dialer
		o.Dial = d.DialContext
	}
	if o.Report == nil {
		o.Report = func(Decision) {}
	}
	o.MaxConns = positive(o.MaxConns, DefaultMaxConns)
	o.Idle = positiveDuration(o.Idle, DefaultIdle)
	o.DialTimeout = positiveDuration(o.DialTimeout, DefaultDialTimeout)
	o.HeaderTimeout = positiveDuration(o.HeaderTimeout, DefaultHeaderTimeout)
	return &Proxy{rules: rules, ranges: ranges{blocked: blockedPrefixes(), host: o.HostAddrs, allowed: o.AllowPrivate}, o: o}, nil
}

func (p *Proxy) Serve(ctx context.Context, ln net.Listener) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	slots := make(chan struct{}, p.o.MaxConns)
	var live sync.WaitGroup
	defer live.Wait()
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()
	for {
		c, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return fmt.Errorf("accept an egress connection: %w", err)
		}
		select {
		case slots <- struct{}{}:
			live.Go(func() {
				defer func() { <-slots }()
				p.handle(ctx, c)
			})
		default:
			refuse(c, statusBusy, "too many connections from this job")
			_ = c.Close()
		}
	}
}

func (p *Proxy) decide(host string, port int, tunnel bool) (string, string) {
	name, err := jobspec.NormalizeHost(host)
	if errors.Is(err, jobspec.ErrEgressIP) {
		return printable(host), "an IP address, and the allowlist holds host names only"
	}
	if err != nil {
		return printable(host), "not a valid host name"
	}
	for _, r := range p.rules {
		if r.Allows(name, port, tunnel) {
			return name, ""
		}
	}
	return name, "not in the job's network.egress list"
}

func (p *Proxy) dial(ctx context.Context, name string, port int) (net.Conn, string, int) {
	lctx, cancel := context.WithTimeout(ctx, p.o.DialTimeout)
	defer cancel()
	addrs, err := p.o.Resolver.LookupNetIP(lctx, "ip", name)
	if err != nil || len(addrs) == 0 {
		return nil, "the name did not resolve", statusBadGateway
	}
	denial, status := "", statusForbidden
	for _, addr := range addrs {
		if why := p.ranges.denial(addr); why != "" {
			if denial == "" {
				denial = why
			}
			continue
		}
		dctx, dcancel := context.WithTimeout(ctx, p.o.DialTimeout)
		c, err := p.o.Dial(dctx, "tcp", netip.AddrPortFrom(addr.Unmap(), uint16(port)).String()) //nolint:gosec // The port was parsed into 1..65535.
		dcancel()
		if err == nil {
			return c, "", 0
		}
		denial, status = "the connection failed", statusBadGateway
	}
	return nil, denial, status
}

func (p *Proxy) report(host string, port int, reason string) {
	p.o.Report(Decision{Host: host, Port: port, Allowed: reason == "", Reason: reason})
}

func positive(v, def int) int {
	if v > 0 {
		return v
	}
	return def
}

func positiveDuration(v, def time.Duration) time.Duration {
	if v > 0 {
		return v
	}
	return def
}

func printable(host string) string {
	const maxShown = 64
	if len(host) > maxShown {
		host = host[:maxShown]
	}
	return strconv.QuoteToASCII(host)
}
