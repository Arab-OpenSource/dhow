package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

func NewSessionToken(n int) (string, []byte, error) {
	raw := make([]byte, n)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("read random bytes: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), HashBytes(raw), nil
}

func HashBytes(b []byte) []byte {
	sum := sha256.Sum256(b)
	return sum[:]
}

func HashSessionToken(token string, n int) ([]byte, bool) {
	raw, err := base64.RawURLEncoding.Strict().DecodeString(token)
	if err != nil || len(raw) != n {
		return nil, false
	}
	return HashBytes(raw), true
}
