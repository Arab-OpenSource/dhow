package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
	appcrypto "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Crypto"
)

const bearerScheme = "Bearer "

type APITokens struct {
	key []byte
}

func NewAPITokens(secret []byte) (*APITokens, error) {
	key, err := appcrypto.DeriveKey(secret, config.APITokenKeyLabel)
	if err != nil {
		return nil, fmt.Errorf("derive api token key: %w", err)
	}
	return &APITokens{key: key}, nil
}

func (t *APITokens) mac(raw []byte) []byte {
	m := hmac.New(sha256.New, t.key)
	m.Write(raw)
	return m.Sum(nil)
}

func (t *APITokens) New() (string, []byte, error) {
	raw := make([]byte, config.APITokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("read random bytes: %w", err)
	}
	return config.APITokenPrefix + base64.RawURLEncoding.EncodeToString(raw), t.mac(raw), nil
}

func (t *APITokens) Hash(token string) ([]byte, bool) {
	rest, ok := strings.CutPrefix(token, config.APITokenPrefix)
	if !ok {
		return nil, false
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(rest)
	if err != nil || len(raw) != config.APITokenBytes {
		return nil, false
	}
	return t.mac(raw), true
}

func Authorization(r *http.Request) (string, bool) {
	values := r.Header.Values("Authorization")
	if len(values) == 0 {
		return "", false
	}
	if len(values) != 1 {
		return "", true
	}
	v := values[0]
	if len(v) <= len(bearerScheme) || !strings.EqualFold(v[:len(bearerScheme)], bearerScheme) {
		return "", true
	}
	return strings.TrimSpace(v[len(bearerScheme):]), true
}
