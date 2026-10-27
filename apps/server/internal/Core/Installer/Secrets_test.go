package installer

import (
	"encoding/base64"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerifyKeyFingerprint(t *testing.T) {
	apt, err := os.ReadFile("testdata/pgdg-apt.asc")
	require.NoError(t, err)
	rpm, err := os.ReadFile("testdata/pgdg-rpm.asc")
	require.NoError(t, err)
	tests := []struct {
		name    string
		key     []byte
		finger  string
		wantErr error
	}{
		{"apt key", apt, AptKeyFinger, nil},
		{"rpm key", rpm, DnfKeyFinger, nil},
		{"lower-case fingerprint", apt, strings.ToLower(AptKeyFinger), nil},
		{"wrong key for the fingerprint", rpm, AptKeyFinger, ErrKeyFingerprint},
		{"two keys in one file", append(append([]byte{}, apt...), rpm...), AptKeyFinger, ErrArmor},
		{"not armored", []byte("hello"), AptKeyFinger, ErrArmor},
		{"broken base64", []byte("-----BEGIN PGP PUBLIC KEY BLOCK-----\n\n!!!!\n-----END PGP PUBLIC KEY BLOCK-----\n"), AptKeyFinger, ErrArmor},
		{"truncated packet", []byte("-----BEGIN PGP PUBLIC KEY BLOCK-----\n\n" + base64.StdEncoding.EncodeToString([]byte{0x99, 0x01, 0x8d, 0x04}) + "\n-----END PGP PUBLIC KEY BLOCK-----\n"), AptKeyFinger, ErrArmor},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifyKeyFingerprint(tt.key, tt.finger)
			if tt.wantErr == nil {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestScramVerifierMatchesAnIndependentImplementation(t *testing.T) {
	salt, err := base64.StdEncoding.DecodeString("W22ZaJ0SNY7soEsUEjb6gQ==")
	require.NoError(t, err)
	got, err := scramVerifierWithSalt("pencil", salt)
	require.NoError(t, err)
	assert.Equal(t, "SCRAM-SHA-256$4096:W22ZaJ0SNY7soEsUEjb6gQ==$WG5d8oPm3OtcPnkdi4Uo7BkeZkBFzpcXkuLmtbsT4qY=:wfPLwcE6nTWhTAmQ7tl2KeoiWGPlZqQxSrmfPwDl2dU=", got)
	a, err := ScramVerifier("pencil")
	require.NoError(t, err)
	b, err := ScramVerifier("pencil")
	require.NoError(t, err)
	assert.NotEqual(t, a, b, "each verifier gets a fresh salt")
	assert.NotContains(t, a, "pencil")
}

func TestRandomHex(t *testing.T) {
	a, err := RandomHex(SecretKeyBytes)
	require.NoError(t, err)
	b, err := RandomHex(SecretKeyBytes)
	require.NoError(t, err)
	assert.Len(t, a, 2*SecretKeyBytes)
	assert.NotEqual(t, a, b)
}

func TestLocalDatabaseURL(t *testing.T) {
	u := LocalDatabaseURL("abc123")
	assert.Equal(t, "postgres://dhow:abc123@/dhow?host=/var/run/postgresql&port=5432&sslmode=require", u)
	assert.Equal(t, "abc123", passwordFromURL(u+"\n"))
	assert.Empty(t, passwordFromURL("postgres://db/dhow"))
}
