package runnerauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
)

const bearerPrefix = "Bearer "

func newToken(prefix string) (string, []byte, error) {
	raw := make([]byte, config.RunnerTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("read random bytes: %w", err)
	}
	return prefix + base64.RawURLEncoding.EncodeToString(raw), raw, nil
}

func parseToken(prefix, token string) ([]byte, bool) {
	rest, ok := strings.CutPrefix(token, prefix)
	if !ok {
		return nil, false
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(rest)
	if err != nil || len(raw) != config.RunnerTokenBytes {
		return nil, false
	}
	return raw, true
}

func NewRegistrationToken() (string, []byte, error) {
	return newToken(config.RunnerRegistrationTokenPrefix)
}

func ParseRegistrationToken(token string) ([]byte, bool) {
	return parseToken(config.RunnerRegistrationTokenPrefix, token)
}

func NewAccessToken() (string, []byte, error) {
	token, raw, err := newToken(config.RunnerAccessTokenPrefix)
	if err != nil {
		return "", nil, err
	}
	sum := sha256.Sum256(raw)
	return token, sum[:], nil
}

func HashAccessToken(token string) ([]byte, bool) {
	raw, ok := parseToken(config.RunnerAccessTokenPrefix, token)
	if !ok {
		return nil, false
	}
	sum := sha256.Sum256(raw)
	return sum[:], true
}

func BearerToken(h http.Header) string {
	v := h.Get("Authorization")
	if len(v) <= len(bearerPrefix) || !strings.EqualFold(v[:len(bearerPrefix)], bearerPrefix) {
		return ""
	}
	return strings.TrimSpace(v[len(bearerPrefix):])
}
