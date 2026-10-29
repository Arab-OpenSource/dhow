package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	config "github.com/Khaymah/dhow/apps/runner/internal/Config"
)

func TestParseEnvFile(t *testing.T) {
	vars, err := config.ParseEnvFile([]byte("# runner\nDHOW_RUNNER_URL=https://dhow.example.com\n\nDHOW_RUNNER_CAPACITY = \"2\"\nDHOW_RUNNER_LOG_LEVEL='debug'\n"))
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"DHOW_RUNNER_URL": "https://dhow.example.com", "DHOW_RUNNER_CAPACITY": "2", "DHOW_RUNNER_LOG_LEVEL": "debug",
	}, vars)
	for _, bad := range []string{"no equals sign\n", "=value\n", "TWO WORDS=x\n"} {
		_, err := config.ParseEnvFile([]byte(bad))
		require.ErrorIs(t, err, config.ErrEnvFileLine, bad)
	}
}

func TestWithEnvFileFillsWhatTheProcessLacks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runner.env")
	require.NoError(t, os.WriteFile(path, []byte("DHOW_RUNNER_URL=https://file.example\nDHOW_RUNNER_CAPACITY=3\n"), 0o640))
	process := map[string]string{"DHOW_RUNNER_URL": "https://process.example"}
	get, err := config.WithEnvFile(func(k string) string { return process[k] }, path)
	require.NoError(t, err)
	assert.Equal(t, "https://process.example", get("DHOW_RUNNER_URL"))
	assert.Equal(t, "3", get("DHOW_RUNNER_CAPACITY"))
	same, err := config.WithEnvFile(func(k string) string { return process[k] }, filepath.Join(t.TempDir(), "missing"))
	require.NoError(t, err)
	assert.Empty(t, same("DHOW_RUNNER_CAPACITY"))
}

func TestCredentialsDirectory(t *testing.T) {
	state := t.TempDir()
	env := map[string]string{config.EnvStateDir: state, config.EnvCredentialsDir: "/run/credentials/dhow-runner.service"}
	cfg, err := config.Load(func(k string) string { return env[k] }, config.Options{})
	require.NoError(t, err)
	assert.Equal(t, "/run/credentials/dhow-runner.service/identity", cfg.CredentialPath())
	assert.Equal(t, filepath.Join(state, "identity.cred"), cfg.SealedIdentityPath())
	env[config.EnvCredentialsDir] = "relative"
	_, err = config.Load(func(k string) string { return env[k] }, config.Options{})
	require.ErrorIs(t, err, config.ErrRelativePath)
}

func TestOwnerOfAndAtomicWritesAsAnotherUser(t *testing.T) {
	dir := t.TempDir()
	owner, err := config.OwnerOf(dir)
	require.NoError(t, err)
	assert.Equal(t, os.Geteuid(), owner.UID)
	path := filepath.Join(dir, "identity")
	require.NoError(t, config.WriteFileAtomicAs(path, []byte("x"), owner))
	got, err := config.OwnerOf(path)
	require.NoError(t, err)
	assert.Equal(t, owner, got)
	assert.False(t, config.NoOwner.Set())
	if os.Geteuid() != 0 {
		return
	}
	other := config.Owner{UID: 4242, GID: 4242}
	require.NoError(t, config.WriteFileAtomicAs(path, []byte("y"), other))
	got, err = config.OwnerOf(path)
	require.NoError(t, err)
	assert.Equal(t, other, got, "root writes the runner's files as the runner user")
}

func TestOnlyPrivilegedCommandsAcceptAnotherUsersStateDirectory(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("needs root to hand the directory to another user")
	}
	state := filepath.Join(t.TempDir(), "state")
	require.NoError(t, os.Mkdir(state, 0o700))
	require.NoError(t, os.Chown(state, 4242, 4242))
	env := map[string]string{config.EnvStateDir: state}
	_, err := config.Load(func(k string) string { return env[k] }, config.Options{})
	require.ErrorIs(t, err, config.ErrWrongOwner, "run as root refuses the runner user's directory")
	_, err = config.Load(func(k string) string { return env[k] }, config.Options{Privileged: true})
	require.NoError(t, err)
	lock, err := config.LockState(filepath.Join(state, config.LockFileName))
	require.NoError(t, err)
	require.NoError(t, lock.Release())
	owner, err := config.OwnerOf(filepath.Join(state, config.LockFileName))
	require.NoError(t, err)
	assert.Equal(t, config.Owner{UID: 4242, GID: 4242}, owner, "a lock root creates belongs to the runner user")
}
