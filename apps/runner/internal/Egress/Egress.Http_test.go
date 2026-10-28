package egress

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func send(t *testing.T, proxy, raw string) (int, string) {
	t.Helper()
	c, err := net.Dial("tcp", proxy)
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })
	_, err = io.WriteString(c, raw)
	require.NoError(t, err)
	resp, err := http.ReadResponse(bufio.NewReader(c), nil)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

func TestAPlainHTTPRequestIsForwardedWithoutProxyHeaders(t *testing.T) {
	seen := make(chan *http.Request, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen <- r
		_, _ = io.WriteString(w, "hello from upstream")
	}))
	t.Cleanup(upstream.Close)
	p, err := New(Options{
		Allow:    []string{"deb.debian.org"},
		Resolver: &fakeResolver{answers: map[string][][]netip.Addr{"deb.debian.org": {addrs(public)}}},
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, network, upstream.Listener.Addr().String())
		},
	})
	require.NoError(t, err)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = p.Serve(ctx, ln) }()
	status, body := send(t, ln.Addr().String(), "GET http://deb.debian.org/debian/dists HTTP/1.1\r\nHost: deb.debian.org\r\n"+
		"Proxy-Authorization: Basic c2VjcmV0\r\nProxy-Connection: keep-alive\r\nX-Keep: yes\r\n\r\n")
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, "hello from upstream", body)
	r := <-seen
	assert.Equal(t, "/debian/dists", r.RequestURI)
	assert.Empty(t, r.Header.Get("Proxy-Authorization"))
	assert.Empty(t, r.Header.Get("Proxy-Connection"))
	assert.Equal(t, "yes", r.Header.Get("X-Keep"))
}

func TestRequestsTheProxyDoesNotServeAreRefused(t *testing.T) {
	f := start(t, Options{Allow: []string{"ok.example.com"}}, map[string][][]netip.Addr{"ok.example.com": {addrs(public)}})
	tests := []struct {
		name   string
		raw    string
		status int
	}{
		{"an https URL without CONNECT", "GET https://ok.example.com/ HTTP/1.1\r\nHost: ok.example.com\r\n\r\n", http.StatusBadRequest},
		{"an origin-form request", "GET / HTTP/1.1\r\nHost: ok.example.com\r\n\r\n", http.StatusBadRequest},
		{"CONNECT without a port", "CONNECT ok.example.com HTTP/1.1\r\nHost: ok.example.com\r\n\r\n", http.StatusBadRequest},
		{"CONNECT to port zero", "CONNECT ok.example.com:0 HTTP/1.1\r\nHost: x\r\n\r\n", http.StatusBadRequest},
		{"plain HTTP to a port the rule does not open", "GET http://ok.example.com:8080/ HTTP/1.1\r\nHost: ok.example.com\r\n\r\n", http.StatusForbidden},
		{"a request head over the limit", "GET http://ok.example.com/ HTTP/1.1\r\nX-Big: " + strings.Repeat("a", maxHeaderBytes) + "\r\n\r\n", http.StatusBadRequest},
		{"garbage", "\x00\x01\x02 not http\r\n\r\n", http.StatusBadRequest},
	}
	for _, tt := range tests {
		status, body := send(t, f.proxy, tt.raw)
		assert.Equal(t, tt.status, status, tt.name)
		assert.True(t, strings.HasPrefix(body, "dhow egress: "), tt.name)
	}
}

func TestTheProxyCapsConnectionsPerJob(t *testing.T) {
	f := start(t, Options{Allow: []string{"ok.example.com"}, MaxConns: 1}, map[string][][]netip.Addr{"ok.example.com": {addrs(public), addrs(public)}})
	_, _, status := connect(t, f.proxy, "ok.example.com:443")
	require.Equal(t, http.StatusOK, status)
	status, _ = send(t, f.proxy, "CONNECT ok.example.com:443 HTTP/1.1\r\nHost: x\r\n\r\n")
	assert.Equal(t, http.StatusServiceUnavailable, status)
}

func TestAnIdleTunnelIsClosed(t *testing.T) {
	f := start(t, Options{Allow: []string{"ok.example.com"}, Idle: 100 * time.Millisecond}, map[string][][]netip.Addr{"ok.example.com": {addrs(public)}})
	c, br, status := connect(t, f.proxy, "ok.example.com:443")
	require.Equal(t, http.StatusOK, status)
	_ = c.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, err := br.ReadByte()
	assert.ErrorIs(t, err, io.EOF, "the proxy closed the idle tunnel")
}

func TestAnEmptyAllowlistGetsNoProxy(t *testing.T) {
	_, err := New(Options{})
	assert.ErrorIs(t, err, ErrNoRules)
	_, err = New(Options{Allow: []string{"10.0.0.1"}})
	assert.Error(t, err)
}

func TestTheReporterWritesEachDecisionOnceAMinute(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	var lines []string
	r := NewReporter(func() time.Time { return now }, func(l string) { lines = append(lines, l) })
	ok := Decision{Host: "registry.npmjs.org", Port: 443, Allowed: true}
	denied := Decision{Host: "evil.example.com", Port: 443, Reason: "not in the job's network.egress list"}
	r.Report(ok)
	r.Report(ok)
	r.Report(denied)
	now = now.Add(30 * time.Second)
	r.Report(ok)
	now = now.Add(31 * time.Second)
	r.Report(ok)
	assert.Equal(t, []string{
		"dhow: egress allowed registry.npmjs.org:443",
		"dhow: egress denied evil.example.com:443 (not in the job's network.egress list)",
		"dhow: egress allowed registry.npmjs.org:443",
	}, lines)
}

func TestAnInvalidHostIsQuotedInTheLog(t *testing.T) {
	f := start(t, Options{Allow: []string{"ok.example.com"}}, nil)
	_, _ = send(t, f.proxy, "CONNECT \x1b[31mred.example.com:443 HTTP/1.1\r\nHost: x\r\n\r\n")
	select {
	case d := <-f.report:
		assert.NotContains(t, d.Host, "\x1b")
	case <-time.After(2 * time.Second):
	}
}

func FuzzProxyRequest(f *testing.F) {
	for _, seed := range []string{
		"CONNECT ok.example.com:443 HTTP/1.1\r\nHost: x\r\n\r\n",
		"GET http://ok.example.com/ HTTP/1.1\r\nHost: ok.example.com\r\n\r\n",
		"CONNECT [::1]:443 HTTP/1.1\r\n\r\n",
		"GET http://[fe80::1%25eth0]/ HTTP/1.1\r\n\r\n",
	} {
		f.Add(seed)
	}
	p, err := New(Options{
		Allow:    []string{"ok.example.com", "*.example.org:8443"},
		Resolver: &fakeResolver{answers: map[string][][]netip.Addr{}},
		Dial: func(context.Context, string, string) (net.Conn, error) {
			return nil, net.ErrClosed
		},
		HeaderTimeout: 200 * time.Millisecond,
	})
	if err != nil {
		f.Fatal(err)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		client, server := net.Pipe()
		done := make(chan struct{})
		go func() {
			p.handle(context.Background(), server)
			close(done)
		}()
		go func() {
			_, _ = io.WriteString(client, raw)
		}()
		_ = client.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, _ = io.Copy(io.Discard, client)
		_ = client.Close()
		<-done
	})
}
