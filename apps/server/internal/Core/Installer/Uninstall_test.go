package installer

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUninstallKeepsDataAndSecrets(t *testing.T) {
	f, _ := installedHost(t)
	app, out := newApp(f, "")
	require.NoError(t, app.Uninstall(context.Background(), UninstallOptions{Yes: true}), out.String())
	for _, gone := range []string{UnitPath, BinaryPath, EnvFilePath} {
		assert.False(t, f.has(gone), gone)
	}
	for _, kept := range []string{SecretKeyPath, DatabaseURLPath, AnswersPath} {
		assert.True(t, f.has(kept), kept)
	}
	assert.True(t, f.ran("systemctl disable --now dhow"))
	assert.True(t, f.ran("systemctl daemon-reload"))
	assert.False(t, f.ran("userdel"))
	assert.Contains(t, out.String(), "--purge")

	app, out = newApp(f, "")
	require.NoError(t, app.Uninstall(context.Background(), UninstallOptions{Yes: true}), out.String())

	app, out = newApp(f, PurgePhrase+"\n")
	require.NoError(t, app.Uninstall(context.Background(), UninstallOptions{Purge: true}), out.String())
	assert.False(t, f.has(SecretKeyPath), "purge after a plain uninstall still deletes the data")
	assert.True(t, f.ran("userdel dhow"))
	assert.Contains(t, strings.Join(f.stdins, ""), "DROP DATABASE IF EXISTS dhow")
}

func TestUninstallPurgeNeedsTheTypedPhrase(t *testing.T) {
	f, _ := installedHost(t)
	app, out := newApp(f, "delete it\n")
	err := app.Uninstall(context.Background(), UninstallOptions{Purge: true, Yes: true})
	require.Error(t, err)
	assert.Contains(t, out.String(), "type exactly")
	assert.True(t, f.has(SecretKeyPath))
	assert.True(t, f.has(UnitPath), "a wrong phrase changes nothing")

	app, out = newApp(f, PurgePhrase+"\n")
	require.NoError(t, app.Uninstall(context.Background(), UninstallOptions{Purge: true}), out.String())
	assert.False(t, f.has(SecretKeyPath))
	assert.False(t, f.has(ConfigDir))
	assert.True(t, f.ran("userdel dhow"))
	assert.Contains(t, strings.Join(f.stdins, ""), "DROP DATABASE IF EXISTS dhow WITH (FORCE)")
	assert.Contains(t, out.String(), "The dhow database and its role are deleted too.")
}

func TestUninstallWithoutAnInstall(t *testing.T) {
	app, out := newApp(debianHost(), "")
	require.NoError(t, app.Uninstall(context.Background(), UninstallOptions{Yes: true}))
	assert.Contains(t, out.String(), "Nothing to uninstall.")
}

func TestUninstallRemovesTheManagedCaddySite(t *testing.T) {
	f, _ := installedHost(t)
	f.put(CaddySitePath, "site", 0o644)
	f.put(CaddyfilePath, string(WithCaddyImport([]byte(":80 {\n}\n"))), 0o644)
	app, out := newApp(f, "")
	require.NoError(t, app.Uninstall(context.Background(), UninstallOptions{Yes: true}), out.String())
	assert.False(t, f.has(CaddySitePath))
	assert.Equal(t, ":80 {\n}\n", f.content(CaddyfilePath))
	assert.True(t, f.ran("systemctl reload-or-restart caddy"))
}
