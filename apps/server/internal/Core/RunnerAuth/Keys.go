package runnerauth

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"fmt"

	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
	appcrypto "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Crypto"
)

type Keys struct {
	signing      ed25519.PrivateKey
	registration []byte
	challenge    []byte
}

func DeriveKeys(secret []byte) (*Keys, error) {
	seed, err := appcrypto.DeriveKey(secret, config.RunnerSigningKeyLabel)
	if err != nil {
		return nil, fmt.Errorf("derive runner plane signing key: %w", err)
	}
	registration, err := appcrypto.DeriveKey(secret, config.RunnerRegistrationTokenLabel)
	if err != nil {
		return nil, fmt.Errorf("derive runner registration token key: %w", err)
	}
	challenge, err := appcrypto.DeriveKey(secret, config.RunnerChallengeKeyLabel)
	if err != nil {
		return nil, fmt.Errorf("derive runner challenge key: %w", err)
	}
	return &Keys{signing: ed25519.NewKeyFromSeed(seed), registration: registration, challenge: challenge}, nil
}

func (k *Keys) SigningPublicKey() ed25519.PublicKey {
	pub, ok := k.signing.Public().(ed25519.PublicKey)
	if !ok {
		return nil
	}
	return append(ed25519.PublicKey(nil), pub...)
}

func (k *Keys) SigningKeyVersion() int32 { return config.RunnerServerKeyVersion }

func (k *Keys) Sign(message []byte) []byte { return ed25519.Sign(k.signing, message) }

func (k *Keys) RegistrationTokenHash(raw []byte) []byte { return mac(k.registration, raw) }

func (k *Keys) ChallengeMAC(challenge []byte) []byte { return mac(k.challenge, challenge) }

func (k *Keys) VerifyChallengeMAC(challenge, presented []byte) bool {
	return len(presented) == sha256.Size && hmac.Equal(k.ChallengeMAC(challenge), presented)
}

func mac(key, data []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(data)
	return m.Sum(nil)
}
