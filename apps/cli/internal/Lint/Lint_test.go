package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Khaymah/dhow/internal/pipelinespec"
	"github.com/Khaymah/dhow/internal/pipelinespec/pipelinespectest"
)

func write(t *testing.T, name, src string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(src), 0o600))
	return path
}

func TestFileGivesExactlyTheValidatorIssues(t *testing.T) {
	for _, tc := range pipelinespectest.Invalid() {
		path := write(t, "p.yml", tc.Source)
		res, err := File(path)
		require.NoError(t, err, tc.Name)
		_, want := pipelinespec.Check([]byte(tc.Source))
		assert.Equal(t, want, res.Issues, tc.Name)
		assert.Positive(t, res.Errors(), tc.Name)
	}
}

func TestCounts(t *testing.T) {
	res := Source("p", []byte(pipelinespectest.Pipeline("  a:\n    image: alpine:3\n    steps:\n      - run: echo ${{ git.sha }}\n")))
	assert.Equal(t, 1, res.Errors())
	assert.Equal(t, 1, res.Warnings())
}

func TestOversizedFileIsReadOnlyUpToTheLimit(t *testing.T) {
	path := write(t, "big.yml", strings.Repeat("#", 4*pipelinespec.MaxFileBytes))
	res, err := File(path)
	require.NoError(t, err)
	require.Len(t, res.Issues, 1)
	assert.Equal(t, pipelinespec.CodeFileTooLarge, res.Issues[0].Code)
}

func TestFileErrors(t *testing.T) {
	dir := t.TempDir()
	_, err := File(filepath.Join(dir, "missing.yml"))
	assert.ErrorIs(t, err, ErrNotFound)
	_, err = File(dir)
	assert.ErrorIs(t, err, ErrNotRegular)
	_, err = File(os.DevNull)
	assert.ErrorIs(t, err, ErrNotRegular)
	if os.Geteuid() != 0 {
		locked := write(t, "locked.yml", "version: 1\n")
		require.NoError(t, os.Chmod(locked, 0o000))
		_, err = File(locked)
		assert.ErrorIs(t, err, ErrNoAccess)
	}
}

func TestSymlinksAreNeverFollowed(t *testing.T) {
	secret := write(t, "config.json", "{\"auths\": {}}\n")
	link := filepath.Join(t.TempDir(), ".dhow.yml")
	require.NoError(t, os.Symlink(secret, link))
	res, err := File(link)
	require.ErrorIs(t, err, ErrSymlink)
	assert.Empty(t, res.Issues)
	dangling := filepath.Join(t.TempDir(), ".dhow.yml")
	require.NoError(t, os.Symlink(filepath.Join(t.TempDir(), "nowhere"), dangling))
	_, err = File(dangling)
	assert.ErrorIs(t, err, ErrSymlink)
}

func TestResolve(t *testing.T) {
	dir := t.TempDir()
	file := write(t, "x.yml", "")
	assert.Equal(t, DefaultFile, Resolve(""))
	assert.Equal(t, filepath.Join(dir, DefaultFile), Resolve(dir))
	assert.Equal(t, file, Resolve(file))
	assert.Equal(t, "missing.yml", Resolve("missing.yml"))
}
