package mptcp_test

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func listen(t *testing.T, lc *net.ListenConfig) *net.TCPListener {
	t.Helper()
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	tcp, ok := ln.(*net.TCPListener)
	require.True(t, ok)
	return tcp
}

func socketIsMPTCP(t *testing.T, ln *net.TCPListener) bool {
	t.Helper()
	raw, err := ln.SyscallConn()
	require.NoError(t, err)
	proto, sockErr := 0, error(nil)
	require.NoError(t, raw.Control(func(fd uintptr) {
		proto, sockErr = unix.GetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_PROTOCOL)
	}))
	require.NoError(t, sockErr)
	return proto == unix.IPPROTO_MPTCP
}

func mptcpWorksHere(t *testing.T) *net.TCPListener {
	t.Helper()
	var forced net.ListenConfig
	forced.SetMultipathTCP(true)
	ln := listen(t, &forced)
	if !socketIsMPTCP(t, ln) {
		t.Skip("this kernel did not open an MPTCP listener even when asked (MPTCP not built in, or net.mptcp.enabled=0), so the default cannot be told apart here")
	}
	return ln
}

func TestDefaultListenersAreNotMPTCP(t *testing.T) {
	mptcpWorksHere(t)
	assert.False(t, socketIsMPTCP(t, listen(t, &net.ListenConfig{})), "net.ListenConfig{} opened an MPTCP listener: the go.mod godebug multipathtcp=0 line is missing or overridden by GODEBUG")

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	tcp, ok := ln.(*net.TCPListener)
	require.True(t, ok)
	assert.False(t, socketIsMPTCP(t, tcp), "net.Listen opened an MPTCP listener")
}

func TestDefaultDialersDoNotUseMPTCP(t *testing.T) {
	ln := mptcpWorksHere(t)
	accepted := make(chan net.Conn, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			close(accepted)
			return
		}
		accepted <- c
	}()
	var d net.Dialer
	conn, err := d.DialContext(context.Background(), "tcp", ln.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	server, ok := <-accepted
	require.True(t, ok)
	t.Cleanup(func() { _ = server.Close() })

	tcp, ok := conn.(*net.TCPConn)
	require.True(t, ok)
	used, err := tcp.MultipathTCP()
	require.NoError(t, err)
	assert.False(t, used, "a default dialer used MPTCP against an MPTCP listener")
}
