package auth_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	auth "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Auth"
)

func TestCSRFTokenIsBoundToTheSessionAndTheServerKey(t *testing.T) {
	a := auth.NewCSRF([]byte("0123456789abcdef0123456789abcdef"))
	b := auth.NewCSRF([]byte("fedcba9876543210fedcba9876543210"))
	assert.Equal(t, a.Token("session-1"), a.Token("session-1"))
	assert.NotEqual(t, a.Token("session-1"), a.Token("session-2"))
	assert.NotEqual(t, a.Token("session-1"), b.Token("session-1"))
	assert.NotContains(t, a.Token("session-1"), "session-1")
}

func TestCSRFVerify(t *testing.T) {
	c := auth.NewCSRF([]byte("0123456789abcdef0123456789abcdef"))
	token := c.Token("session-1")
	want := c.Hash(token)
	tests := []struct {
		name      string
		presented string
		want      bool
	}{
		{"matching token", token, true},
		{"token of another session", c.Token("session-2"), false},
		{"empty", "", false},
		{"truncated", token[:len(token)-1], false},
		{"padded", token + "A", false},
		{"upper-cased", func() string { return string([]byte(token)[:1]) + "x" + token[2:] }(), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, c.Verify(tt.presented, want))
		})
	}
	assert.False(t, c.Verify(token, nil))
	assert.False(t, c.Verify(token, []byte("short")))
}

func TestSessionTokens(t *testing.T) {
	raw, hash, err := auth.NewSessionToken(32)
	require.NoError(t, err)
	assert.Len(t, hash, 32)
	assert.Len(t, raw, 43)
	again, ok := auth.HashSessionToken(raw, 32)
	require.True(t, ok)
	assert.Equal(t, hash, again)

	other, otherHash, err := auth.NewSessionToken(32)
	require.NoError(t, err)
	assert.NotEqual(t, raw, other)
	assert.NotEqual(t, hash, otherHash)

	for name, bad := range map[string]string{"empty": "", "not base64": "!!!", "too short": raw[:20], "too long": raw + "AAAA", "padded": raw + "="} {
		_, ok := auth.HashSessionToken(bad, 32)
		assert.False(t, ok, name)
	}
}

func TestCookieAttributes(t *testing.T) {
	set := auth.SessionCookie("__Host-dhow_session", "abc", 3600e9)
	for _, want := range []string{"__Host-dhow_session=abc", "Path=/", "Max-Age=3600", "HttpOnly", "Secure", "SameSite=Strict"} {
		assert.Contains(t, set, want)
	}
	assert.NotContains(t, set, "Domain")
	cleared := auth.ClearedSessionCookie("__Host-dhow_session")
	for _, want := range []string{"__Host-dhow_session=", "Path=/", "Max-Age=0", "HttpOnly", "Secure", "SameSite=Strict"} {
		assert.Contains(t, cleared, want)
	}
}

func TestEmailAndNames(t *testing.T) {
	assert.Equal(t, "mona@example.com", auth.NormalizeEmail("  Mona@Example.COM "))
	valid := []string{"mona@example.com", "a.b+tag@sub.example.org"}
	for _, v := range valid {
		assert.True(t, auth.ValidEmail(v), v)
	}
	invalid := []string{"", "no-at-sign", "a@b@c.com", "Mona <m@example.com>", "m@example.com, x@example.com", "a b@example.com", `"quoted"@example.com`, "@example.com", string(make([]byte, 300)) + "@example.com"}
	for _, v := range invalid {
		assert.False(t, auth.ValidEmail(v), v)
	}
	assert.True(t, auth.ValidDisplayName("منى", 100))
	assert.False(t, auth.ValidDisplayName("", 100))
	assert.False(t, auth.ValidDisplayName(" padded", 100))
	assert.False(t, auth.ValidDisplayName("tab\there", 100))
	assert.False(t, auth.ValidDisplayName("too long", 3))
}
