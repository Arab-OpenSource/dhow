package identity_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	config "github.com/Khaymah/dhow/apps/runner/internal/Config"
	identity "github.com/Khaymah/dhow/apps/runner/internal/Identity"
	"github.com/Khaymah/dhow/internal/runnersig"
)

func sample(t *testing.T) *identity.Identity {
	t.Helper()
	keys, err := identity.GenerateKeys()
	require.NoError(t, err)
	server, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	return &identity.Identity{
		Keys: keys, RunnerID: uuid.NewString(), OrganizationID: uuid.NewString(), ServerURL: "https://dhow.example.com",
		ServerOrigin: "https://dhow.example.com", ServerSigningKey: server, ServerKeyVersion: 1, KeyVersion: 1, Labels: []string{"dhow.os=linux"},
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), config.IdentityFileName)
	id := sample(t)
	require.NoError(t, identity.Save(path, id))
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	got, err := identity.Load(path)
	require.NoError(t, err)
	assert.Equal(t, id.RunnerID, got.RunnerID)
	assert.Equal(t, id.SigningPublicKey(), got.SigningPublicKey())
	assert.Equal(t, id.EncryptionPublicKey(), got.EncryptionPublicKey())
	msg := []byte("hello")
	assert.True(t, ed25519.Verify(id.SigningPublicKey(), msg, got.Sign(msg)))

	aad := runnersig.SecretsAAD("l", 1, "a", id.RunnerID)
	enc, ct, err := runnersig.SealSecrets(id.EncryptionPublicKey(), aad, []byte("value"))
	require.NoError(t, err)
	plain, err := runnersig.OpenSecrets(got.Encryption, enc, aad, ct)
	require.NoError(t, err)
	assert.Equal(t, "value", string(plain), "the reloaded encryption key opens secrets sealed for the registered key")
}

func TestLoadRefusesOpenPermissionsAndDamage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, config.IdentityFileName)
	_, err := identity.Load(path)
	assert.ErrorIs(t, err, config.ErrNotRegistered)

	require.NoError(t, identity.Save(path, sample(t)))
	require.NoError(t, os.Chmod(path, 0o644))
	_, err = identity.Load(path)
	assert.ErrorIs(t, err, config.ErrOpenMode)

	require.NoError(t, os.Chmod(path, 0o600))
	require.NoError(t, os.WriteFile(path, []byte(`{"Version":1}`), 0o600))
	_, err = identity.Load(path)
	assert.ErrorIs(t, err, identity.ErrCorrupt)
	_, err = identity.Parse([]byte("not json"))
	assert.ErrorIs(t, err, identity.ErrCorrupt)
	_, err = identity.Parse([]byte(`{"Version":3}`))
	assert.ErrorIs(t, err, identity.ErrUnsupportedV)
}

func TestSaveRefusesABadServerKey(t *testing.T) {
	id := sample(t)
	id.ServerSigningKey = id.ServerSigningKey[:10]
	assert.ErrorIs(t, identity.Save(filepath.Join(t.TempDir(), "x"), id), identity.ErrServerKey)
}

func TestTheRegisterProofVerifies(t *testing.T) {
	keys, err := identity.GenerateKeys()
	require.NoError(t, err)
	proof := keys.RegisterProof("https://dhow.example.com")
	msg := runnersig.RegisterMessage("https://dhow.example.com", keys.SigningPublicKey(), keys.EncryptionPublicKey())
	assert.True(t, runnersig.Verify(keys.SigningPublicKey(), msg, proof))
	assert.Len(t, keys.EncryptionPublicKey(), runnersig.EncryptionKeySize)
}

func TestLoadRefusesASymlinkedStateDirectoryAndSpecialFiles(t *testing.T) {
	base := t.TempDir()
	real := filepath.Join(base, "real")
	require.NoError(t, os.Mkdir(real, 0o700))
	require.NoError(t, identity.Save(filepath.Join(real, config.IdentityFileName), sample(t)))
	link := filepath.Join(base, "link")
	require.NoError(t, os.Symlink(real, link))
	_, err := identity.Load(filepath.Join(link, config.IdentityFileName))
	assert.ErrorIs(t, err, config.ErrNotFile, "the state directory itself must not be a symlink")

	fifo := filepath.Join(real, "fifo")
	require.NoError(t, syscall.Mkfifo(fifo, 0o600))
	done := make(chan error, 1)
	go func() { _, err := config.ReadPrivateFile(fifo); done <- err }()
	select {
	case err := <-done:
		assert.ErrorIs(t, err, config.ErrNotFile)
	case <-time.After(5 * time.Second):
		t.Fatal("reading a FIFO blocked")
	}
}
