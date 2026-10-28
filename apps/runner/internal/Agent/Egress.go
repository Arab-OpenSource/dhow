package agent

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

const (
	EgressSocketPath = "/run/dhow/control/egress.sock"
	ProxyPort        = "3128"
	egressDialWait   = 10 * time.Second
	maxForwards      = 256
)

var (
	ErrEgressRefused = errors.New("the runner's egress proxy refused the connection")
	ErrNoEgress      = errors.New("this job has no network.egress list, so it has no proxy")
)

func egressAddrs() []string {
	return []string{net.JoinHostPort("127.0.0.1", ProxyPort), net.JoinHostPort("::1", ProxyPort)}
}

func startForwarder(ctx context.Context, socket string, addrs []string) (func(), error) {
	if _, err := os.Stat(socket); errors.Is(err, os.ErrNotExist) {
		return func() {}, nil
	}
	var lc net.ListenConfig
	listeners := make([]net.Listener, 0, len(addrs))
	for i, addr := range addrs {
		ln, err := lc.Listen(ctx, "tcp", addr)
		if err != nil && i > 0 {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("open the egress proxy on %s: %w", addr, err)
		}
		listeners = append(listeners, ln)
	}
	slots := make(chan struct{}, maxForwards)
	for _, ln := range listeners {
		go forwardAll(ctx, ln, socket, slots)
	}
	return func() {
		for _, ln := range listeners {
			_ = ln.Close()
		}
	}, nil
}

func forwardAll(ctx context.Context, ln net.Listener, socket string, slots chan struct{}) {
	for {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		select {
		case slots <- struct{}{}:
			go func() {
				defer func() { <-slots }()
				forward(ctx, c, socket)
			}()
		default:
			_ = c.Close()
		}
	}
}

func dialEgress(ctx context.Context, socket string) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(ctx, egressDialWait)
	defer cancel()
	var d net.Dialer
	return d.DialContext(ctx, "unix", socket)
}

func forward(ctx context.Context, c net.Conn, socket string) {
	defer func() { _ = c.Close() }()
	up, err := dialEgress(ctx, socket)
	if err != nil {
		return
	}
	defer func() { _ = up.Close() }()
	pipe(c, c, up)
}

func pipe(in io.Reader, out io.Writer, up net.Conn) {
	var wg sync.WaitGroup
	wg.Go(func() {
		_, _ = io.Copy(up, in)
		if cw, ok := up.(interface{ CloseWrite() error }); ok {
			_ = cw.CloseWrite()
		}
	})
	_, _ = io.Copy(out, up)
	if cw, ok := out.(interface{ CloseWrite() error }); ok {
		_ = cw.CloseWrite()
	}
	_ = up.Close()
	wg.Wait()
}

func Connect(ctx context.Context, socket, host, port string, in io.Reader, out io.Writer) error {
	if _, err := os.Stat(socket); errors.Is(err, os.ErrNotExist) {
		return ErrNoEgress
	}
	up, err := dialEgress(ctx, socket)
	if err != nil {
		return fmt.Errorf("reach the runner's egress proxy: %w", err)
	}
	defer func() { _ = up.Close() }()
	target := net.JoinHostPort(host, port)
	if _, err := fmt.Fprintf(up, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", target, target); err != nil {
		return fmt.Errorf("ask the egress proxy for %s: %w", target, err)
	}
	br := bufio.NewReader(up)
	resp, err := http.ReadResponse(br, &http.Request{Method: http.MethodConnect})
	if err != nil {
		return fmt.Errorf("read the egress proxy's answer: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		why, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("%w: %s %s", ErrEgressRefused, resp.Status, why)
	}
	buffered, _ := br.Peek(br.Buffered())
	if _, err := out.Write(buffered); err != nil {
		return err
	}
	pipe(in, out, up)
	return nil
}
