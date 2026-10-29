package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	config "github.com/Khaymah/dhow/apps/runner/internal/Config"
)

func TestLockStateIsExclusiveUntilReleased(t *testing.T) {
	path := filepath.Join(t.TempDir(), config.LockFileName)
	first, err := config.LockState(path)
	require.NoError(t, err)
	_, err = config.LockState(path)
	require.ErrorIs(t, err, config.ErrStateBusy)
	require.NoError(t, first.Release())
	require.NoError(t, first.Release(), "releasing twice is harmless")
	again, err := config.LockState(path)
	require.NoError(t, err)
	require.NoError(t, again.Release())
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(config.PrivateFileMode), info.Mode().Perm())
}

func TestLockStateRefusesASymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "elsewhere")
	require.NoError(t, os.WriteFile(target, nil, 0o600))
	link := filepath.Join(dir, config.LockFileName)
	require.NoError(t, os.Symlink(target, link))
	_, err := config.LockState(link)
	require.ErrorIs(t, err, config.ErrNotFile)
}

func TestShredFileOverwritesAndRemoves(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pending")
	require.NoError(t, os.WriteFile(path, []byte("private key material"), 0o600))
	keep, err := os.Open(path)
	require.NoError(t, err)
	defer func() { _ = keep.Close() }()
	require.NoError(t, config.ShredFile(path))
	_, err = os.Lstat(path)
	require.ErrorIs(t, err, os.ErrNotExist)
	left := make([]byte, 64)
	n, _ := keep.Read(left)
	assert.Equal(t, make([]byte, n), left[:n], "the bytes were zeroed before the file was removed")
	require.NoError(t, config.ShredFile(path), "a missing file is already gone")

	target := filepath.Join(dir, "target")
	require.NoError(t, os.WriteFile(target, []byte("x"), 0o600))
	link := filepath.Join(dir, "link")
	require.NoError(t, os.Symlink(target, link))
	require.ErrorIs(t, config.ShredFile(link), config.ErrNotFile, "a symlink is never followed")
}
