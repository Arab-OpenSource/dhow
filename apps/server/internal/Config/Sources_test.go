package config_test

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
)

func TestSourcesDefaults(t *testing.T) {
	cfg, err := config.Load(env(valid()))
	require.NoError(t, err)
	assert.Equal(t, int64(config.WebhookMaxBytesDefault), cfg.Sources.WebhookMaxBytes)
	assert.Empty(t, cfg.Sources.AllowPrivate)
}

func TestSourcesSettings(t *testing.T) {
	m := valid()
	m[config.EnvWebhookMaxBytes] = "1048576"
	m[config.EnvGitAllowPrivateCIDRs] = "10.20.0.0/16, fd12:3456::/32"
	cfg, err := config.Load(env(m))
	require.NoError(t, err)
	assert.Equal(t, int64(1<<20), cfg.Sources.WebhookMaxBytes)
	assert.Equal(t, []netip.Prefix{netip.MustParsePrefix("10.20.0.0/16"), netip.MustParsePrefix("fd12:3456::/32")}, cfg.Sources.AllowPrivate)
}

func TestSourcesSettingsFailClosed(t *testing.T) {
	for name, kv := range map[string][2]string{
		"a tiny webhook limit":     {config.EnvWebhookMaxBytes, "10"},
		"a huge webhook limit":     {config.EnvWebhookMaxBytes, "1073741824"},
		"a webhook limit as text":  {config.EnvWebhookMaxBytes, "5MiB"},
		"a host instead of a cidr": {config.EnvGitAllowPrivateCIDRs, "10.0.0.1"},
		"everything":               {config.EnvGitAllowPrivateCIDRs, "0.0.0.0/0"},
		"an unmasked cidr":         {config.EnvGitAllowPrivateCIDRs, "10.0.0.1/8"},
	} {
		t.Run(name, func(t *testing.T) {
			m := valid()
			m[kv[0]] = kv[1]
			_, err := config.Load(env(m))
			require.Error(t, err)
			assert.Contains(t, err.Error(), kv[0])
		})
	}
}
