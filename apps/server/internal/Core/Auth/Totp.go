package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // RFC 6238 TOTP with SHA-1 is what every authenticator app supports; HMAC-SHA-1 is not affected by SHA-1 collisions.
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	TotpDigits      = 6
	TotpPeriod      = 30 * time.Second
	TotpSecretBytes = 20
	TotpSkewSteps   = 1
	NoTotpStep      = int64(-1)
)

var totpEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

func NewTotpSecret() ([]byte, error) {
	secret := make([]byte, TotpSecretBytes)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("read random bytes: %w", err)
	}
	return secret, nil
}

func HOTP(secret []byte, counter uint64, digits int) string {
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], counter)
	mac := hmac.New(sha1.New, secret)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	value := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	mod := uint32(1)
	for range digits {
		mod *= 10
	}
	return fmt.Sprintf("%0*d", digits, value%mod)
}

func TotpStep(t time.Time) int64 {
	return t.Unix() / int64(TotpPeriod/time.Second)
}

func TotpCodeAt(secret []byte, step int64) string {
	if step < 0 {
		return ""
	}
	return HOTP(secret, uint64(step), TotpDigits)
}

func NormalizeTotpCode(code string) (string, bool) {
	code = strings.Join(strings.Fields(code), "")
	if len(code) != TotpDigits {
		return "", false
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return "", false
		}
	}
	return code, true
}

func MatchTotp(secret []byte, code string, now time.Time, lastStep int64) (int64, bool) {
	normalized, ok := NormalizeTotpCode(code)
	if !ok {
		return 0, false
	}
	current := TotpStep(now)
	matched, found := int64(0), 0
	for step := current - TotpSkewSteps; step <= current+TotpSkewSteps; step++ {
		equal := subtle.ConstantTimeCompare([]byte(TotpCodeAt(secret, step)), []byte(normalized))
		if equal == 1 && step > lastStep && (found == 0 || step > matched) {
			matched, found = step, 1
		}
	}
	return matched, found == 1
}

func TotpSecretText(secret []byte) string {
	return totpEncoding.EncodeToString(secret)
}

func TotpURI(issuer, account string, secret []byte) string {
	q := url.Values{}
	q.Set("secret", TotpSecretText(secret))
	q.Set("issuer", issuer)
	q.Set("algorithm", "SHA1")
	q.Set("digits", fmt.Sprint(TotpDigits))
	q.Set("period", fmt.Sprint(int(TotpPeriod/time.Second)))
	label := url.PathEscape(issuer) + ":" + url.PathEscape(account)
	return "otpauth://totp/" + label + "?" + q.Encode()
}
