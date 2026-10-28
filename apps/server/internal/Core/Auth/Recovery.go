package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"fmt"
	"strings"

	appcrypto "github.com/Khaymah/dhow/apps/server/internal/Core/Crypto"
)

const (
	RecoveryCodeCount = 10
	recoveryCodeBytes = 10
	recoveryCodeChars = 16
	recoveryGroup     = 4
	recoveryKeyLabel  = "dhow/recovery-code/v1"
	deviceLabel       = "dhow/mfa-device/v1\x00"
)

var recoveryEncoding = base32.NewEncoding("0123456789ABCDEFGHJKMNPQRSTVWXYZ").WithPadding(base32.NoPadding)

var recoveryAliases = strings.NewReplacer("O", "0", "I", "1", "L", "1", "-", "", " ", "")

func NewRecoveryCodes(n int) ([]string, error) {
	out := make([]string, 0, n)
	raw := make([]byte, recoveryCodeBytes)
	for range n {
		if _, err := rand.Read(raw); err != nil {
			return nil, fmt.Errorf("read random bytes: %w", err)
		}
		text := recoveryEncoding.EncodeToString(raw)
		groups := make([]string, 0, recoveryCodeChars/recoveryGroup)
		for i := 0; i < len(text); i += recoveryGroup {
			groups = append(groups, text[i:i+recoveryGroup])
		}
		out = append(out, strings.Join(groups, "-"))
	}
	return out, nil
}

func NormalizeRecoveryCode(code string) (string, bool) {
	if len(code) > 64 {
		return "", false
	}
	normalized := recoveryAliases.Replace(strings.ToUpper(strings.TrimSpace(code)))
	if len(normalized) != recoveryCodeChars {
		return "", false
	}
	if _, err := recoveryEncoding.DecodeString(normalized); err != nil {
		return "", false
	}
	return normalized, true
}

type RecoveryHasher struct {
	key []byte
}

func NewRecoveryHasher(secret []byte) (*RecoveryHasher, error) {
	key, err := appcrypto.DeriveKey(secret, recoveryKeyLabel)
	if err != nil {
		return nil, err
	}
	return &RecoveryHasher{key: key}, nil
}

func (h *RecoveryHasher) Hash(normalized string) []byte {
	mac := hmac.New(sha256.New, h.key)
	mac.Write([]byte(normalized))
	return mac.Sum(nil)
}

func DeviceHash(userAgent string) []byte {
	sum := sha256.Sum256([]byte(deviceLabel + userAgent))
	return sum[:]
}
