// Package config loads and validates the control plane configuration from environment variables.
//
// Loading fails closed: a missing, placeholder or weak secret, a plain-HTTP public URL or an unencrypted
// database connection stops the process at boot unless DHOW_DEV=true. Error messages never echo secret values.
package config

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
)

// Environment variable names.
const (
	EnvDatabaseURL    = "DHOW_DATABASE_URL"
	EnvSecretKey      = "DHOW_SECRET_KEY"
	EnvPublicURL      = "DHOW_PUBLIC_URL"
	EnvListenAddr     = "DHOW_LISTEN_ADDR"
	EnvTrustedProxies = "DHOW_TRUSTED_PROXIES"
	EnvLogLevel       = "DHOW_LOG_LEVEL"
	EnvDev            = "DHOW_DEV"
)

// DefaultListenAddr binds to loopback so the server is only reachable through a reverse proxy by default.
const DefaultListenAddr = "127.0.0.1:8080"

// MinSecretKeyBytes is the minimum decoded length of DHOW_SECRET_KEY.
const MinSecretKeyBytes = 32

// Config is the validated configuration.
type Config struct {
	DatabaseURL    string
	SecretKey      []byte
	PublicURL      *url.URL
	ListenAddr     string
	TrustedProxies []netip.Prefix
	LogLevel       slog.Level
	Dev            bool
}

// Load reads the configuration through getenv (usually os.Getenv) and validates every field.
// All problems are reported together.
func Load(getenv func(string) string) (Config, error) {
	var cfg Config
	var errs []error

	dev, err := parseBool(getenv(EnvDev))
	if err != nil {
		errs = append(errs, fmt.Errorf("%s: %w", EnvDev, err))
	}
	cfg.Dev = dev

	if cfg.SecretKey, err = parseSecretKey(getenv(EnvSecretKey)); err != nil {
		errs = append(errs, fmt.Errorf("%s: %w", EnvSecretKey, err))
	}
	if cfg.DatabaseURL, err = parseDatabaseURL(getenv(EnvDatabaseURL), dev); err != nil {
		errs = append(errs, fmt.Errorf("%s: %w", EnvDatabaseURL, err))
	}
	if cfg.PublicURL, err = parsePublicURL(getenv(EnvPublicURL), dev); err != nil {
		errs = append(errs, fmt.Errorf("%s: %w", EnvPublicURL, err))
	}
	if cfg.ListenAddr, err = parseListenAddr(getenv(EnvListenAddr)); err != nil {
		errs = append(errs, fmt.Errorf("%s: %w", EnvListenAddr, err))
	}
	if cfg.TrustedProxies, err = parseTrustedProxies(getenv(EnvTrustedProxies)); err != nil {
		errs = append(errs, fmt.Errorf("%s: %w", EnvTrustedProxies, err))
	}
	if cfg.LogLevel, err = parseLogLevel(getenv(EnvLogLevel)); err != nil {
		errs = append(errs, fmt.Errorf("%s: %w", EnvLogLevel, err))
	}

	if len(errs) > 0 {
		return Config{}, errors.Join(errs...)
	}
	return cfg, nil
}

func parseBool(v string) (bool, error) {
	if v == "" {
		return false, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, errors.New("must be true or false")
	}
	return b, nil
}

// Words that appear in copied example values. All are five letters or longer, so a random
// base64 key is practically never rejected by accident; hex keys cannot contain them at all.
var placeholderMarkers = []string{
	"change", "replace", "example", "placeholder", "secret", "password", "default", "dummy",
}

func parseSecretKey(v string) ([]byte, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil, fmt.Errorf("is required; generate one with `openssl rand -hex %d`", MinSecretKeyBytes)
	}
	lower := strings.ToLower(v)
	for _, m := range placeholderMarkers {
		if strings.Contains(lower, m) {
			return nil, errors.New("looks like a placeholder; generate a random key")
		}
	}
	key, err := decodeKey(v)
	if err != nil {
		return nil, err
	}
	if len(key) < MinSecretKeyBytes {
		return nil, fmt.Errorf("must decode to at least %d bytes", MinSecretKeyBytes)
	}
	if !looksRandom(key) {
		return nil, errors.New("has too little variety to be random; generate a random key")
	}
	return key, nil
}

// decodeKey accepts hex first (unambiguous), then standard or URL-safe base64 with or without padding.
func decodeKey(v string) ([]byte, error) {
	if b, err := hex.DecodeString(v); err == nil {
		return b, nil
	}
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if b, err := enc.DecodeString(v); err == nil {
			return b, nil
		}
	}
	return nil, errors.New("must be hex or base64")
}

// looksRandom rejects keys with very few distinct bytes (for example a repeated character).
// 32 random bytes have about 30 distinct values; 16 is a conservative floor.
func looksRandom(key []byte) bool {
	seen := make(map[byte]struct{}, len(key))
	for _, b := range key {
		seen[b] = struct{}{}
	}
	return len(seen) >= 16
}

var secureSSLModes = map[string]bool{"require": true, "verify-ca": true, "verify-full": true}

func parseDatabaseURL(v string, dev bool) (string, error) {
	if v == "" {
		return "", errors.New("is required")
	}
	u, err := url.Parse(v)
	// Never wrap the parse error: it can contain the password.
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		return "", errors.New("must be a postgres:// URL")
	}
	if u.Host == "" && u.Query().Get("host") == "" {
		return "", errors.New("must name a host")
	}
	if !dev && !secureSSLModes[u.Query().Get("sslmode")] {
		return "", errors.New("must set sslmode=require, verify-ca or verify-full (or DHOW_DEV=true for local development)")
	}
	return v, nil
}

func parsePublicURL(v string, dev bool) (*url.URL, error) {
	if v == "" {
		return nil, errors.New("is required, for example https://dhow.example.com")
	}
	u, err := url.Parse(v)
	if err != nil || u.Host == "" || !u.IsAbs() {
		return nil, errors.New("must be an absolute URL")
	}
	switch {
	case u.Scheme == "https":
	case u.Scheme == "http" && dev:
	default:
		return nil, errors.New("must use https (http is allowed only with DHOW_DEV=true)")
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("must not contain credentials, a query or a fragment")
	}
	u.Path = strings.TrimSuffix(u.Path, "/")
	return u, nil
}

func parseListenAddr(v string) (string, error) {
	if v == "" {
		return DefaultListenAddr, nil
	}
	if _, port, err := net.SplitHostPort(v); err != nil || port == "" {
		return "", errors.New("must be host:port")
	}
	return v, nil
}

func parseTrustedProxies(v string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, part := range strings.Split(v, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "/") {
			p, err := netip.ParsePrefix(part)
			if err != nil {
				return nil, fmt.Errorf("invalid CIDR %q", part)
			}
			out = append(out, p.Masked())
			continue
		}
		a, err := netip.ParseAddr(part)
		if err != nil {
			return nil, fmt.Errorf("invalid IP %q", part)
		}
		out = append(out, netip.PrefixFrom(a.Unmap(), a.Unmap().BitLen()))
	}
	return out, nil
}

func parseLogLevel(v string) (slog.Level, error) {
	switch strings.ToLower(v) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, errors.New("must be debug, info, warn or error")
	}
}
