package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
)

const csrfKeyLabel = "dhow/csrf/v1"

type CSRF struct {
	key []byte
}

func NewCSRF(secret []byte) *CSRF {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(csrfKeyLabel))
	return &CSRF{key: mac.Sum(nil)}
}

func (c *CSRF) Token(sessionToken string) string {
	mac := hmac.New(sha256.New, c.key)
	mac.Write([]byte(sessionToken))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (c *CSRF) Hash(token string) []byte {
	return HashBytes([]byte(token))
}

func (c *CSRF) Verify(presented string, wantHash []byte) bool {
	if presented == "" {
		return false
	}
	return subtle.ConstantTimeCompare(c.Hash(presented), wantHash) == 1
}
