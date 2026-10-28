package identity

import (
	"crypto/ed25519"
	"crypto/hpke"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/google/uuid"

	config "github.com/Khaymah/dhow/apps/runner/internal/Config"
	"github.com/Khaymah/dhow/internal/runnersig"
)

const fileVersion = 1

var (
	ErrExists        = errors.New("this runner is already registered; remove the state directory to register it again")
	ErrMissing       = config.ErrNotRegistered
	ErrCorrupt       = errors.New("the identity file is damaged")
	ErrServerKey     = errors.New("the server sent an invalid signing key")
	ErrUnsupportedV  = errors.New("the identity file has an unsupported version")
	ErrEncryptionKEM = errors.New("the encryption key is not an MLKEM768-X25519 key")
)

type Keys struct {
	Signing    ed25519.PrivateKey
	Encryption hpke.PrivateKey
}

type Identity struct {
	Keys
	RunnerID         string
	OrganizationID   string
	ServerURL        string
	ServerOrigin     string
	ServerSigningKey ed25519.PublicKey
	ServerKeyVersion int32
	Labels           []string
}

type fileFormat struct {
	Version          int      `json:"Version"`
	RunnerID         string   `json:"RunnerId"`
	OrganizationID   string   `json:"OrganizationId"`
	ServerURL        string   `json:"ServerUrl"`
	ServerOrigin     string   `json:"ServerOrigin"`
	ServerSigningKey []byte   `json:"ServerSigningKey"`
	ServerKeyVersion int32    `json:"ServerKeyVersion"`
	SigningSeed      []byte   `json:"SigningSeed"`
	EncryptionKey    []byte   `json:"EncryptionKey"`
	Labels           []string `json:"Labels"`
}

func GenerateKeys() (Keys, error) {
	_, signing, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return Keys{}, fmt.Errorf("generate the signing key: %w", err)
	}
	enc, err := hpke.MLKEM768X25519().GenerateKey()
	if err != nil {
		return Keys{}, fmt.Errorf("generate the encryption key: %w", err)
	}
	return Keys{Signing: signing, Encryption: enc}, nil
}

func (k Keys) SigningPublicKey() []byte {
	pub, ok := k.Signing.Public().(ed25519.PublicKey)
	if !ok {
		return nil
	}
	return append([]byte(nil), pub...)
}

func (k Keys) EncryptionPublicKey() []byte { return k.Encryption.PublicKey().Bytes() }

func (k Keys) Sign(message []byte) []byte { return ed25519.Sign(k.Signing, message) }

func (k Keys) RegisterProof(origin string) []byte {
	return k.Sign(runnersig.RegisterMessage(origin, k.SigningPublicKey(), k.EncryptionPublicKey()))
}

func (i *Identity) VerifyServer(message, signature []byte) bool {
	return runnersig.Verify(i.ServerSigningKey, message, signature)
}

func Exists(path string) (bool, error) {
	_, err := os.Lstat(path)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, fs.ErrNotExist):
		return false, nil
	default:
		return false, fmt.Errorf("read %s: %w", path, err)
	}
}

func Save(path string, id *Identity) error {
	if err := id.validate(); err != nil {
		return err
	}
	encryption, err := id.Encryption.Bytes()
	if err != nil {
		return fmt.Errorf("encode the encryption key: %w", err)
	}
	raw, err := json.Marshal(fileFormat{
		Version: fileVersion, RunnerID: id.RunnerID, OrganizationID: id.OrganizationID, ServerURL: id.ServerURL,
		ServerOrigin: id.ServerOrigin, ServerSigningKey: id.ServerSigningKey, ServerKeyVersion: id.ServerKeyVersion,
		SigningSeed: id.Signing.Seed(), EncryptionKey: encryption, Labels: id.Labels,
	})
	if err != nil {
		return fmt.Errorf("encode the identity: %w", err)
	}
	return config.WriteFileAtomic(path, raw)
}

func Load(path string) (*Identity, error) {
	exists, err := Exists(path)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrMissing
	}
	raw, err := config.ReadPrivateFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(raw)
}

func Parse(raw []byte) (*Identity, error) {
	var f fileFormat
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, ErrCorrupt
	}
	if f.Version != fileVersion {
		return nil, ErrUnsupportedV
	}
	if len(f.SigningSeed) != ed25519.SeedSize {
		return nil, ErrCorrupt
	}
	enc, err := hpke.MLKEM768X25519().NewPrivateKey(f.EncryptionKey)
	if err != nil {
		return nil, ErrCorrupt
	}
	id := &Identity{
		Keys:     Keys{Signing: ed25519.NewKeyFromSeed(f.SigningSeed), Encryption: enc},
		RunnerID: f.RunnerID, OrganizationID: f.OrganizationID, ServerURL: f.ServerURL, ServerOrigin: f.ServerOrigin,
		ServerSigningKey: ed25519.PublicKey(f.ServerSigningKey), ServerKeyVersion: f.ServerKeyVersion, Labels: f.Labels,
	}
	if err := id.validate(); err != nil {
		return nil, err
	}
	return id, nil
}

func (i *Identity) validate() error {
	if _, err := uuid.Parse(i.RunnerID); err != nil {
		return fmt.Errorf("%w: runner id", ErrCorrupt)
	}
	if _, err := uuid.Parse(i.OrganizationID); err != nil {
		return fmt.Errorf("%w: organization id", ErrCorrupt)
	}
	if i.ServerOrigin == "" || i.ServerURL == "" {
		return fmt.Errorf("%w: server address", ErrCorrupt)
	}
	if len(i.ServerSigningKey) != ed25519.PublicKeySize || i.ServerKeyVersion < 1 {
		return ErrServerKey
	}
	if len(i.Signing) != ed25519.PrivateKeySize || i.Encryption == nil {
		return fmt.Errorf("%w: keys", ErrCorrupt)
	}
	if i.Encryption.KEM().ID() != hpke.MLKEM768X25519().ID() {
		return ErrEncryptionKEM
	}
	return nil
}
