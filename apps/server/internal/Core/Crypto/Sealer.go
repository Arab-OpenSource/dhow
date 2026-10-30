package appcrypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
)

const (
	KeyBytes      = 32
	sealVersion   = byte(1)
	nonceBytes    = 12
	minSecretSize = 32
	MaxPlaintext  = 1 << 20
)

var (
	ErrShortSecret = errors.New("secret is shorter than 32 bytes")
	ErrSealed      = errors.New("sealed value is malformed or was not sealed with this key and context")
	ErrTooLarge    = errors.New("plaintext is larger than 1 MiB")
)

func DeriveKey(secret []byte, label string) ([]byte, error) {
	if len(secret) < minSecretSize {
		return nil, ErrShortSecret
	}
	key, err := hkdf.Key(sha256.New, secret, nil, label, KeyBytes)
	if err != nil {
		return nil, fmt.Errorf("derive %s key: %w", label, err)
	}
	return key, nil
}

type Sealer struct {
	aead cipher.AEAD
}

func NewSealer(secret []byte, label string) (*Sealer, error) {
	key, err := DeriveKey(secret, label)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}
	return &Sealer{aead: aead}, nil
}

func (s *Sealer) Seal(plaintext, context []byte) ([]byte, error) {
	if len(plaintext) > MaxPlaintext {
		return nil, ErrTooLarge
	}
	out := make([]byte, 1+nonceBytes, 1+nonceBytes+len(plaintext)+s.aead.Overhead())
	out[0] = sealVersion
	if _, err := rand.Read(out[1:]); err != nil {
		return nil, fmt.Errorf("read nonce: %w", err)
	}
	return s.aead.Seal(out, out[1:], plaintext, aad(context)), nil
}

func (s *Sealer) Open(sealed, context []byte) ([]byte, error) {
	if len(sealed) < 1+nonceBytes+s.aead.Overhead() || sealed[0] != sealVersion {
		return nil, ErrSealed
	}
	plain, err := s.aead.Open(nil, sealed[1:1+nonceBytes], sealed[1+nonceBytes:], aad(context))
	if err != nil {
		return nil, ErrSealed
	}
	return plain, nil
}

func aad(context []byte) []byte {
	out := make([]byte, 0, 1+len(context))
	out = append(out, sealVersion)
	return append(out, context...)
}
