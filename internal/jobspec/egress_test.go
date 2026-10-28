package jobspec_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Khaymah/dhow/internal/jobspec"
)

func TestParseEgressAcceptsHostsPortsAndWildcards(t *testing.T) {
	tests := []struct {
		entry string
		want  jobspec.EgressRule
	}{
		{"registry.npmjs.org", jobspec.EgressRule{Host: "registry.npmjs.org"}},
		{"proxy.golang.org:8443", jobspec.EgressRule{Host: "proxy.golang.org", Port: 8443}},
		{"*.github.com", jobspec.EgressRule{Host: "github.com", Wildcard: true}},
		{"*.github.com:22", jobspec.EgressRule{Host: "github.com", Wildcard: true, Port: 22}},
		{"xn--mgbh0fb.xn--wgbh1c", jobspec.EgressRule{Host: "xn--mgbh0fb.xn--wgbh1c"}},
		{"a-b.c1", jobspec.EgressRule{Host: "a-b.c1"}},
	}
	for _, tt := range tests {
		got, err := jobspec.ParseEgress(tt.entry)
		require.NoError(t, err, tt.entry)
		assert.Equal(t, tt.want, got, tt.entry)
	}
}

func TestParseEgressRefusesWhatCouldBeAnAddressOrAmbiguous(t *testing.T) {
	tests := []struct {
		entry string
		want  error
	}{
		{"", jobspec.ErrEgressEmpty},
		{"localhost", jobspec.ErrEgressSingle},
		{"*.com", jobspec.ErrEgressSingle},
		{"kubernetes:443", jobspec.ErrEgressSingle},
		{strings.Repeat("a", 300), jobspec.ErrEgressTooLong},
		{"1.2.3.4", jobspec.ErrEgressIP},
		{"2130706433", jobspec.ErrEgressIP},
		{"0x7f.1", jobspec.ErrEgressIP},
		{"10.0.0.1:443", jobspec.ErrEgressIP},
		{"[::1]:443", jobspec.ErrEgressPort},
		{"::1", jobspec.ErrEgressPort},
		{"host:0", jobspec.ErrEgressPort},
		{"host:65536", jobspec.ErrEgressPort},
		{"host:080", jobspec.ErrEgressPort},
		{"host:", jobspec.ErrEgressPort},
		{"host:1:2", jobspec.ErrEgressPort},
		{"HOST.example.com", jobspec.ErrEgressHostname},
		{"example.com.", jobspec.ErrEgressHostname},
		{".example.com", jobspec.ErrEgressHostname},
		{"*.", jobspec.ErrEgressHostname},
		{"*", jobspec.ErrEgressHostname},
		{"*.*.example.com", jobspec.ErrEgressHostname},
		{"a.*.example.com", jobspec.ErrEgressHostname},
		{"مثال.مصر", jobspec.ErrEgressHostname},
		{"-bad.example.com", jobspec.ErrEgressHostname},
		{"bad-.example.com", jobspec.ErrEgressHostname},
		{"under_score.example.com", jobspec.ErrEgressHostname},
		{strings.Repeat("a", 64) + ".com", jobspec.ErrEgressHostname},
		{"exa mple.com", jobspec.ErrEgressHostname},
	}
	for _, tt := range tests {
		_, err := jobspec.ParseEgress(tt.entry)
		assert.ErrorIs(t, err, tt.want, tt.entry)
	}
}

func TestAnEgressRuleMatchesOnlyItsHostAndPort(t *testing.T) {
	exact, err := jobspec.ParseEgress("registry.npmjs.org")
	require.NoError(t, err)
	wild, err := jobspec.ParseEgress("*.github.com")
	require.NoError(t, err)
	ssh, err := jobspec.ParseEgress("git.example.com:22")
	require.NoError(t, err)
	tests := []struct {
		name   string
		rule   jobspec.EgressRule
		host   string
		port   int
		tunnel bool
		want   bool
	}{
		{"exact host on 443 through CONNECT", exact, "registry.npmjs.org", 443, true, true},
		{"exact host on 80 in plain HTTP", exact, "registry.npmjs.org", 80, false, true},
		{"80 through CONNECT is not the default", exact, "registry.npmjs.org", 80, true, false},
		{"443 in plain HTTP is not the default", exact, "registry.npmjs.org", 443, false, false},
		{"another port", exact, "registry.npmjs.org", 8080, true, false},
		{"a subdomain of an exact host", exact, "evil.registry.npmjs.org", 443, true, false},
		{"a suffix that is not a label", exact, "xregistry.npmjs.org", 443, true, false},
		{"a wildcard subdomain", wild, "api.github.com", 443, true, true},
		{"a deep wildcard subdomain", wild, "a.b.github.com", 443, true, true},
		{"the wildcard apex", wild, "github.com", 443, true, false},
		{"a wildcard look-alike", wild, "evilgithub.com", 443, true, false},
		{"an explicit port", ssh, "git.example.com", 22, true, true},
		{"an explicit port does not open 443", ssh, "git.example.com", 443, true, false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.rule.Allows(tt.host, tt.port, tt.tunnel), tt.name)
	}
}

func TestNormalizeHostLowercasesAndDropsOneTrailingDot(t *testing.T) {
	got, err := jobspec.NormalizeHost("Registry.NPMJS.org.")
	require.NoError(t, err)
	assert.Equal(t, "registry.npmjs.org", got)
	for _, bad := range []string{"127.0.0.1", "[::1]", "a..b", "example.com..", "", "0x7f000001"} {
		_, err := jobspec.NormalizeHost(bad)
		assert.Error(t, err, bad)
	}
}

func FuzzAllowlist(f *testing.F) {
	for _, seed := range []string{"registry.npmjs.org", "*.github.com:443", "1.2.3.4", "[::1]:22", "a:b:c", "*.", "xn--a.b", "0x7f.1"} {
		f.Add(seed, "api.github.com", 443)
	}
	f.Fuzz(func(t *testing.T, entry, host string, port int) {
		rule, err := jobspec.ParseEgress(entry)
		if err != nil {
			return
		}
		if rule.Host == "" || strings.ContainsAny(rule.Host, ":/[] *") {
			t.Fatalf("parsed %q into an unsafe host %q", entry, rule.Host)
		}
		if _, err := jobspec.NormalizeHost(rule.Host); err != nil {
			t.Fatalf("parsed %q into a host that does not normalize: %v", entry, err)
		}
		normalized, err := jobspec.NormalizeHost(host)
		if err != nil {
			return
		}
		if rule.Allows(normalized, port, true) {
			if port != rule.Port && (rule.Port != 0 || port != jobspec.DefaultTLSPort) {
				t.Fatalf("%q allowed port %d", entry, port)
			}
			if normalized != rule.Host && !strings.HasSuffix(normalized, "."+rule.Host) {
				t.Fatalf("%q allowed the unrelated host %q", entry, normalized)
			}
			if !rule.Wildcard && normalized != rule.Host {
				t.Fatalf("%q allowed the subdomain %q", entry, normalized)
			}
		}
	})
}
