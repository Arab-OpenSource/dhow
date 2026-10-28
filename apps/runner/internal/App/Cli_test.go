package app_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	app "github.com/Arab-OpenSource/dhow/apps/runner/internal/App"
	config "github.com/Arab-OpenSource/dhow/apps/runner/internal/Config"
)

func cli(t *testing.T, env map[string]string, stdin string, args ...string) (string, error) {
	t.Helper()
	var out, errOut bytes.Buffer
	err := app.Main(context.Background(), args, app.Env{
		Getenv: func(k string) string { return env[k] }, Stdio: app.Stdio{In: strings.NewReader(stdin), Out: &out, Err: &errOut},
		Machine: func() (config.Machine, error) { return config.Machine{}, nil }, Harden: func() error { return nil },
	})
	return out.String() + errOut.String(), err
}

func TestTheTokenNeverComesFromTheCommandLine(t *testing.T) {
	env := map[string]string{config.EnvStateDir: filepath.Join(t.TempDir(), "s")}
	for _, args := range [][]string{
		{"register", "--url", "https://d.example.com", "--token", "dhow_rrt_abc"},
		{"register", "--url", "https://d.example.com", "-token=dhow_rrt_abc"},
		{"register", "--url", "https://d.example.com", "--token-file", "-", "dhow_rrt_abcdefghijklmnopqrstuv"},
	} {
		_, err := cli(t, env, "", args...)
		assert.ErrorIs(t, err, app.ErrTokenOnFlags, args)
	}
	_, err := cli(t, env, "", "register", "--url", "https://d.example.com")
	assert.ErrorIs(t, err, app.ErrNoTokenFile)
}

func TestTheTokenMustLookLikeARegistrationToken(t *testing.T) {
	env := map[string]string{config.EnvStateDir: filepath.Join(t.TempDir(), "s")}
	_, err := cli(t, env, "not-a-token\n", "register", "--url", "https://d.example.com", "--token-file", "-")
	assert.ErrorIs(t, err, app.ErrTokenFormat)
	_, err = cli(t, env, "", "register", "--token-file", "-")
	assert.ErrorIs(t, err, config.ErrNeedsURL)
	_, err = cli(t, env, "", "register", "--url", "http://d.example.com", "--token-file", "-")
	assert.ErrorIs(t, err, config.ErrNotHTTPS)
}

func TestRunNeedsARegisteredIdentity(t *testing.T) {
	env := map[string]string{config.EnvStateDir: filepath.Join(t.TempDir(), "s")}
	_, err := cli(t, env, "", "run")
	assert.ErrorIs(t, err, config.ErrNotRegistered)
}

func TestAnIdentityWithOpenPermissionsStopsTheRunner(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "s")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, config.IdentityFileName), []byte("{}"), 0o644))
	_, err := cli(t, map[string]string{config.EnvStateDir: dir}, "", "run")
	assert.ErrorIs(t, err, config.ErrOpenMode)
}

func TestUsage(t *testing.T) {
	_, err := cli(t, nil, "")
	assert.ErrorIs(t, err, app.ErrUsage)
	_, err = cli(t, nil, "", "launch")
	assert.ErrorIs(t, err, app.ErrUsage)
	out, err := cli(t, nil, "", "version")
	require.NoError(t, err)
	assert.Contains(t, out, "dhow-runner ")
}
