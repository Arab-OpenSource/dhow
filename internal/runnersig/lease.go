package runnersig

import (
	"crypto/hpke"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
)

const (
	GrantLabel      = "dhow/lease-grant/v1"
	StartLabel      = "dhow/lease-start/v1"
	CompletionLabel = "dhow/attempt-completion/v1"
	SecretsInfo     = "dhow/secrets/v1"

	DigestSize = sha256.Size
)

var ErrSecretsOpen = errors.New("the sealed secrets do not open with this key and lease")

func (b *builder) int64(v int64) {
	b.buf = binary.BigEndian.AppendUint64(b.buf, uint64(v)) //nolint:gosec // A fencing token is a positive sequence value.
}

func GrantMessage(grantBytes []byte) []byte {
	b := newBuilder(GrantLabel)
	b.bytes(grantBytes)
	return b.buf
}

func GrantDigest(grantBytes []byte) []byte {
	sum := sha256.Sum256(grantBytes)
	return sum[:]
}

func StartMessage(leaseID string, fencingToken int64, grantDigest []byte) []byte {
	b := newBuilder(StartLabel)
	b.bytes([]byte(leaseID))
	b.int64(fencingToken)
	b.bytes(grantDigest)
	return b.buf
}

func CompletionDigest(attemptID string, fencingToken int64, result, exitCode, failureReason int32) []byte {
	b := newBuilder(CompletionLabel)
	b.bytes([]byte(attemptID))
	b.int64(fencingToken)
	b.int32(result)
	b.int32(exitCode)
	b.int32(failureReason)
	sum := sha256.Sum256(b.buf)
	return sum[:]
}

func SecretsAAD(leaseID string, fencingToken int64, attemptID, runnerID string) []byte {
	b := newBuilder(SecretsInfo)
	b.bytes([]byte(leaseID))
	b.int64(fencingToken)
	b.bytes([]byte(attemptID))
	b.bytes([]byte(runnerID))
	return b.buf
}

func SealSecrets(encryptionKey, aad, plaintext []byte) ([]byte, []byte, error) {
	pk, err := ParseEncryptionKey(encryptionKey)
	if err != nil {
		return nil, nil, err
	}
	enc, sender, err := hpke.NewSender(pk, hpke.HKDFSHA256(), hpke.AES256GCM(), []byte(SecretsInfo))
	if err != nil {
		return nil, nil, fmt.Errorf("start hpke sender: %w", err)
	}
	ciphertext, err := sender.Seal(aad, plaintext)
	if err != nil {
		return nil, nil, fmt.Errorf("seal secrets: %w", err)
	}
	return enc, ciphertext, nil
}

func OpenSecrets(key hpke.PrivateKey, enc, aad, ciphertext []byte) ([]byte, error) {
	recipient, err := hpke.NewRecipient(enc, key, hpke.HKDFSHA256(), hpke.AES256GCM(), []byte(SecretsInfo))
	if err != nil {
		return nil, errors.Join(ErrSecretsOpen, err)
	}
	plaintext, err := recipient.Open(aad, ciphertext)
	if err != nil {
		return nil, errors.Join(ErrSecretsOpen, err)
	}
	return plaintext, nil
}
