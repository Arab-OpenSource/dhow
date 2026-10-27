package auth

import (
	"bufio"
	"context"
	"crypto/sha1" //nolint:gosec // The Have I Been Pwned range API is defined over SHA-1; the digest never protects anything.
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	rangePrefixLength = 5
	maxRangeBody      = 4 << 20
)

type BreachChecker interface {
	IsBreached(ctx context.Context, password string) (bool, error)
}

type NoBreachCheck struct{}

func (NoBreachCheck) IsBreached(context.Context, string) (bool, error) { return false, nil }

type HIBP struct {
	client  *http.Client
	baseURL string
}

func NewHIBP(baseURL string, timeout time.Duration, transport http.RoundTripper) *HIBP {
	return &HIBP{
		baseURL: baseURL,
		client: &http.Client{
			Timeout:       timeout,
			Transport:     transport,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

func (h *HIBP) IsBreached(ctx context.Context, password string) (bool, error) {
	sum := sha1.Sum([]byte(password)) //nolint:gosec // See the import: the range API requires SHA-1.
	digest := strings.ToUpper(hex.EncodeToString(sum[:]))
	prefix, suffix := digest[:rangePrefixLength], digest[rangePrefixLength:]

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.baseURL+prefix, http.NoBody)
	if err != nil {
		return false, fmt.Errorf("build breached-password request: %w", err)
	}
	req.Header.Set("Add-Padding", "true")
	req.Header.Set("User-Agent", "dhow-server")
	resp, err := h.client.Do(req)
	if err != nil {
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return false, fmt.Errorf("breached-password range request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("breached-password range request: status %d", resp.StatusCode)
	}
	return MatchRange(io.LimitReader(resp.Body, maxRangeBody), suffix)
}

func MatchRange(r io.Reader, suffix string) (bool, error) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		candidate, count, ok := strings.Cut(line, ":")
		if !ok || !strings.EqualFold(candidate, suffix) {
			continue
		}
		n, err := strconv.Atoi(strings.TrimSpace(count))
		if err != nil {
			return false, fmt.Errorf("parse breach count: %w", err)
		}
		return n > 0, nil
	}
	if err := sc.Err(); err != nil {
		return false, fmt.Errorf("read breached-password range: %w", err)
	}
	return false, nil
}
