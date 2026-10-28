package agent

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fakeRunnerProxy(t *testing.T, allow string) string {
	t.Helper()
	sock := filepath.Join(t.TempDir(), "egress.sock")
	ln, err := net.Listen("unix", sock)
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
				br := bufio.NewReader(c)
				req, err := http.ReadRequest(br)
				if err != nil {
					return
				}
				if req.Method != http.MethodConnect || req.RequestURI != allow {
					_, _ = io.WriteString(c, "HTTP/1.1 403 Forbidden\r\nContent-Length: 22\r\n\r\ndhow egress: not here\n")
					return
				}
				_, _ = io.WriteString(c, "HTTP/1.1 200 Connection Established\r\n\r\n")
				_, _ = io.Copy(c, br)
			}()
		}
	}()
	return sock
}

func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()
	require.NoError(t, ln.Close())
	return addr
}

func TestConnectTunnelsStdinAndStdoutThroughTheProxy(t *testing.T) {
	sock := fakeRunnerProxy(t, "git.example.com:22")
	var out bytes.Buffer
	err := Connect(context.Background(), sock, "git.example.com", "22", strings.NewReader("SSH-2.0-test\r\n"), &out)
	require.NoError(t, err)
	assert.Equal(t, "SSH-2.0-test\r\n", out.String())
}

func TestConnectReportsARefusal(t *testing.T) {
	sock := fakeRunnerProxy(t, "git.example.com:22")
	err := Connect(context.Background(), sock, "evil.example.com", "22", strings.NewReader(""), io.Discard)
	assert.ErrorIs(t, err, ErrEgressRefused)
	assert.Contains(t, err.Error(), "not here")
}

func TestWithoutAnEgressSocketThereIsNoProxy(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "egress.sock")
	assert.ErrorIs(t, Connect(context.Background(), missing, "x.example.com", "443", strings.NewReader(""), io.Discard), ErrNoEgress)
	addr := freeAddr(t)
	stop, err := startForwarder(context.Background(), missing, []string{addr})
	require.NoError(t, err)
	defer stop()
	_, err = net.Dial("tcp", addr)
	assert.Error(t, err, "nothing listens on the proxy port when the job has no egress")
}

func TestTheForwarderCarriesProxyTrafficToTheRunner(t *testing.T) {
	sock := fakeRunnerProxy(t, "registry.npmjs.org:443")
	addr := freeAddr(t)
	stop, err := startForwarder(context.Background(), sock, []string{addr})
	require.NoError(t, err)
	defer stop()
	c, err := net.Dial("tcp", addr)
	require.NoError(t, err)
	defer func() { _ = c.Close() }()
	_, err = io.WriteString(c, "CONNECT registry.npmjs.org:443 HTTP/1.1\r\nHost: registry.npmjs.org:443\r\n\r\nhello")
	require.NoError(t, err)
	br := bufio.NewReader(c)
	resp, err := http.ReadResponse(br, &http.Request{Method: http.MethodConnect})
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	got := make([]byte, 5)
	_, err = io.ReadFull(br, got)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(got))
}
