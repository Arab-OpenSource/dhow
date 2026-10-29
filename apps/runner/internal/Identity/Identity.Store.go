package identity

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	config "github.com/Khaymah/dhow/apps/runner/internal/Config"
)

const maxCredential = 1 << 20

var (
	ErrPendingSealed = errors.New("an unfinished key rotation left new keys that only root can open; run sudo dhow-runner rotate-keys to finish it")
	ErrSealedNoRoot  = errors.New("the identity is encrypted with systemd-creds; start the runner with its unit, which opens it, or run this command as root")
	ErrBothKinds     = errors.New("the state directory holds both a plain identity and an encrypted one; run sudo dhow-runner doctor")
)

type Store interface {
	Encrypted() bool
	Path() string
	PendingPath() string
	Exists() (bool, error)
	Load() (*Identity, error)
	Save(id *Identity) error
	LoadPending() (*Identity, error)
	SavePending(id *Identity) error
	Promote(next *Identity) error
	Discard() error
	RemovePending() error
	RemoveStaleTemps() error
}

type FileStore struct {
	Dir   string
	Owner config.Owner
}

func NewFileStore(dir string) FileStore { return FileStore{Dir: dir, Owner: config.NoOwner} }

func (s FileStore) Encrypted() bool { return false }

func (s FileStore) Path() string { return filepath.Join(s.Dir, config.IdentityFileName) }

func (s FileStore) PendingPath() string { return filepath.Join(s.Dir, config.PendingFileName) }

func (s FileStore) Exists() (bool, error) { return Exists(s.Path()) }

func (s FileStore) Load() (*Identity, error) { return Load(s.Path()) }

func (s FileStore) Save(id *Identity) error { return SaveAs(s.Path(), id, s.Owner) }

func (s FileStore) SavePending(id *Identity) error { return SaveAs(s.PendingPath(), id, s.Owner) }

func (s FileStore) LoadPending() (*Identity, error) {
	exists, err := Exists(s.PendingPath())
	if err != nil || !exists {
		return nil, err
	}
	return Load(s.PendingPath())
}

func (s FileStore) Promote(next *Identity) error {
	if err := s.Save(next); err != nil {
		return fmt.Errorf("switch to the new identity: %w", err)
	}
	if err := config.RemoveDurable(s.PendingPath()); err != nil {
		return fmt.Errorf("remove the pending identity after the switch: %w", err)
	}
	return nil
}

func (s FileStore) Discard() error { return Discard(s.PendingPath()) }

func (s FileStore) RemovePending() error { return config.RemoveDurable(s.PendingPath()) }

func (s FileStore) RemoveStaleTemps() error { return RemoveStaleTemps(s.Dir) }

type Sealer interface {
	Seal(name string, plain []byte) ([]byte, error)
	Open(name string, sealed []byte) ([]byte, error)
}

type SealedStore struct {
	Dir      string
	Unsealed string
	Sealer   Sealer
}

func (s SealedStore) Encrypted() bool { return true }

func (s SealedStore) Path() string { return filepath.Join(s.Dir, config.SealedIdentityFileName) }

func (s SealedStore) PendingPath() string { return filepath.Join(s.Dir, config.SealedPendingFileName) }

func (s SealedStore) Exists() (bool, error) { return Exists(s.Path()) }

func (s SealedStore) Load() (*Identity, error) {
	if s.Unsealed != "" {
		passed, err := Exists(s.Unsealed)
		if err != nil {
			return nil, err
		}
		if passed {
			raw, err := readCredential(s.Unsealed)
			if err != nil {
				return nil, err
			}
			return Parse(raw)
		}
	}
	return s.open(s.Path())
}

func (s SealedStore) open(path string) (*Identity, error) {
	exists, err := Exists(path)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrMissing
	}
	if s.Sealer == nil {
		return nil, ErrSealedNoRoot
	}
	sealed, err := readCredential(path)
	if err != nil {
		return nil, err
	}
	raw, err := s.Sealer.Open(config.IdentityCredential, sealed)
	if err != nil {
		return nil, fmt.Errorf("open %s with systemd-creds: %w", path, err)
	}
	return Parse(raw)
}

func (s SealedStore) seal(path string, id *Identity) error {
	if s.Sealer == nil {
		return ErrSealedNoRoot
	}
	raw, err := Marshal(id)
	if err != nil {
		return err
	}
	sealed, err := s.Sealer.Seal(config.IdentityCredential, raw)
	clear(raw)
	if err != nil {
		return fmt.Errorf("encrypt the identity with systemd-creds: %w", err)
	}
	return config.WriteFileAtomic(path, sealed)
}

func (s SealedStore) Save(id *Identity) error { return s.seal(s.Path(), id) }

func (s SealedStore) SavePending(id *Identity) error { return s.seal(s.PendingPath(), id) }

func (s SealedStore) LoadPending() (*Identity, error) {
	exists, err := Exists(s.PendingPath())
	if err != nil || !exists {
		return nil, err
	}
	if s.Sealer == nil {
		return nil, ErrPendingSealed
	}
	return s.open(s.PendingPath())
}

func (s SealedStore) Promote(next *Identity) error {
	if err := s.Save(next); err != nil {
		return fmt.Errorf("switch to the new identity: %w", err)
	}
	if err := config.RemoveDurable(s.PendingPath()); err != nil {
		return fmt.Errorf("remove the pending identity after the switch: %w", err)
	}
	return nil
}

func (s SealedStore) Discard() error { return config.RemoveDurable(s.PendingPath()) }

func (s SealedStore) RemovePending() error { return config.RemoveDurable(s.PendingPath()) }

func (s SealedStore) RemoveStaleTemps() error { return RemoveStaleTemps(s.Dir) }

func readCredential(path string) ([]byte, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0) //nolint:gosec // The runner's own state directory, or the credentials directory systemd made for the unit.
	if errors.Is(err, syscall.ELOOP) {
		return nil, fmt.Errorf("%s %w", path, config.ErrNotFile)
	}
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s %w", path, config.ErrNotFile)
	}
	data, err := io.ReadAll(io.LimitReader(f, maxCredential+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if len(data) > maxCredential {
		return nil, fmt.Errorf("%s %w", path, config.ErrTooLarge)
	}
	return data, nil
}

func Open(cfg config.Config, sealer Sealer, owner config.Owner) (Store, error) {
	sealed, err := Exists(cfg.SealedIdentityPath())
	if err != nil {
		return nil, err
	}
	plain, err := Exists(cfg.IdentityPath())
	if err != nil {
		return nil, err
	}
	switch {
	case sealed && plain:
		return nil, ErrBothKinds
	case sealed:
		return SealedStore{Dir: cfg.StateDir, Unsealed: cfg.CredentialPath(), Sealer: sealer}, nil
	default:
		return FileStore{Dir: cfg.StateDir, Owner: owner}, nil
	}
}
