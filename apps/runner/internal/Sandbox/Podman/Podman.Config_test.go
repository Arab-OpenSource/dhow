package podman

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheStateDirMustSitUnderDirectoriesOthersCanEnter(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{filepath.Dir(root), root} {
		require.NoError(t, os.Chmod(dir, 0o711))
	}

	closed := filepath.Join(root, "home")
	require.NoError(t, os.Mkdir(closed, 0o750))
	state := filepath.Join(closed, "state")
	require.NoError(t, os.Mkdir(state, 0o700))
	err := traversable(state)
	require.ErrorIs(t, err, ErrStateDirClosed)
	assert.Contains(t, err.Error(), closed, "the error names the directory to fix")

	open := filepath.Join(root, "lib")
	require.NoError(t, os.Mkdir(open, 0o755))
	state = filepath.Join(open, "state")
	require.NoError(t, os.Mkdir(state, 0o700))
	assert.NoError(t, traversable(state), "the state directory itself stays private")
}
