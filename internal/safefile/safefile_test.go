package safefile

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadRegular(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "f")
	require.NoError(t, os.WriteFile(file, []byte("0123456789"), 0o600))
	got, err := ReadRegular(file, 4)
	require.NoError(t, err)
	assert.Equal(t, "0123", string(got))
	got, err = ReadRegular(file, 100)
	require.NoError(t, err)
	assert.Equal(t, "0123456789", string(got))
}

func TestReadRegularRefusesWhatIsNotAPlainFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "f")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))
	link := filepath.Join(dir, "link")
	require.NoError(t, os.Symlink(file, link))
	dangling := filepath.Join(dir, "dangling")
	require.NoError(t, os.Symlink(filepath.Join(dir, "nowhere"), dangling))
	_, err := ReadRegular(link, 10)
	assert.ErrorIs(t, err, ErrSymlink)
	_, err = ReadRegular(dangling, 10)
	assert.ErrorIs(t, err, ErrSymlink)
	_, err = ReadRegular(dir, 10)
	assert.ErrorIs(t, err, ErrNotRegular)
	_, err = ReadRegular(filepath.Join(dir, "missing"), 10)
	assert.ErrorIs(t, err, fs.ErrNotExist)
}
