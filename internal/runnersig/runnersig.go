package runnersig

import (
	"crypto/ed25519"
	"crypto/hpke"
	"encoding/binary"
	"errors"
)

const (
	RegisterLabel = "dhow/runner-register/v1"
	AuthLabel     = "dhow/runner-auth/v1"
	RotateLabel   = "dhow/runner-rotate-keys/v1"

	SigningKeySize    = ed25519.PublicKeySize
	EncryptionKeySize = 1216
	NonceSize         = 16
)

var (
	ErrSigningKey    = errors.New("signing public key must be a 32-byte Ed25519 key")
	ErrEncryptionKey = errors.New("encryption public key must be an MLKEM768-X25519 (X-Wing) key")
)

type builder struct{ buf []byte }

func newBuilder(label string) *builder {
	b := &builder{buf: make([]byte, 0, 2048)}
	b.bytes([]byte(label))
	return b
}

func (b *builder) bytes(v []byte) {
	b.buf = binary.BigEndian.AppendUint32(b.buf, uint32(len(v))) //nolint:gosec // Every field is bounded far below 4 GiB by the request limits.
	b.buf = append(b.buf, v...)
}

func (b *builder) int32(v int32) {
	b.buf = binary.BigEndian.AppendUint32(b.buf, uint32(v)) //nolint:gosec // A key version is a small positive number.
}

func RegisterMessage(origin string, signingKey, encryptionKey []byte) []byte {
	b := newBuilder(RegisterLabel)
	b.bytes([]byte(origin))
	b.bytes(signingKey)
	b.bytes(encryptionKey)
	return b.buf
}

func AuthMessage(origin string, serverKeyVersion int32, challenge []byte) []byte {
	b := newBuilder(AuthLabel)
	b.bytes([]byte(origin))
	b.int32(serverKeyVersion)
	b.bytes(challenge)
	return b.buf
}

func RotateMessage(origin, runnerID string, keyVersion int32, newSigningKey, newEncryptionKey []byte) []byte {
	b := newBuilder(RotateLabel)
	b.bytes([]byte(origin))
	b.bytes([]byte(runnerID))
	b.int32(keyVersion)
	b.bytes(newSigningKey)
	b.bytes(newEncryptionKey)
	return b.buf
}

func ParseSigningKey(v []byte) (ed25519.PublicKey, error) {
	if len(v) != SigningKeySize {
		return nil, ErrSigningKey
	}
	return ed25519.PublicKey(append([]byte(nil), v...)), nil
}

func ParseEncryptionKey(v []byte) (hpke.PublicKey, error) {
	if len(v) != EncryptionKeySize {
		return nil, ErrEncryptionKey
	}
	pk, err := hpke.MLKEM768X25519().NewPublicKey(v)
	if err != nil {
		return nil, errors.Join(ErrEncryptionKey, err)
	}
	return pk, nil
}

func Verify(key ed25519.PublicKey, message, signature []byte) bool {
	return len(key) == SigningKeySize && len(signature) == ed25519.SignatureSize && ed25519.Verify(key, message, signature)
}
