//go:build dhowtest

package runnertest

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"testing"

	config "github.com/Khaymah/dhow/apps/runner/internal/Config"
	identity "github.com/Khaymah/dhow/apps/runner/internal/Identity"
)

var ErrSealedTampered = errors.New("the sealed test credential does not open")

type Sealer struct {
	aead  cipher.AEAD
	Seals int
}

func NewSealer(t testing.TB) *Sealer {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	return &Sealer{aead: aead}
}

func (s *Sealer) Seal(name string, plain []byte) ([]byte, error) {
	s.Seals++
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return s.aead.Seal(nonce, nonce, plain, []byte(name)), nil
}

func (s *Sealer) Open(name string, sealed []byte) ([]byte, error) {
	n := s.aead.NonceSize()
	if len(sealed) < n {
		return nil, ErrSealedTampered
	}
	plain, err := s.aead.Open(nil, sealed[:n], sealed[n:], []byte(name))
	if err != nil {
		return nil, ErrSealedTampered
	}
	return plain, nil
}

func (r *Runner) Seal(s *Sealer) {
	r.T.Helper()
	id := r.Identity()
	if err := (identity.SealedStore{Dir: r.StateDir, Sealer: s}).Save(id); err != nil {
		r.T.Fatal(err)
	}
	if err := config.ShredFile(filepath.Join(r.StateDir, config.IdentityFileName)); err != nil {
		r.T.Fatal(err)
	}
	r.Sealer = s
}

func (r *Runner) Store() identity.Store {
	if r.Sealer != nil {
		return identity.SealedStore{Dir: r.StateDir, Sealer: r.Sealer}
	}
	return identity.NewFileStore(r.StateDir)
}

func (r *Runner) UnitStore() identity.SealedStore {
	r.T.Helper()
	sealed, err := os.ReadFile(filepath.Join(r.StateDir, config.SealedIdentityFileName))
	if err != nil {
		r.T.Fatal(err)
	}
	plain, err := r.Sealer.Open(config.IdentityCredential, sealed)
	if err != nil {
		r.T.Fatal(err)
	}
	dir := r.T.TempDir()
	path := filepath.Join(dir, config.IdentityCredential)
	if err := os.WriteFile(path, plain, 0o400); err != nil { //nolint:gosec // A path inside the test temporary directory.
		r.T.Fatal(err)
	}
	return identity.SealedStore{Dir: r.StateDir, Unsealed: path}
}

func (r *Runner) DiskHolds(secret []byte) bool {
	r.T.Helper()
	found := false
	err := filepath.WalkDir(r.StateDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path) //nolint:gosec // A test's own temporary state directory.
		if err != nil {
			return err
		}
		found = found || bytes.Contains(data, secret)
		return nil
	})
	if err != nil {
		r.T.Fatal(err)
	}
	return found
}
