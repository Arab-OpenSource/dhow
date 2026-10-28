//go:build !dhowtest

package app_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	app "github.com/Khaymah/dhow/apps/runner/internal/App"
	config "github.com/Khaymah/dhow/apps/runner/internal/Config"
	identity "github.com/Khaymah/dhow/apps/runner/internal/Identity"
)

func TestAReleaseBuildRefusesToRunWithoutARealDriver(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "s")
	require.NoError(t, config.EnsurePrivateDir(dir))
	keys, err := identity.GenerateKeys()
	require.NoError(t, err)
	server, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	require.NoError(t, identity.Save(filepath.Join(dir, config.IdentityFileName), &identity.Identity{
		Keys: keys, RunnerID: uuid.NewString(), OrganizationID: uuid.NewString(), ServerURL: "https://d.example.com",
		ServerOrigin: "https://d.example.com", ServerSigningKey: server, ServerKeyVersion: 1,
	}))
	_, err = cli(t, map[string]string{config.EnvStateDir: dir, config.EnvDriver: config.DriverKubernetes}, "", "run")
	assert.ErrorIs(t, err, app.ErrDriverMissing)
	t.Setenv("PATH", t.TempDir())
	_, err = cli(t, map[string]string{config.EnvStateDir: dir}, "", "run")
	assert.ErrorContains(t, err, "podman", "the default Podman driver refuses to start without Podman")
	_, err = cli(t, map[string]string{config.EnvStateDir: dir, config.EnvDriver: config.DriverFake}, "", "run")
	assert.ErrorIs(t, err, config.ErrFakeDriver, "the fake driver is not in release builds")
}
