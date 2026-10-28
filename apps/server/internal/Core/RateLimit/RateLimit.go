package ratelimit

import (
	"context"
	"errors"
	"net/netip"
	"regexp"
	"time"

	apperrors "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Errors"
)

const (
	MaxHits        = 1_000_000
	MinWindow      = time.Second
	MaxKeyLength   = 200
	UnknownNetwork = "unknown"
	IPv6Prefix     = 64
)

var (
	ErrInvalidLimit = errors.New("rate limit: invalid limit")
	ErrInvalidKey   = errors.New("rate limit: invalid key")

	namePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)
)

type Limit struct {
	Name   string
	Max    int
	Window time.Duration
}

func (l Limit) Valid() bool {
	return namePattern.MatchString(l.Name) && l.Max >= 1 && l.Max <= MaxHits && l.Window >= MinWindow
}

type Decision struct {
	Allowed    bool
	RetryAfter time.Duration
}

func (d Decision) Err() error {
	if d.Allowed {
		return nil
	}
	return apperrors.RateLimited(d.RetryAfter)
}

type Limiter interface {
	Take(ctx context.Context, l Limit, subject string) (Decision, error)
}

func Key(l Limit, subject string) (string, error) {
	if !l.Valid() {
		return "", ErrInvalidLimit
	}
	key := l.Name + ":" + subject
	if subject == "" || len(key) > MaxKeyLength {
		return "", ErrInvalidKey
	}
	return key, nil
}

func Network(a netip.Addr) string {
	if !a.IsValid() {
		return UnknownNetwork
	}
	a = a.Unmap()
	if a.Is4() {
		return a.String()
	}
	p, err := a.WithZone("").Prefix(IPv6Prefix)
	if err != nil {
		return UnknownNetwork
	}
	return p.String()
}

func NetworkOf(ip string) string {
	a, err := netip.ParseAddr(ip)
	if err != nil {
		return UnknownNetwork
	}
	return Network(a)
}

func clampRetry(d, window time.Duration) time.Duration {
	return min(max(d, MinWindow), window)
}
