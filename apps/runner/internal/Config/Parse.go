package config

import (
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	ErrNotHTTPS     = errors.New("must use https:// (plain http:// is allowed only with --dev on localhost)")
	ErrDevLocalOnly = errors.New("plain http:// is allowed only for localhost, 127.0.0.1 or [::1]")
	ErrURLShape     = errors.New("must be a bare server address like https://dhow.example.com, without a path, query, fragment or credentials")
	ErrBadNumber    = errors.New("is not a valid number")
	ErrBadSize      = errors.New("is not a valid size (use bytes or a unit: KiB, MiB, GiB, TiB)")
	ErrBadBool      = errors.New("must be true or false")
	ErrBadLevel     = errors.New("must be debug, info, warn or error")
	ErrBadPrefix    = errors.New("must be a comma separated list of CIDR prefixes")
)

var sizeUnits = []struct {
	suffix string
	factor int64
}{
	{"TiB", 1 << 40}, {"GiB", 1 << 30}, {"MiB", 1 << 20}, {"KiB", 1 << 10},
	{"T", 1 << 40}, {"G", 1 << 30}, {"M", 1 << 20}, {"K", 1 << 10}, {"B", 1},
}

func ParseServerURL(raw string, dev bool) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || u.Opaque != "" {
		return nil, ErrURLShape
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, ErrURLShape
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
	case "http":
		if !dev {
			return nil, ErrNotHTTPS
		}
		if !isLoopbackHost(u.Hostname()) {
			return nil, ErrDevLocalOnly
		}
	default:
		return nil, ErrNotHTTPS
	}
	return &url.URL{Scheme: strings.ToLower(u.Scheme), Host: strings.ToLower(u.Host)}, nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func OriginOf(u *url.URL) string {
	host := strings.ToLower(u.Host)
	switch u.Scheme {
	case "https":
		host = strings.TrimSuffix(host, ":443")
	case "http":
		host = strings.TrimSuffix(host, ":80")
	}
	return strings.ToLower(u.Scheme) + "://" + host
}

func loadOrigin(raw string, u *url.URL, dev bool) (string, error) {
	if raw == "" {
		if u == nil {
			return "", nil
		}
		return OriginOf(u), nil
	}
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" ||
		(parsed.Path != "" && parsed.Path != "/") {
		return "", ErrURLShape
	}
	if parsed.Scheme != "https" && !(dev && parsed.Scheme == "http") {
		return "", ErrNotHTTPS
	}
	return OriginOf(parsed), nil
}

func checkCAFile(path string) error {
	data, err := os.ReadFile(path) //nolint:gosec // An operator-chosen CA bundle path.
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if !x509.NewCertPool().AppendCertsFromPEM(data) {
		return fmt.Errorf("%s %w", path, ErrCAFileNoCerts)
	}
	return nil
}

func parseIntRange(v string, def, lo, hi int) (int, error) {
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return 0, ErrBadNumber
	}
	if n < lo || n > hi {
		return 0, fmt.Errorf("%d %w (%d to %d)", n, ErrOutOfRange, lo, hi)
	}
	return n, nil
}

func parseCPU(v string, machine int64) (int64, error) {
	if v == "" {
		if machine > 0 {
			return machine, nil
		}
		return MillisPerCPU, nil
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, ErrBadNumber
	}
	millis := int64(math.Round(f * MillisPerCPU))
	if millis < 100 || millis > 64*MillisPerCPU {
		return 0, fmt.Errorf("%s %w (0.1 to 64 CPUs)", v, ErrOutOfRange)
	}
	return millis, nil
}

func ParseSize(v string) (int64, error) {
	s := strings.TrimSpace(v)
	factor := int64(1)
	for _, u := range sizeUnits {
		if strings.HasSuffix(s, u.suffix) {
			s, factor = strings.TrimSpace(strings.TrimSuffix(s, u.suffix)), u.factor
			break
		}
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 0 || n > math.MaxInt64/factor {
		return 0, ErrBadSize
	}
	return n * factor, nil
}

func parseBytesRange(v string, def, lo, hi int64) (int64, error) {
	if v == "" {
		return def, nil
	}
	n, err := ParseSize(v)
	if err != nil {
		return 0, err
	}
	if n < lo || n > hi {
		return 0, fmt.Errorf("%s %w (%d to %d bytes)", v, ErrOutOfRange, lo, hi)
	}
	return n, nil
}

func parseDurationRange(v string, def, lo, hi time.Duration) (time.Duration, error) {
	if v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(strings.TrimSpace(v))
	if err != nil {
		return 0, fmt.Errorf("%q is not a duration like 30s or 6h: %w", v, err)
	}
	if d < lo || d > hi {
		return 0, fmt.Errorf("%s %w (%s to %s)", d, ErrOutOfRange, lo, hi)
	}
	return d, nil
}

func parseBool(v string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "false", "0", "no":
		return false, nil
	case "true", "1", "yes":
		return true, nil
	default:
		return false, ErrBadBool
	}
}

func parseLevel(v string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, ErrBadLevel
	}
}

func parsePrefixes(v string) ([]netip.Prefix, error) {
	if strings.TrimSpace(v) == "" {
		return nil, nil
	}
	var out []netip.Prefix
	for _, part := range strings.Split(v, ",") {
		p, err := netip.ParsePrefix(strings.TrimSpace(part))
		if err != nil {
			return nil, fmt.Errorf("%q %w", part, ErrBadPrefix)
		}
		out = append(out, p.Masked())
	}
	return out, nil
}
