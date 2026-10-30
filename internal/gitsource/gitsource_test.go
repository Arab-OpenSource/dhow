package gitsource

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseNormalizesTheAcceptedForms(t *testing.T) {
	tests := []struct {
		raw       string
		canonical string
		transport Transport
		hostPort  string
		fullName  string
		known     string
	}{
		{"https://git.example.com/team/app.git", "https://git.example.com/team/app.git", TransportHTTPS, "git.example.com:443", "git.example.com/team/app", ""},
		{"https://Git.Example.COM/team/app", "https://git.example.com/team/app", TransportHTTPS, "git.example.com:443", "git.example.com/team/app", ""},
		{"https://git.example.com:8443/team/app.git", "https://git.example.com:8443/team/app.git", TransportHTTPS, "git.example.com:8443", "git.example.com/team/app", ""},
		{"ssh://git@git.example.com:2222/team/app.git", "ssh://git@git.example.com:2222/team/app.git", TransportSSH, "git.example.com:2222", "git.example.com/team/app", "[git.example.com]:2222"},
		{"ssh://git.example.com/team/app.git", "ssh://git.example.com/team/app.git", TransportSSH, "git.example.com:22", "git.example.com/team/app", "git.example.com"},
		{"git@git.example.com:team/app.git", "ssh://git@git.example.com/team/app.git", TransportSSH, "git.example.com:22", "git.example.com/team/app", "git.example.com"},
		{"forgejo@code.example.org:/srv/git/app.git", "ssh://forgejo@code.example.org/srv/git/app.git", TransportSSH, "code.example.org:22", "code.example.org/srv/git/app", "code.example.org"},
		{"https://xn--mgbh0fb.example/app.git", "https://xn--mgbh0fb.example/app.git", TransportHTTPS, "xn--mgbh0fb.example:443", "xn--mgbh0fb.example/app", ""},
		{"https://git.example.com/team/my%2Bapp.git", "https://git.example.com/team/my%2Bapp.git", TransportHTTPS, "git.example.com:443", "git.example.com/team/my%2Bapp", ""},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			u, err := Parse(tt.raw)
			require.NoError(t, err)
			assert.Equal(t, tt.canonical, u.String())
			assert.Equal(t, tt.transport, u.Transport)
			assert.Equal(t, tt.hostPort, u.HostPort())
			assert.Equal(t, tt.fullName, u.FullName())
			assert.Equal(t, tt.known, u.KnownHostsPattern())
			again, err := Parse(u.String())
			require.NoError(t, err)
			assert.Equal(t, u, again)
		})
	}
}

func TestParseRefusesDangerousAddresses(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want error
	}{
		{"empty", "", ErrEmpty},
		{"too long", "https://git.example.com/" + strings.Repeat("a", MaxURLBytes), ErrTooLong},
		{"ext transport", "ext::sh -c touch% /tmp/pwned", ErrCharacter},
		{"ext transport without spaces", "ext::ssh%20-oProxyCommand=x", ErrScheme},
		{"fd transport", "fd::17", ErrScheme},
		{"a file url", "file:///etc/passwd", ErrScheme},
		{"a local path", "/srv/git/app.git", ErrScheme},
		{"a relative path", "../app.git", ErrScheme},
		{"plain http", "http://git.example.com/app.git", ErrScheme},
		{"git protocol", "git://git.example.com/app.git", ErrScheme},
		{"an ssh option as the address", "-oProxyCommand=touch /tmp/x", ErrCharacter},
		{"an ssh option without spaces", "-oProxyCommand=x", ErrScheme},
		{"an upload-pack option", "--upload-pack=touch", ErrScheme},
		{"an option as the host", "ssh://-oProxyCommand=x/app.git", ErrHost},
		{"an option as the scp host", "git@-oProxyCommand=x:app.git", ErrHost},
		{"an option as the user", "ssh://-oProxyCommand@git.example.com/app.git", ErrUser},
		{"an option as the path", "ssh://git@git.example.com/-oProxyCommand=x", ErrPath},
		{"an option as the scp path", "git@git.example.com:--upload-pack=x", ErrPath},
		{"an upload-pack option as a path segment", "https://git.example.com/a/--upload-pack=x", ErrPath},
		{"a password", "https://user:secret@git.example.com/app.git", ErrUser},
		{"a user on https", "https://user@git.example.com/app.git", ErrUser},
		{"a password on ssh", "ssh://git:secret@git.example.com/app.git", ErrUser},
		{"a loopback address", "https://127.0.0.1/app.git", ErrHost},
		{"a metadata address", "https://169.254.169.254/latest", ErrHost},
		{"a decimal address", "https://2130706433/app.git", ErrHost},
		{"a hex address", "https://0x7f.0x1/app.git", ErrHost},
		{"an ipv6 address", "https://[::1]/app.git", ErrHost},
		{"a single label", "ssh://git@gitea/app.git", ErrHost},
		{"an empty host", "https:///app.git", ErrPath},
		{"a port zero", "https://git.example.com:0/app.git", ErrPort},
		{"a port too big", "https://git.example.com:65536/app.git", ErrPort},
		{"a port with a leading zero", "https://git.example.com:022/app.git", ErrPort},
		{"a parent path", "https://git.example.com/a/../b.git", ErrPath},
		{"an encoded parent path", "https://git.example.com/a/%2e%2e/b.git", ErrPath},
		{"no path", "https://git.example.com/", ErrPath},
		{"a query", "https://git.example.com/app.git?x=1", ErrPath},
		{"a fragment", "https://git.example.com/app.git#x", ErrPath},
		{"a newline", "https://git.example.com/app.git\nevil", ErrCharacter},
		{"a carriage return", "https://git.example.com/app\r.git", ErrCharacter},
		{"a tab", "https://git.example.com/a\tb", ErrCharacter},
		{"a space", "https://git.example.com/a b", ErrCharacter},
		{"a nul byte", "https://git.example.com/a\x00", ErrCharacter},
		{"an encoded newline", "https://git.example.com/app%0a.git", ErrCharacter},
		{"an encoded nul", "https://git.example.com/app%00.git", ErrCharacter},
		{"an encoded space", "https://git.example.com/my%20app.git", ErrCharacter},
		{"an encoded percent", "https://git.example.com/a%25b", ErrCharacter},
		{"a broken escape", "https://git.example.com/a%2", ErrCharacter},
		{"non-ascii", "https://git.example.com/مشروع.git", ErrCharacter},
		{"a backslash", "https://git.example.com/a\\b", ErrCharacter},
		{"a quote", "https://git.example.com/a'b", ErrCharacter},
		{"a semicolon", "https://git.example.com/a;b", ErrCharacter},
		{"a dollar", "https://git.example.com/$(id)", ErrCharacter},
		{"a pipe", "https://git.example.com/a|b", ErrCharacter},
		{"a wildcard host", "https://*.example.com/app.git", ErrHost},
		{"scp without a user", "git.example.com:app.git", ErrScheme},
		{"scp with an empty path", "git@git.example.com:", ErrPath},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.raw)
			require.ErrorIs(t, err, tt.want)
		})
	}
}

func FuzzCloneUrl(f *testing.F) {
	for _, seed := range []string{
		"https://git.example.com/a.git", "ssh://git@git.example.com:2222/a.git", "git@git.example.com:a/b.git",
		"ext::sh", "file:///etc/passwd", "-oProxyCommand=x", "--upload-pack=x", "https://git.example.com/%0a",
		"ssh://git@git.example.com/../x", "https://127.0.0.1/a", "https://u:p@git.example.com/a",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		u, err := Parse(raw)
		if err != nil {
			return
		}
		out := u.String()
		if !strings.HasPrefix(out, "https://") && !strings.HasPrefix(out, "ssh://") {
			t.Fatalf("accepted %q as %q", raw, out)
		}
		if strings.Contains(out, "::") || strings.Contains(out, "/-") || strings.Contains(out, "..") {
			t.Fatalf("accepted a dangerous address %q", out)
		}
		for i := 0; i < len(out); i++ {
			if out[i] <= ' ' || out[i] >= 0x7f {
				t.Fatalf("accepted a control or non-ascii byte in %q", out)
			}
		}
		if u.Transport == TransportHTTPS && u.User != "" {
			t.Fatalf("accepted a user on https in %q", raw)
		}
		again, err := Parse(out)
		if err != nil || again != u {
			t.Fatalf("the canonical form %q does not parse back to %+v: %v", out, u, err)
		}
		args, err := FetchArgs(u, "refs/heads/main", 1)
		if err != nil {
			t.Fatalf("fetch args for an accepted address: %v", err)
		}
		if args[len(args)-3] != "--" || args[len(args)-2] != out {
			t.Fatalf("the address is not after -- in %q", args)
		}
	})
}
