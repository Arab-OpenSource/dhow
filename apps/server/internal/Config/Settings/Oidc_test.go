package settingspec_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	settingspec "github.com/Khaymah/dhow/apps/server/internal/Config/Settings"
)

func validProvider() settingspec.OIDCProvider {
	return settingspec.OIDCProvider{
		ID: "corp", Kind: "oidc", Name: "Corp", Issuer: "https://sso.example.com/realms/staff", ClientID: "dhow",
		ClientSecret: "Zq3vT8wLp2Rk9sXy4Nb7Hc1Jd6Fg0Ma5", AllowedDomains: []string{"Example.com", "example.com"},
	}
}

func TestTheDashboardRunsTheSameProviderRulesAsTheEnvironment(t *testing.T) {
	tests := []struct {
		name   string
		change func(p *settingspec.OIDCProvider)
		dev    bool
		rule   string
	}{
		{"valid", func(*settingspec.OIDCProvider) {}, false, ""},
		{"an id with a dash", func(p *settingspec.OIDCProvider) { p.ID = "corp-1" }, false, settingspec.RuleOIDCID},
		{"an id too long", func(p *settingspec.OIDCProvider) { p.ID = "a234567890123456789012345678901234" }, false, settingspec.RuleOIDCID},
		{"saml", func(p *settingspec.OIDCProvider) { p.Kind = "saml" }, false, settingspec.RuleOIDCType},
		{"a long name", func(p *settingspec.OIDCProvider) { p.Name = "12345678901234567890123456789012345678901" }, false, settingspec.RuleOIDCName},
		{"plain http", func(p *settingspec.OIDCProvider) { p.Issuer = "http://sso.example.com" }, false, settingspec.RuleOIDCIssuerHTTPS},
		{"plain http in dev", func(p *settingspec.OIDCProvider) { p.Issuer = "http://localhost:8180" }, true, ""},
		{"a fragment", func(p *settingspec.OIDCProvider) { p.Issuer = "https://sso.example.com#x" }, false, settingspec.RuleOIDCIssuerParts},
		{"an empty query", func(p *settingspec.OIDCProvider) { p.Issuer = "https://sso.example.com?" }, false, settingspec.RuleOIDCIssuerParts},
		{"link-local", func(p *settingspec.OIDCProvider) { p.Issuer = "https://169.254.10.10" }, false, settingspec.RuleOIDCIssuerHost},
		{"metadata in dev", func(p *settingspec.OIDCProvider) { p.Issuer = "http://metadata.google.internal" }, true, settingspec.RuleOIDCIssuerHost},
		{"a client id with a space", func(p *settingspec.OIDCProvider) { p.ClientID = "a b" }, false, settingspec.RuleOIDCClientID},
		{"a sample secret", func(p *settingspec.OIDCProvider) { p.ClientSecret = "replace-this-value" }, false, settingspec.RuleOIDCPlaceholder},
		{"a wildcard domain", func(p *settingspec.OIDCProvider) { p.AllowedDomains = []string{"*.example.com"} }, false, settingspec.RuleOIDCDomain},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := validProvider()
			tt.change(&p)
			got, errs := settingspec.ValidateOIDCProvider(p, tt.dev, true)
			if tt.rule == "" {
				require.Empty(t, errs)
				assert.Equal(t, []string{"example.com"}, got.AllowedDomains)
				return
			}
			require.Len(t, errs, 1)
			assert.Equal(t, tt.rule, errs[0].Rule)
			assert.NotContains(t, errs[0].Message, "Zq3vT8wLp2Rk9sXy4Nb7Hc1Jd6Fg0Ma5")
		})
	}
}

func TestASecretIsOptionalOnlyWhenTheCallerSaysSo(t *testing.T) {
	p := validProvider()
	p.ClientSecret = ""
	_, errs := settingspec.ValidateOIDCProvider(p, false, false)
	assert.Empty(t, errs, "a provider without a secret is stored, turned off")
	_, errs = settingspec.ValidateOIDCProvider(p, false, true)
	require.Len(t, errs, 1)
	assert.Equal(t, settingspec.OIDCFieldClientSecret, errs[0].Field)
}

func TestIssuersAreComparedWithoutCaseOrATrailingSlash(t *testing.T) {
	a := settingspec.OIDCProvider{ID: "a", Issuer: "https://SSO.example.com/realms/x/"}
	b := settingspec.OIDCProvider{ID: "b", Issuer: "https://sso.example.com/realms/x"}
	c := settingspec.OIDCProvider{ID: "c", Issuer: "https://sso.example.com/realms/y"}
	first, second, found := settingspec.SharedIssuer([]settingspec.OIDCProvider{a, c, b})
	require.True(t, found)
	assert.Equal(t, "a", first.ID)
	assert.Equal(t, "b", second.ID)
	_, _, found = settingspec.SharedIssuer([]settingspec.OIDCProvider{a, c})
	assert.False(t, found)
}
