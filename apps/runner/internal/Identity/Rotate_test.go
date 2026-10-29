package identity_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	config "github.com/Khaymah/dhow/apps/runner/internal/Config"
	identity "github.com/Khaymah/dhow/apps/runner/internal/Identity"
	"github.com/Khaymah/dhow/internal/runnersig"
)

func TestAVersionOneIdentityFileReadsAsTheFirstKeyVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), config.IdentityFileName)
	id := sample(t)
	id.KeyVersion = 7
	require.NoError(t, identity.Save(path, id))
	got, err := identity.Load(path)
	require.NoError(t, err)
	assert.Equal(t, int32(7), got.KeyVersion)

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	var fields map[string]any
	require.NoError(t, json.Unmarshal(raw, &fields))
	assert.InDelta(t, 2, fields["Version"], 0)
	fields["Version"] = 1
	delete(fields, "KeyVersion")
	old, err := json.Marshal(fields)
	require.NoError(t, err)
	got, err = identity.Parse(old)
	require.NoError(t, err)
	assert.Equal(t, int32(identity.FirstKeyVersion), got.KeyVersion)

	fields["Version"] = 2
	missing, err := json.Marshal(fields)
	require.NoError(t, err)
	_, err = identity.Parse(missing)
	assert.ErrorIs(t, err, identity.ErrCorrupt, "a version 2 file must carry its key version")
}

func TestNextKeepsTheRunnerAndSignsTheRotationWithBothKeys(t *testing.T) {
	current := sample(t)
	keys, err := identity.GenerateKeys()
	require.NoError(t, err)
	next, err := current.Next(keys)
	require.NoError(t, err)
	assert.Equal(t, current.RunnerID, next.RunnerID)
	assert.Equal(t, current.KeyVersion+1, next.KeyVersion)
	assert.False(t, next.SameKeys(current))
	require.NoError(t, current.Matches(next))
	assert.Equal(t, int32(1), current.KeyVersion, "the current identity is not changed")

	proof := current.ProveRotation(keys)
	msg := runnersig.RotateMessage(current.ServerOrigin, current.RunnerID, current.KeyVersion, keys.SigningPublicKey(), keys.EncryptionPublicKey())
	assert.True(t, runnersig.Verify(current.SigningPublicKey(), msg, proof.OldKeySignature))
	assert.True(t, runnersig.Verify(keys.SigningPublicKey(), msg, proof.NewKeySignature))

	current.KeyVersion = math.MaxInt32
	_, err = current.Next(keys)
	assert.ErrorIs(t, err, identity.ErrKeyVersionLimit)
}

func TestMatchesRefusesAnotherRunnerOrServer(t *testing.T) {
	current := sample(t)
	tests := map[string]func(*identity.Identity){
		"runner":     func(i *identity.Identity) { i.RunnerID = "0190c8a2-0000-7000-8000-0000000000ff" },
		"origin":     func(i *identity.Identity) { i.ServerOrigin = "https://other.example.com" },
		"server key": func(i *identity.Identity) { i.ServerSigningKey = sample(t).ServerSigningKey },
	}
	for name, change := range tests {
		t.Run(name, func(t *testing.T) {
			other := *current
			change(&other)
			assert.ErrorIs(t, current.Matches(&other), identity.ErrPendingMismatch)
		})
	}
}

func TestPromoteSwapsThePendingFileAndRemoveStaleTempsCleansUp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, config.IdentityFileName)
	pending := filepath.Join(dir, config.PendingFileName)
	current := sample(t)
	require.NoError(t, identity.Save(path, current))
	keys, err := identity.GenerateKeys()
	require.NoError(t, err)
	next, err := current.Next(keys)
	require.NoError(t, err)
	require.NoError(t, identity.Save(pending, next))
	require.NoError(t, identity.Promote(pending, path))
	got, err := identity.Load(path)
	require.NoError(t, err)
	assert.True(t, got.SameKeys(next))
	_, err = os.Lstat(pending)
	assert.ErrorIs(t, err, os.ErrNotExist)

	for _, name := range []string{".identity.tmp-1", ".identity.pending.tmp-2", ".journal.tmp-3"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600))
	}
	require.NoError(t, identity.RemoveStaleTemps(dir))
	left, err := filepath.Glob(filepath.Join(dir, ".*"))
	require.NoError(t, err)
	assert.Equal(t, []string{filepath.Join(dir, ".journal.tmp-3")}, left, "only identity temp files are removed")
}
