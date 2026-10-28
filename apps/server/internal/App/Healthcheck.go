package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"time"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
)

const (
	readyPath        = "/api/v1/health/ready"
	loopbackHost     = "127.0.0.1"
	healthDrainBytes = 4 << 10
)

var ErrNotReady = errors.New("instance is not ready")

func RunHealthcheck(ctx context.Context) error {
	addr, err := config.LoadListenAddrFromEnv()
	if err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}
	return Healthcheck(ctx, addr, config.HealthcheckTimeout)
}

func Healthcheck(ctx context.Context, listenAddr string, timeout time.Duration) error {
	target, err := ReadyURL(listenAddr)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, http.NoBody)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, healthDrainBytes))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: %s answered %d", ErrNotReady, target, resp.StatusCode)
	}
	return nil
}

func ReadyURL(listenAddr string) (string, error) {
	host, port, err := net.SplitHostPort(listenAddr)
	if err != nil {
		return "", fmt.Errorf("listen address %q: %w", listenAddr, err)
	}
	if ip, perr := netip.ParseAddr(host); host == "" || (perr == nil && ip.IsUnspecified()) {
		host = loopbackHost
	}
	u := url.URL{Scheme: "http", Host: net.JoinHostPort(host, port), Path: readyPath}
	return u.String(), nil
}
