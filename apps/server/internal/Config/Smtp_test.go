package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
)

func withSMTP(extra map[string]string) map[string]string {
	m := valid()
	for k, v := range extra {
		m[k] = v
	}
	return m
}

func TestSMTPIsOffByDefault(t *testing.T) {
	cfg, err := resolve(valid())
	require.NoError(t, err)
	assert.False(t, cfg.Mail.Enabled())
	assert.False(t, cfg.AnyUserCreatesOrganizations())
}

func TestSMTPDefaults(t *testing.T) {
	cfg, err := resolve(withSMTP(map[string]string{
		config.EnvSMTPHost: "smtp.example.com", config.EnvSMTPFrom: "Dhow <dhow@example.com>",
		config.EnvSMTPUsername: "dhow", config.EnvSMTPPassword: "mail-relay-credential",
	}))
	require.NoError(t, err)
	assert.True(t, cfg.Mail.Enabled())
	assert.Equal(t, 587, cfg.Mail.Port)
	assert.Equal(t, config.SMTPTLSStartTLS, cfg.Mail.TLS)
	assert.Equal(t, "dhow@example.com", cfg.Mail.From.Address)
	assert.Equal(t, "Dhow", cfg.Mail.From.Name)
	assert.Equal(t, "mail-relay-credential", cfg.Mail.Password)
}

func TestSMTPImplicitTLSDefaultsTo465(t *testing.T) {
	cfg, err := resolve(withSMTP(map[string]string{
		config.EnvSMTPHost: "smtp.example.com", config.EnvSMTPFrom: "dhow@example.com", config.EnvSMTPTLS: "TLS",
	}))
	require.NoError(t, err)
	assert.Equal(t, 465, cfg.Mail.Port)
	assert.Equal(t, config.SMTPTLSImplicit, cfg.Mail.TLS)
}

func TestSMTPPasswordFromAFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "smtp")
	require.NoError(t, os.WriteFile(path, []byte("from-a-file\n"), 0o600))
	cfg, err := resolve(withSMTP(map[string]string{
		config.EnvSMTPHost: "smtp.example.com", config.EnvSMTPFrom: "dhow@example.com", config.EnvSMTPUsername: "dhow",
		config.EnvSMTPPasswordFile: path,
	}))
	require.NoError(t, err)
	assert.Equal(t, "from-a-file", cfg.Mail.Password)
}

func TestSMTPRejects(t *testing.T) {
	base := map[string]string{config.EnvSMTPHost: "smtp.example.com", config.EnvSMTPFrom: "dhow@example.com"}
	tests := []struct {
		name  string
		extra map[string]string
		want  string
	}{
		{"settings without a host", map[string]string{config.EnvSMTPHost: "", config.EnvSMTPUsername: "dhow"}, config.EnvSMTPHost},
		{"a host with a port", map[string]string{config.EnvSMTPHost: "smtp.example.com:25"}, config.EnvSMTPHost},
		{"no sender", map[string]string{config.EnvSMTPFrom: ""}, config.EnvSMTPFrom},
		{"a sender with a header break", map[string]string{config.EnvSMTPFrom: "a@example.com\r\nBcc: x@example.com"}, config.EnvSMTPFrom},
		{"a malformed sender", map[string]string{config.EnvSMTPFrom: "not an address"}, config.EnvSMTPFrom},
		{"a port out of range", map[string]string{config.EnvSMTPPort: "70000"}, config.EnvSMTPPort},
		{"an unknown TLS mode", map[string]string{config.EnvSMTPTLS: "maybe"}, config.EnvSMTPTLS},
		{"a user without a password", map[string]string{config.EnvSMTPUsername: "dhow"}, config.EnvSMTPUsername},
		{"a password without TLS", map[string]string{config.EnvSMTPTLS: "none", config.EnvSMTPUsername: "dhow", config.EnvSMTPPassword: "relay-credential"}, config.EnvSMTPTLS},
		{"an unknown organization creation mode", map[string]string{config.EnvOrgCreation: "everyone"}, config.EnvOrgCreation},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := withSMTP(base)
			for k, v := range tt.extra {
				m[k] = v
			}
			_, err := resolve(m)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
			assert.NotContains(t, err.Error(), "relay-credential")
		})
	}
}

func TestSMTPNoneWithoutCredentialsIsAllowed(t *testing.T) {
	cfg, err := resolve(withSMTP(map[string]string{
		config.EnvSMTPHost: "relay.internal", config.EnvSMTPFrom: "dhow@example.com", config.EnvSMTPTLS: "none", config.EnvSMTPPort: "25",
	}))
	require.NoError(t, err)
	assert.Equal(t, config.SMTPTLSNone, cfg.Mail.TLS)
	assert.Equal(t, 25, cfg.Mail.Port)
}

func TestOrgCreation(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{{"", false}, {"instance-owner", false}, {"ANY-USER", true}, {" any-user ", true}}
	for _, tt := range tests {
		cfg, err := resolve(withSMTP(map[string]string{config.EnvOrgCreation: tt.in}))
		require.NoError(t, err, tt.in)
		assert.Equal(t, tt.want, cfg.AnyUserCreatesOrganizations(), strings.TrimSpace(tt.in))
	}
}
