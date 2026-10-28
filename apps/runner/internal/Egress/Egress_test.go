package egress

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeResolver struct {
	mu      sync.Mutex
	answers map[string][][]netip.Addr
}

func (f *fakeResolver) LookupNetIP(_ context.Context, _, host string) ([]netip.Addr, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	host = strings.TrimSuffix(host, ".")
	queue := f.answers[host]
	if len(queue) == 0 {
		return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
	}
	answer := queue[0]
	if len(queue) > 1 {
		f.answers[host] = queue[1:]
	}
	return answer, nil
}

func addrs(list ...string) []netip.Addr {
	out := make([]netip.Addr, 0, len(list))
	for _, a := range list {
		out = append(out, netip.MustParseAddr(a))
	}
	return out
}

type fixture struct {
	proxy  string
	dialed chan string
	report chan Decision
	target net.Listener
}

func echoServer(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = c.Close() }()
				_, _ = io.Copy(c, c)
			}()
		}
	}()
	return ln
}

func start(t *testing.T, o Options, answers map[string][][]netip.Addr) fixture {
	t.Helper()
	f := fixture{dialed: make(chan string, 16), report: make(chan Decision, 16), target: echoServer(t)}
	if o.Resolver == nil {
		o.Resolver = &fakeResolver{answers: answers}
	}
	o.Dial = func(ctx context.Context, network, address string) (net.Conn, error) {
		f.dialed <- address
		var d net.Dialer
		return d.DialContext(ctx, network, f.target.Addr().String())
	}
	o.Report = func(d Decision) { f.report <- d }
	p, err := New(o)
	require.NoError(t, err)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- p.Serve(ctx, ln) }()
	t.Cleanup(func() {
		cancel()
		require.NoError(t, <-done)
	})
	f.proxy = ln.Addr().String()
	return f
}

func connect(t *testing.T, proxy, target string) (net.Conn, *bufio.Reader, int) {
	t.Helper()
	c, err := net.Dial("tcp", proxy)
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })
	_, err = io.WriteString(c, "CONNECT "+target+" HTTP/1.1\r\nHost: "+target+"\r\n\r\n")
	require.NoError(t, err)
	br := bufio.NewReader(c)
	resp, err := http.ReadResponse(br, nil)
	require.NoError(t, err)
	_ = resp.Body.Close()
	return c, br, resp.StatusCode
}

const public = "93.184.216.34"

func TestAnAllowedHostIsTunnelledToTheAddressItResolvedTo(t *testing.T) {
	f := start(t, Options{Allow: []string{"registry.npmjs.org"}}, map[string][][]netip.Addr{"registry.npmjs.org": {addrs(public)}})
	c, br, status := connect(t, f.proxy, "Registry.NPMJS.org.:443")
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, public+":443", <-f.dialed)
	_, err := io.WriteString(c, "ping")
	require.NoError(t, err)
	got := make([]byte, 4)
	_, err = io.ReadFull(br, got)
	require.NoError(t, err)
	assert.Equal(t, "ping", string(got))
	assert.Equal(t, Decision{Host: "registry.npmjs.org", Port: 443, Allowed: true}, <-f.report)
}

func TestTheProxyRefusesWhatTheJobDidNotAllow(t *testing.T) {
	answers := map[string][][]netip.Addr{
		"ok.example.com":      {addrs(public)},
		"private.example.com": {addrs("10.1.2.3")},
		"loop.example.com":    {addrs("127.0.0.1")},
		"mapped.example.com":  {addrs("::ffff:127.0.0.1")},
		"meta.example.com":    {addrs("169.254.169.254")},
		"meta6.example.com":   {addrs("fd00:ec2::254")},
		"ula.example.com":     {addrs("fd12::1")},
		"self.example.com":    {addrs("203.0.113.9")},
		"mixed.example.com":   {addrs("10.0.0.1", "192.168.1.1")},
	}
	allow := []string{
		"ok.example.com", "private.example.com", "loop.example.com", "mapped.example.com", "meta.example.com",
		"meta6.example.com", "ula.example.com", "self.example.com", "mixed.example.com", "nxdomain.example.com",
	}
	f := start(t, Options{Allow: allow, HostAddrs: addrs("203.0.113.9")}, answers)
	tests := []struct {
		target string
		status int
		reason string
	}{
		{"ok.example.com:8443", http.StatusForbidden, "not in the job's network.egress list"},
		{"other.example.com:443", http.StatusForbidden, "not in the job's network.egress list"},
		{"sub.ok.example.com:443", http.StatusForbidden, "not in the job's network.egress list"},
		{"private.example.com:443", http.StatusForbidden, "a private, loopback or reserved address"},
		{"loop.example.com:443", http.StatusForbidden, "a private, loopback or reserved address"},
		{"mapped.example.com:443", http.StatusForbidden, "a private, loopback or reserved address"},
		{"meta.example.com:443", http.StatusForbidden, "a cloud metadata address"},
		{"meta6.example.com:443", http.StatusForbidden, "a cloud metadata address"},
		{"ula.example.com:443", http.StatusForbidden, "a private, loopback or reserved address"},
		{"self.example.com:443", http.StatusForbidden, "an address of the runner host"},
		{"mixed.example.com:443", http.StatusForbidden, "a private, loopback or reserved address"},
		{"nxdomain.example.com:443", http.StatusBadGateway, "the name did not resolve"},
		{"1.2.3.4:443", http.StatusForbidden, "an IP address, and the allowlist holds host names only"},
		{"[::1]:443", http.StatusForbidden, "an IP address, and the allowlist holds host names only"},
		{"2130706433:443", http.StatusForbidden, "an IP address, and the allowlist holds host names only"},
	}
	for _, tt := range tests {
		_, _, status := connect(t, f.proxy, tt.target)
		assert.Equal(t, tt.status, status, tt.target)
		d := <-f.report
		assert.False(t, d.Allowed, tt.target)
		assert.Equal(t, tt.reason, d.Reason, tt.target)
	}
	select {
	case a := <-f.dialed:
		t.Fatalf("a refused target was dialed: %s", a)
	default:
	}
}

func TestTheOperatorCanOpenAPrivateRangeButNeverMetadata(t *testing.T) {
	answers := map[string][][]netip.Addr{
		"registry.internal": {addrs("10.20.0.5")},
		"meta.internal":     {addrs("169.254.169.254")},
		"self.internal":     {addrs("10.20.0.1")},
	}
	f := start(t, Options{
		Allow:        []string{"registry.internal", "meta.internal", "self.internal"},
		AllowPrivate: []netip.Prefix{netip.MustParsePrefix("10.20.0.0/16"), netip.MustParsePrefix("169.254.0.0/16")},
		HostAddrs:    addrs("10.20.0.1"),
	}, answers)
	_, _, status := connect(t, f.proxy, "registry.internal:443")
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, "10.20.0.5:443", <-f.dialed)
	<-f.report
	_, _, status = connect(t, f.proxy, "self.internal:443")
	assert.Equal(t, http.StatusOK, status, "an operator range covers the host's own address when it is written out")
	<-f.dialed
	<-f.report
	_, _, status = connect(t, f.proxy, "meta.internal:443")
	assert.Equal(t, http.StatusForbidden, status)
	assert.Equal(t, "a cloud metadata address", (<-f.report).Reason)
}

func TestARebindingAnswerIsCheckedOnEveryConnection(t *testing.T) {
	answers := map[string][][]netip.Addr{"rebind.example.com": {addrs(public), addrs("127.0.0.1")}}
	f := start(t, Options{Allow: []string{"rebind.example.com"}}, answers)
	_, _, status := connect(t, f.proxy, "rebind.example.com:443")
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, public+":443", <-f.dialed)
	<-f.report
	_, _, status = connect(t, f.proxy, "rebind.example.com:443")
	assert.Equal(t, http.StatusForbidden, status)
	assert.Equal(t, "a private, loopback or reserved address", (<-f.report).Reason)
	select {
	case a := <-f.dialed:
		t.Fatalf("the rebound address was dialed: %s", a)
	default:
	}
}
