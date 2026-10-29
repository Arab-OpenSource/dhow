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

func TestRenameDurableMovesTheFile(t *testing.T) {
	dir := t.TempDir()
	from, to := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	require.NoError(t, os.WriteFile(from, []byte("x"), 0o600))
	require.NoError(t, config.RenameDurable(from, to))
	raw, err := os.ReadFile(to)
	require.NoError(t, err)
	assert.Equal(t, "x", string(raw))
	assert.Error(t, config.RenameDurable(from, to), "a missing source is an error")
}
