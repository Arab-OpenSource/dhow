package identity

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"

	config "github.com/Khaymah/dhow/apps/runner/internal/Config"
	"github.com/Khaymah/dhow/internal/runnersig"
)

var (
	ErrPendingMismatch = errors.New("the pending identity belongs to another runner or server")
	ErrKeyVersionLimit = errors.New("the key version cannot grow any further")
)

type RotateProof struct {
	OldKeySignature []byte
	NewKeySignature []byte
}

func (i *Identity) Next(keys Keys) (*Identity, error) {
	if i.KeyVersion == math.MaxInt32 {
		return nil, ErrKeyVersionLimit
	}
	next := *i
	next.Keys = keys
	next.KeyVersion = i.KeyVersion + 1
	next.Labels = append([]string(nil), i.Labels...)
	return &next, nil
}

func (i *Identity) ProveRotation(next Keys) RotateProof {
	message := runnersig.RotateMessage(i.ServerOrigin, i.RunnerID, i.KeyVersion, next.SigningPublicKey(), next.EncryptionPublicKey())
	return RotateProof{OldKeySignature: i.Sign(message), NewKeySignature: next.Sign(message)}
}

func (i *Identity) SameKeys(other *Identity) bool {
	return bytes.Equal(i.SigningPublicKey(), other.SigningPublicKey()) &&
		bytes.Equal(i.EncryptionPublicKey(), other.EncryptionPublicKey())
}

func (i *Identity) Matches(pending *Identity) error {
	if pending.RunnerID != i.RunnerID || pending.OrganizationID != i.OrganizationID || pending.ServerOrigin != i.ServerOrigin ||
		!bytes.Equal(pending.ServerSigningKey, i.ServerSigningKey) {
		return ErrPendingMismatch
	}
	return nil
}

func Promote(pendingPath, path string) error {
	if err := config.RenameDurable(pendingPath, path); err != nil {
		return fmt.Errorf("switch to the new identity: %w", err)
	}
	return nil
}

func RemoveStaleTemps(dir string) error {
	for _, name := range []string{config.IdentityFileName, config.PendingFileName} {
		matches, err := filepath.Glob(filepath.Join(dir, "."+name+".tmp-*"))
		if err != nil {
			return err
		}
		for _, m := range matches {
			if err := os.Remove(m); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("remove %s: %w", m, err)
			}
		}
	}
	return nil
}
