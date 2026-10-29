package e2e_test

import (
	"bufio"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apptest "github.com/Khaymah/dhow/apps/server/internal/App/Testing"
)

type sink struct {
	ln   net.Listener
	mu   sync.Mutex
	rcpt []string
	data []string
}

func newSink(t *testing.T) *sink {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	s := &sink{ln: ln}
	go s.accept()
	t.Cleanup(func() { _ = ln.Close() })
	return s
}

func (s *sink) port() int { return s.ln.Addr().(*net.TCPAddr).Port }

func (s *sink) accept() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.serve(conn)
	}
}

func (s *sink) serve(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	r := bufio.NewReader(conn)
	say := func(line string) { _, _ = conn.Write([]byte(line + "\r\n")) }
	say("220 sink ready")
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.ToUpper(strings.TrimSpace(line))
		switch {
		case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
			say("250 sink")
		case strings.HasPrefix(cmd, "MAIL FROM"):
			say("250 ok")
		case strings.HasPrefix(cmd, "RCPT TO"):
			s.mu.Lock()
			s.rcpt = append(s.rcpt, strings.TrimSpace(line[len("RCPT TO:"):]))
			s.mu.Unlock()
			say("250 ok")
		case cmd == "DATA":
			say("354 go on")
			var body strings.Builder
			for {
				l, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if l == ".\r\n" {
					break
				}
				body.WriteString(l)
			}
			s.mu.Lock()
			s.data = append(s.data, body.String())
			s.mu.Unlock()
			say("250 queued")
		case cmd == "QUIT":
			say("221 bye")
			return
		default:
			say("502 not here")
		}
	}
}

func (s *sink) messages() ([]string, []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.rcpt...), append([]string(nil), s.data...)
}

func sendTest(c *apptest.Client) apptest.Response {
	return c.Do(apptest.Request{Method: http.MethodPost, Path: "/instance/settings/mail/test"})
}

func TestTheTestEmailGoesToTheOwnerThroughTheSettingsInEffect(t *testing.T) {
	env := apptest.New(t)
	owner, _, fresh := ownerWithTotp(t, env)

	off := sendTest(owner)
	assert.Equal(t, http.StatusConflict, off.Status, string(off.Raw))
	assert.Equal(t, "MAIL_NOT_CONFIGURED", errorCode(off))

	closed, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	deadPort := closed.Addr().(*net.TCPAddr).Port
	require.NoError(t, closed.Close())
	save(t, owner, "mail", fresh(), map[string]any{"Host": "127.0.0.1", "Port": deadPort, "Tls": "none", "From": "Dhow <dhow@example.com>"})
	failed := sendTest(owner)
	assert.Equal(t, http.StatusBadGateway, failed.Status, string(failed.Raw))
	assert.Equal(t, "MAIL_TEST_FAILED", errorCode(failed))
	assert.NotContains(t, string(failed.Raw), strconv.Itoa(deadPort), "the reason stays in the server log")
	assert.Contains(t, env.Logs.String(), "the test email was not sent")

	s := newSink(t)
	save(t, owner, "mail", fresh(), map[string]any{"Port": s.port()})
	ok := sendTest(owner)
	require.Equal(t, http.StatusOK, ok.Status, string(ok.Raw))
	var data struct{ SentTo string }
	ok.Decode(t, &data)
	assert.Equal(t, "owner@example.com", strings.ToLower(data.SentTo))
	rcpt, bodies := s.messages()
	require.Len(t, bodies, 1)
	assert.Contains(t, strings.ToLower(rcpt[0]), "owner@example.com")
	assert.Contains(t, bodies[0], "From: \"Dhow\" <dhow@example.com>")

	for range 2 {
		require.Equal(t, http.StatusOK, sendTest(owner).Status)
	}
	limited := sendTest(owner)
	assert.Equal(t, http.StatusTooManyRequests, limited.Status, string(limited.Raw))
	assert.Equal(t, "RATE_LIMITED", errorCode(limited))
	assert.NotEmpty(t, limited.Header.Get("Retry-After"))
}
