package auth_test

import (
	"crypto/rand"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	auth "github.com/Khaymah/dhow/apps/server/internal/Core/Auth"
)

var recoveryFormat = regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{4}(-[0-9A-HJKMNP-TV-Z]{4}){3}$`)

func TestRecoveryCodesCarryEightyRandomBitsEach(t *testing.T) {
	codes, err := auth.NewRecoveryCodes(auth.RecoveryCodeCount)
	require.NoError(t, err)
	require.Len(t, codes, 10)
	seen := map[string]bool{}
	for _, c := range codes {
		assert.Regexp(t, recoveryFormat, c)
		assert.False(t, seen[c], "codes are unique")
		seen[c] = true
		normalized, ok := auth.NormalizeRecoveryCode(c)
		require.True(t, ok)
		assert.Len(t, normalized, 16, "16 base32 characters are 80 bits")
	}
}

func TestNormalizeRecoveryCodeForgivesCaseDashesAndLookalikes(t *testing.T) {
	want, ok := auth.NormalizeRecoveryCode("7KQ2-M9XD-P4TR-82WN")
	require.True(t, ok)
	for _, in := range []string{"7kq2m9xdp4tr82wn", " 7KQ2 M9XD P4TR 82WN ", "7KQ2-M9XD-P4TR-82WN"} {
		got, ok := auth.NormalizeRecoveryCode(in)
		require.True(t, ok, in)
		assert.Equal(t, want, got, in)
	}
	a, _ := auth.NormalizeRecoveryCode("O0IL-0000-0000-0000")
	b, _ := auth.NormalizeRecoveryCode("0011-0000-0000-0000")
	assert.Equal(t, b, a, "O reads as 0, and I and L read as 1")
	for _, in := range []string{"", "7KQ2-M9XD-P4TR", "7KQ2-M9XD-P4TR-82WN-AAAA", "7KQ2-M9XD-P4TR-82W!", "UUUU-UUUU-UUUU-UUUU"} {
		_, ok := auth.NormalizeRecoveryCode(in)
		assert.False(t, ok, in)
	}
}

func TestRecoveryHasherIsKeyed(t *testing.T) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	h, err := auth.NewRecoveryHasher(key)
	require.NoError(t, err)
	code, _ := auth.NormalizeRecoveryCode("7KQ2-M9XD-P4TR-82WN")
	assert.Len(t, h.Hash(code), 32)
	assert.Equal(t, h.Hash(code), h.Hash(code))
	_, err = rand.Read(key)
	require.NoError(t, err)
	other, err := auth.NewRecoveryHasher(key)
	require.NoError(t, err)
	assert.NotEqual(t, h.Hash(code), other.Hash(code), "the hash depends on DHOW_SECRET_KEY")
}

func TestDeviceHashDependsOnTheUserAgent(t *testing.T) {
	assert.Len(t, auth.DeviceHash("a"), 32)
	assert.Equal(t, auth.DeviceHash("a"), auth.DeviceHash("a"))
	assert.NotEqual(t, auth.DeviceHash("a"), auth.DeviceHash("b"))
}
