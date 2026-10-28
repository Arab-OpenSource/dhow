package runnersig_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/hpke"
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Arab-OpenSource/dhow/internal/runnersig"
)

func TestMessagesAreDomainSeparatedAndUnambiguous(t *testing.T) {
	sk, ek := bytes.Repeat([]byte{1}, 32), bytes.Repeat([]byte{2}, runnersig.EncryptionKeySize)
	msgs := [][]byte{
		runnersig.RegisterMessage("https://a.example", sk, ek),
		runnersig.RegisterMessage("https://b.example", sk, ek),
		runnersig.RegisterMessage("https://a.exampl", append([]byte{'e'}, sk...), ek),
		runnersig.AuthMessage("https://a.example", 1, sk),
		runnersig.AuthMessage("https://a.example", 2, sk),
		runnersig.RotateMessage("https://a.example", "r1", 1, sk, ek),
		runnersig.RotateMessage("https://a.example", "r1", 2, sk, ek),
		runnersig.RotateMessage("https://a.example", "r2", 1, sk, ek),
	}
	for i := range msgs {
		for j := range msgs {
			if i != j {
				assert.NotEqual(t, msgs[i], msgs[j], "%d and %d", i, j)
			}
		}
	}
	assert.True(t, bytes.HasPrefix(msgs[0][4:], []byte(runnersig.RegisterLabel)))
}

func TestVerify(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	msg := runnersig.AuthMessage("https://dhow.example.com", 1, []byte("challenge"))
	sig := ed25519.Sign(priv, msg)
	assert.True(t, runnersig.Verify(pub, msg, sig))
	assert.False(t, runnersig.Verify(pub, append(msg, 0), sig))
	assert.False(t, runnersig.Verify(pub, msg, sig[:63]))
	assert.False(t, runnersig.Verify(pub[:31], msg, sig))
	other, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	assert.False(t, runnersig.Verify(other, msg, sig))
}

func TestParseKeys(t *testing.T) {
	_, err := runnersig.ParseSigningKey(make([]byte, 31))
	assert.ErrorIs(t, err, runnersig.ErrSigningKey)
	_, err = runnersig.ParseSigningKey(make([]byte, 32))
	assert.NoError(t, err)

	k, err := hpke.MLKEM768X25519().GenerateKey()
	require.NoError(t, err)
	_, err = runnersig.ParseEncryptionKey(k.PublicKey().Bytes())
	assert.NoError(t, err)
	_, err = runnersig.ParseEncryptionKey(make([]byte, 32))
	assert.ErrorIs(t, err, runnersig.ErrEncryptionKey)
}
