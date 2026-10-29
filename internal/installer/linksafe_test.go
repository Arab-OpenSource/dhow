package installer_test

import (
	"os"
	"os/user"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Khaymah/dhow/internal/installer"
)

func ownMode(t *testing.T, perm os.FileMode) installer.FileMode {
	t.Helper()
	u, err := user.Current()
	require.NoError(t, err)
	g, err := user.LookupGroupId(u.Gid)
	require.NoError(t, err)
	return installer.FileMode{Perm: perm, Owner: u.Username, Group: g.Name}
}

func plantLink(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	target := filepath.Join(dir, "shadow")
	require.NoError(t, os.WriteFile(target, []byte("root secrets"), 0o600))
	link := filepath.Join(dir, "identity")
	require.NoError(t, os.Symlink(target, link))
	return target, link
}

func TestNoFollowOperationsRefuseAPlantedSymlink(t *testing.T) {
	h := installer.NewHost()
	target, link := plantLink(t)

	info, err := h.Lstat(link)
	require.NoError(t, err)
	assert.True(t, info.IsSymlink())
	assert.False(t, info.IsRegular())

	require.ErrorIs(t, h.ChmodNoFollow(link, ownMode(t, 0o644)), installer.ErrNotRegular)
	_, _, err = h.ReadFileNoFollow(link, 1<<10)
	require.ErrorIs(t, err, installer.ErrNotRegular)
	st, err := os.Stat(target)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), st.Mode().Perm(), "the target keeps its mode")

	require.NoError(t, h.ShredNoFollow(link))
	_, err = os.Lstat(link)
	require.ErrorIs(t, err, os.ErrNotExist, "the link is removed")
	data, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "root secrets", string(data), "the target is not overwritten")
}

func TestNoFollowOperationsWorkOnRegularFiles(t *testing.T) {
	h := installer.NewHost()
	path := filepath.Join(t.TempDir(), "identity")
	require.NoError(t, os.WriteFile(path, []byte("keys"), 0o644))
	require.NoError(t, h.ChmodNoFollow(path, ownMode(t, 0o600)))
	data, info, err := h.ReadFileNoFollow(path, 1<<10)
	require.NoError(t, err)
	assert.Equal(t, "keys", string(data))
	assert.Equal(t, os.FileMode(0o600), info.Mode.Perm())
	_, _, err = h.ReadFileNoFollow(path, 2)
	require.Error(t, err)
	require.NoError(t, h.ShredNoFollow(path))
	_, err = os.Lstat(path)
	require.ErrorIs(t, err, os.ErrNotExist)
	require.NoError(t, h.ShredNoFollow(path), "shredding twice is fine")
}
