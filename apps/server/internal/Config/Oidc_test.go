package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
)

const oidcTestSecret = "Zq3vT8wLp2Rk9sXy4Nb7Hc1Jd6Fg0Ma5"

func withOIDC(extra map[string]string) map[string]string {
	m := valid()
	for k, v := range extra {
		m[k] = v
	}
	return m
}

func TestOIDCProvidersAreOptional(t *testing.T) {
	cfg, err := config.Load(env(valid()))
	require.NoError(t, err)
	assert.Empty(t, cfg.OIDCProviders)
}

func TestOIDCProvidersLoad(t *testing.T) {
	cfg, err := config.Load(env(withOIDC(map[string]string{
		config.EnvOIDCProviders:                           "google, github",
		"DHOW_OIDC_GOOGLE_ISSUER":                         "https://accounts.google.com",
		"DHOW_OIDC_GOOGLE_CLIENT_ID":                      "client-1.apps.googleusercontent.com",
		"DHOW_OIDC_GOOGLE_CLIENT_SECRET":                  oidcTestSecret,
		"DHOW_OIDC_GOOGLE_NAME":                           "Google",
		"DHOW_OIDC_GOOGLE_ALLOWED_DOMAINS":                "Example.com, example.org,example.com",
		"DHOW_OIDC_GITHUB_TYPE":                           "github",
		"DHOW_OIDC_GITHUB_CLIENT_ID":                      "Iv1.0123456789",
		"DHOW_OIDC_GITHUB_CLIENT_SECRET":                  oidcTestSecret,
		config.OIDCEnvName("unused", "_CLIENT_ID"):        "ignored",
		config.OIDCEnvName("google", "_UNKNOWN_SETTINGS"): "ignored",
	})))
	require.NoError(t, err)
	require.Len(t, cfg.OIDCProviders, 2)
	google, github := cfg.OIDCProviders[0], cfg.OIDCProviders[1]
	assert.Equal(t, config.OIDCProvider{
		ID: "google", Kind: config.OIDCKindOIDC, Name: "Google", Issuer: "https://accounts.google.com",
		ClientID: "client-1.apps.googleusercontent.com", ClientSecret: oidcTestSecret, AllowedDomains: []string{"example.com", "example.org"},
	}, google)
	assert.Equal(t, config.OIDCKindGitHub, github.Kind)
	assert.Equal(t, "GitHub", github.Name)
	assert.Equal(t, config.OIDCDefaultGitHubIssuer, github.Issuer)
	assert.Empty(t, github.AllowedDomains)
}

func TestOIDCProviderSecretFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret")
	require.NoError(t, os.WriteFile(path, []byte(oidcTestSecret+"\n"), 0o600))
	cfg, err := config.Load(env(withOIDC(map[string]string{
		config.EnvOIDCProviders:             "corp",
		"DHOW_OIDC_CORP_ISSUER":             "https://sso.example.com/realms/dhow",
		"DHOW_OIDC_CORP_CLIENT_ID":          "dhow",
		"DHOW_OIDC_CORP_CLIENT_SECRET_FILE": path,
	})))
	require.NoError(t, err)
	require.Len(t, cfg.OIDCProviders, 1)
	assert.Equal(t, oidcTestSecret, cfg.OIDCProviders[0].ClientSecret)
	assert.Equal(t, "corp", cfg.OIDCProviders[0].Name)
}

func TestOIDCProviderConfigFailsClosed(t *testing.T) {
	base := func() map[string]string {
		return map[string]string{
			config.EnvOIDCProviders:        "corp",
			"DHOW_OIDC_CORP_ISSUER":        "https://sso.example.com",
			"DHOW_OIDC_CORP_CLIENT_ID":     "dhow",
			"DHOW_OIDC_CORP_CLIENT_SECRET": oidcTestSecret,
		}
	}
	tests := []struct {
		name   string
		change map[string]string
		want   string
	}{
		{"bad id", map[string]string{config.EnvOIDCProviders: "Corp-1"}, "is not a provider id"},
		{"duplicate id", map[string]string{config.EnvOIDCProviders: "corp,corp"}, "listed twice"},
		{"missing issuer", map[string]string{"DHOW_OIDC_CORP_ISSUER": ""}, "DHOW_OIDC_CORP_ISSUER: is required"},
		{"plain http issuer", map[string]string{"DHOW_OIDC_CORP_ISSUER": "http://sso.example.com"}, "must use https"},
		{"issuer with a query", map[string]string{"DHOW_OIDC_CORP_ISSUER": "https://sso.example.com/?x=1"}, "must not contain"},
		{"relative issuer", map[string]string{"DHOW_OIDC_CORP_ISSUER": "/realms/dhow"}, "absolute URL"},
		{"missing client id", map[string]string{"DHOW_OIDC_CORP_CLIENT_ID": ""}, "DHOW_OIDC_CORP_CLIENT_ID: is required"},
		{"missing secret", map[string]string{"DHOW_OIDC_CORP_CLIENT_SECRET": ""}, "DHOW_OIDC_CORP_CLIENT_SECRET: is required"},
		{"placeholder secret", map[string]string{"DHOW_OIDC_CORP_CLIENT_SECRET": "change-me-please"}, "placeholder"},
		{"unknown type", map[string]string{"DHOW_OIDC_CORP_TYPE": "saml"}, "must be oidc or github"},
		{"bad domain", map[string]string{"DHOW_OIDC_CORP_ALLOWED_DOMAINS": "example.com,not a domain"}, "is not a domain name"},
		{"wildcard domain", map[string]string{"DHOW_OIDC_CORP_ALLOWED_DOMAINS": "*.example.com"}, "is not a domain name"},
		{"control character in the name", map[string]string{"DHOW_OIDC_CORP_NAME": "Corp\x07"}, "control characters"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := base()
			for k, v := range tt.change {
				m[k] = v
			}
			_, err := config.Load(env(withOIDC(m)))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
			assert.NotContains(t, err.Error(), oidcTestSecret)
		})
	}
}

func TestOIDCIssuerAllowsHTTPOnlyInDev(t *testing.T) {
	m := withOIDC(map[string]string{
		config.EnvOIDCProviders:         "local",
		"DHOW_OIDC_LOCAL_ISSUER":        "http://localhost:8180/realms/dhow",
		"DHOW_OIDC_LOCAL_CLIENT_ID":     "dhow",
		"DHOW_OIDC_LOCAL_CLIENT_SECRET": oidcTestSecret,
		config.EnvDev:                   "true",
		config.EnvPublicURL:             "http://localhost:8080",
		config.EnvDatabaseURL:           "postgres://dhow:pw@localhost:5432/dhow",
	})
	cfg, err := config.Load(env(m))
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:8180/realms/dhow", cfg.OIDCProviders[0].Issuer)
}

func TestOIDCProvidersMustNotShareAnIssuer(t *testing.T) {
	provider := func(id, issuer string) map[string]string {
		return map[string]string{
			config.OIDCEnvName(id, "_ISSUER"): issuer, config.OIDCEnvName(id, "_CLIENT_ID"): id + "-client",
			config.OIDCEnvName(id, "_CLIENT_SECRET"): oidcTestSecret,
		}
	}
	tests := []struct {
		name    string
		a, b    string
		refused bool
	}{
		{"the same issuer", "https://sso.example.com/realms/staff", "https://sso.example.com/realms/staff", true},
		{"a trailing slash", "https://sso.example.com/realms/staff", "https://sso.example.com/realms/staff/", true},
		{"another case in the host", "https://sso.example.com/realms/staff", "https://SSO.Example.com/realms/staff", true},
		{"another realm", "https://sso.example.com/realms/staff", "https://sso.example.com/realms/guests", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vars := map[string]string{config.EnvOIDCProviders: "corp,second"}
			for k, v := range provider("corp", tt.a) {
				vars[k] = v
			}
			for k, v := range provider("second", tt.b) {
				vars[k] = v
			}
			_, err := config.Load(env(withOIDC(vars)))
			if !tt.refused {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), "DHOW_OIDC_CORP_ISSUER and DHOW_OIDC_SECOND_ISSUER")
		})
	}
	github := withOIDC(map[string]string{
		config.EnvOIDCProviders: "one,two", "DHOW_OIDC_ONE_TYPE": "github", "DHOW_OIDC_TWO_TYPE": "github",
		"DHOW_OIDC_ONE_CLIENT_ID": "a", "DHOW_OIDC_ONE_CLIENT_SECRET": oidcTestSecret,
		"DHOW_OIDC_TWO_CLIENT_ID": "b", "DHOW_OIDC_TWO_CLIENT_SECRET": oidcTestSecret,
	})
	_, err := config.Load(env(github))
	assert.Error(t, err, "two GitHub providers on github.com share an issuer too")
}
