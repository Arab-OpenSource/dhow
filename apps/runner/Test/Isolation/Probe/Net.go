//go:build linux

package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"
)

const (
	proxyAddr = "127.0.0.1:3128"
	netWait   = 2 * time.Second
)

func dial(network, addr string) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), netWait)
	defer cancel()
	var d net.Dialer
	return d.DialContext(ctx, network, addr)
}

func networkNone() []result {
	_, err := dial("tcp", "1.1.1.1:443")
	out := []result{blockedUnless("direct TCP to the internet", err, "a TCP connection to 1.1.1.1:443")}
	ctx, cancel := context.WithTimeout(context.Background(), netWait)
	defer cancel()
	_, err = net.DefaultResolver.LookupHost(ctx, "example.com")
	out = append(out, blockedUnless("DNS", err, "resolving example.com"))
	u, err := dial("udp", "8.8.8.8:53")
	if err == nil {
		_, err = u.Write([]byte{0})
		_ = u.Close()
	}
	out = append(out, blockedUnless("UDP to the internet", err, "sending UDP to 8.8.8.8:53"))
	_, err = dial("tcp", proxyAddr)
	return append(out, blockedUnless("a proxy without network.egress", err, "connecting to "+proxyAddr))
}

func say(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stdout, format+"\n", args...)
}

func proxyTunnel(target string) int {
	c, err := dial("tcp", proxyAddr)
	if err != nil {
		say("no proxy: %v", err)
		return exitBlocked
	}
	defer func() { _ = c.Close() }()
	_ = c.SetDeadline(time.Now().Add(10 * time.Second))
	if _, err := fmt.Fprintf(c, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", target, target); err != nil {
		say("write: %v", err)
		return exitBlocked
	}
	br := bufio.NewReader(c)
	resp, err := http.ReadResponse(br, &http.Request{Method: http.MethodConnect})
	if err != nil {
		say("read: %v", err)
		return exitBlocked
	}
	defer func() { _ = resp.Body.Close() }()
	say("status %d", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		return exitBlocked
	}
	if _, err := io.WriteString(c, "ping"); err != nil {
		return exitBlocked
	}
	got := make([]byte, 4)
	if _, err := io.ReadFull(br, got); err != nil {
		say("echo: %v", err)
		return exitBlocked
	}
	say("echo %s", got)
	return 0
}
