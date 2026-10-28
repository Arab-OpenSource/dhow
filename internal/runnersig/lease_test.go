package runnersig_test

import (
	"crypto/hpke"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Arab-OpenSource/dhow/internal/runnersig"
)

func TestLeaseMessagesAreDistinct(t *testing.T) {
	digest := runnersig.GrantDigest([]byte("grant"))
	require.Len(t, digest, runnersig.DigestSize)
	msgs := [][]byte{
		runnersig.GrantMessage([]byte("grant")),
		runnersig.GrantMessage([]byte("grant2")),
		runnersig.StartMessage("lease-1", 1, digest),
		runnersig.StartMessage("lease-1", 2, digest),
		runnersig.StartMessage("lease-2", 1, digest),
		runnersig.SecretsAAD("lease-1", 1, "attempt-1", "runner-1"),
		runnersig.SecretsAAD("lease-1", 1, "attempt-1", "runner-2"),
		runnersig.SecretsAAD("lease-1", 2, "attempt-1", "runner-1"),
		runnersig.CompletionDigest("attempt-1", 1, 1, 0, 0),
		runnersig.CompletionDigest("attempt-1", 1, 2, 1, 1),
		runnersig.CompletionDigest("attempt-1", 2, 1, 0, 0),
	}
	for i := range msgs {
		for j := range msgs {
			if i != j {
				assert.NotEqual(t, msgs[i], msgs[j], "%d and %d", i, j)
			}
		}
	}
	assert.Equal(t, runnersig.CompletionDigest("a", 7, 1, 0, 0), runnersig.CompletionDigest("a", 7, 1, 0, 0))
}

func TestSealedSecretsOpenOnlyForTheRightKeyAndLease(t *testing.T) {
	key, err := hpke.MLKEM768X25519().GenerateKey()
	require.NoError(t, err)
	other, err := hpke.MLKEM768X25519().GenerateKey()
	require.NoError(t, err)
	aad := runnersig.SecretsAAD("lease-1", 9, "attempt-1", "runner-1")
	enc, ct, err := runnersig.SealSecrets(key.PublicKey().Bytes(), aad, []byte("s3cr3t-value"))
	require.NoError(t, err)
	assert.NotContains(t, string(ct), "s3cr3t-value")

	plain, err := runnersig.OpenSecrets(key, enc, aad, ct)
	require.NoError(t, err)
	assert.Equal(t, "s3cr3t-value", string(plain))

	tests := []struct {
		name string
		key  hpke.PrivateKey
		aad  []byte
	}{
		{"another runner key", other, aad},
		{"another attempt", key, runnersig.SecretsAAD("lease-1", 9, "attempt-2", "runner-1")},
		{"an older fencing token", key, runnersig.SecretsAAD("lease-1", 8, "attempt-1", "runner-1")},
		{"another runner id", key, runnersig.SecretsAAD("lease-1", 9, "attempt-1", "runner-2")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := runnersig.OpenSecrets(tt.key, enc, tt.aad, ct)
			assert.ErrorIs(t, err, runnersig.ErrSecretsOpen)
		})
	}
	_, _, err = runnersig.SealSecrets(make([]byte, 32), aad, []byte("x"))
	assert.ErrorIs(t, err, runnersig.ErrEncryptionKey)
}
