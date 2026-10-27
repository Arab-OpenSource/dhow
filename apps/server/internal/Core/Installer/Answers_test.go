package installer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const minimalAnswers = "mode: native\ndomain: dhow.example.com\ndatabase:\n  mode: install\nproxy:\n  kind: caddy\ninstall_packages: true\n"

func TestParseAnswers(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr string
		check   func(t *testing.T, a Answers)
	}{
		{name: "minimal", yaml: minimalAnswers, check: func(t *testing.T, a Answers) {
			assert.Equal(t, "https://dhow.example.com", a.EffectivePublicURL())
			assert.Equal(t, DefaultListen, a.ListenAddr)
			assert.True(t, a.BreachedCheck())
			assert.Equal(t, loopbackProxies, a.TrustedProxies())
		}},
		{name: "normalises case and trailing dots", yaml: "mode: NATIVE\ndomain: Dhow.Example.COM.\ndatabase: {mode: install}\nproxy: {kind: none, trusted_proxies: 10.0.0.5}\n", check: func(t *testing.T, a Answers) {
			assert.Equal(t, "dhow.example.com", a.Domain)
			assert.Equal(t, "10.0.0.5", a.TrustedProxies())
		}},
		{name: "breached check off", yaml: minimalAnswers + "breached_password_check: false\n", check: func(t *testing.T, a Answers) {
			assert.False(t, a.BreachedCheck())
		}},
		{name: "explicit public url", yaml: minimalAnswers + "public_url: https://ci.example.com:8443\n", check: func(t *testing.T, a Answers) {
			assert.Equal(t, "https://ci.example.com:8443", a.EffectivePublicURL())
		}},
		{name: "unknown top-level key", yaml: minimalAnswers + "colour: blue\n", wantErr: "field colour not found"},
		{name: "unknown nested key", yaml: "mode: native\ndomain: dhow.example.com\ndatabase:\n  mode: install\n  password: x\nproxy: {kind: caddy}\n", wantErr: "field password not found"},
		{name: "two documents", yaml: minimalAnswers + "---\nmode: native\n", wantErr: "only one YAML document"},
		{name: "empty", yaml: "", wantErr: "empty"},
		{name: "bad mode", yaml: strings.Replace(minimalAnswers, "mode: native", "mode: podman", 1), wantErr: "mode: must be native or compose"},
		{name: "domain with scheme", yaml: strings.Replace(minimalAnswers, "dhow.example.com", "https://dhow.example.com", 1), wantErr: "domain: must be a host name"},
		{name: "single label domain", yaml: strings.Replace(minimalAnswers, "dhow.example.com", "localhost", 1), wantErr: "domain: must be a host name"},
		{name: "http public url", yaml: minimalAnswers + "public_url: http://dhow.example.com\n", wantErr: "must use https"},
		{name: "bad listen addr", yaml: minimalAnswers + "listen_addr: 8080\n", wantErr: "listen_addr"},
		{name: "nginx without email", yaml: strings.Replace(minimalAnswers, "kind: caddy", "kind: nginx", 1), wantErr: "proxy.email: is required"},
		{name: "bad email", yaml: strings.Replace(minimalAnswers, "kind: caddy", "kind: nginx\n  email: nope", 1), wantErr: "not an email"},
		{name: "unknown proxy", yaml: strings.Replace(minimalAnswers, "kind: caddy", "kind: traefik", 1), wantErr: "proxy.kind"},
		{name: "existing database without url file", yaml: strings.Replace(minimalAnswers, "mode: install", "mode: existing", 1), wantErr: "database.url_file: is required"},
		{name: "url file with install", yaml: strings.Replace(minimalAnswers, "mode: install", "mode: install\n  url_file: /root/x", 1), wantErr: "only applies"},
		{name: "compose with existing database", yaml: "mode: compose\ndomain: dhow.example.com\ndatabase: {mode: existing}\nproxy: {kind: caddy}\n", wantErr: "compose install always runs its own PostgreSQL"},
		{name: "firewall without managed proxy", yaml: "mode: native\ndomain: dhow.example.com\ndatabase: {mode: install}\nproxy: {kind: none}\nfirewall: true\n", wantErr: "firewall: opening ports"},
		{name: "bad trusted proxies", yaml: "mode: native\ndomain: dhow.example.com\ndatabase: {mode: install}\nproxy: {kind: existing, trusted_proxies: 'nope'}\n", wantErr: "proxy.trusted_proxies"},
		{name: "wrong type", yaml: strings.Replace(minimalAnswers, "install_packages: true", "install_packages: maybe", 1), wantErr: "cannot unmarshal"},
		{name: "bad lang", yaml: minimalAnswers + "lang: fr\n", wantErr: "lang: must be en or ar"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, err := ParseAnswers(strings.NewReader(tt.yaml))
			if tt.wantErr != "" {
				require.ErrorIs(t, err, ErrInvalidAnswers)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			tt.check(t, a)
		})
	}
}

func TestParseAnswersRejectsHugeFiles(t *testing.T) {
	_, err := ParseAnswers(strings.NewReader(strings.Repeat("#", maxAnswersBytes+1)))
	require.ErrorIs(t, err, ErrInvalidAnswers)
}

func TestMarshalNeverWritesSecretsAndRoundTrips(t *testing.T) {
	a, err := ParseAnswers(strings.NewReader("mode: native\ndomain: dhow.example.com\ndatabase:\n  mode: existing\n  url_file: /root/db-url\nproxy: {kind: existing, trusted_proxies: 10.0.0.0/24}\n"))
	require.NoError(t, err)
	a.SetDatabaseURL("postgres://dhow:TopSecretPw@db:5432/dhow?sslmode=verify-full")
	data, err := a.Marshal()
	require.NoError(t, err)
	assert.NotContains(t, string(data), "TopSecretPw")
	assert.NotContains(t, string(data), "postgres://")
	again, err := ParseAnswers(strings.NewReader(string(data)))
	require.NoError(t, err)
	assert.Equal(t, a.Domain, again.Domain)
	assert.Equal(t, a.Database.URLFile, again.Database.URLFile)
	assert.Equal(t, a.Proxy, again.Proxy)
	assert.Empty(t, again.DatabaseURL())
}

func TestValidDomain(t *testing.T) {
	for d, want := range map[string]bool{
		"dhow.example.com": true, "a.b": true, "xn--mgbh0fb.example": true, "-bad.example.com": false,
		"bad-.example.com": false, "dhow.example.com/x": false, "dhow.example.com:443": false, "dhow": false, "": false,
		"under_score.example.com": false, strings.Repeat("a", 64) + ".com": false,
	} {
		assert.Equal(t, want, ValidDomain(d), d)
	}
}
