package appcrypto_test

import (
	"bytes"
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appcrypto "github.com/Khaymah/dhow/apps/server/internal/Core/Crypto"
)

func secret(t *testing.T) []byte {
	t.Helper()
	b := make([]byte, 32)
	_, err := rand.Read(b)
	require.NoError(t, err)
	return b
}

func TestSealRoundTripsAndUsesAFreshNonceEachTime(t *testing.T) {
	s, err := appcrypto.NewSealer(secret(t), "dhow/test/v1")
	require.NoError(t, err)
	plain := []byte("twenty bytes secret!")
	a, err := s.Seal(plain, []byte("user-1"))
	require.NoError(t, err)
	b, err := s.Seal(plain, []byte("user-1"))
	require.NoError(t, err)
	assert.NotEqual(t, a, b, "the same plaintext never seals to the same bytes")
	assert.False(t, bytes.Contains(a, plain), "the plaintext never appears in the sealed value")
	assert.Len(t, a, 1+12+len(plain)+16)
	got, err := s.Open(a, []byte("user-1"))
	require.NoError(t, err)
	assert.Equal(t, plain, got)
}

func TestOpenRejectsAnotherContextKeyOrATamperedValue(t *testing.T) {
	key := secret(t)
	s, err := appcrypto.NewSealer(key, "dhow/test/v1")
	require.NoError(t, err)
	sealed, err := s.Seal([]byte("secret"), []byte("user-1"))
	require.NoError(t, err)

	_, err = s.Open(sealed, []byte("user-2"))
	require.ErrorIs(t, err, appcrypto.ErrSealed, "a value sealed for one record does not open for another")

	other, err := appcrypto.NewSealer(key, "dhow/other/v1")
	require.NoError(t, err)
	_, err = other.Open(sealed, []byte("user-1"))
	require.ErrorIs(t, err, appcrypto.ErrSealed, "keys derived for another purpose do not open it")

	tampered := append([]byte(nil), sealed...)
	tampered[len(tampered)-1] ^= 1
	_, err = s.Open(tampered, []byte("user-1"))
	require.ErrorIs(t, err, appcrypto.ErrSealed)

	versioned := append([]byte(nil), sealed...)
	versioned[0] = 9
	_, err = s.Open(versioned, []byte("user-1"))
	require.ErrorIs(t, err, appcrypto.ErrSealed)

	_, err = s.Open(sealed[:10], []byte("user-1"))
	require.ErrorIs(t, err, appcrypto.ErrSealed)
}

func TestDeriveKeyNeedsALongSecretAndSeparatesLabels(t *testing.T) {
	_, err := appcrypto.DeriveKey(make([]byte, 16), "x")
	require.ErrorIs(t, err, appcrypto.ErrShortSecret)
	key := secret(t)
	a, err := appcrypto.DeriveKey(key, "a")
	require.NoError(t, err)
	b, err := appcrypto.DeriveKey(key, "b")
	require.NoError(t, err)
	assert.Len(t, a, appcrypto.KeyBytes)
	assert.NotEqual(t, a, b)
	again, err := appcrypto.DeriveKey(key, "a")
	require.NoError(t, err)
	assert.Equal(t, a, again)
}
