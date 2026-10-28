package auth_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	auth "github.com/Khaymah/dhow/apps/server/internal/Core/Auth"
)

func TestTokensHaveOneSpellingOnly(t *testing.T) {
	good, _, err := auth.NewSessionToken(32)
	require.NoError(t, err)
	_, ok := auth.HashSessionToken(good, 32)
	require.True(t, ok)
	for name, v := range map[string]string{
		"a newline inside":   good[:10] + "\n" + good[10:],
		"a carriage return":  good[:10] + "\r" + good[10:],
		"padding":            good + "=",
		"a trailing newline": good + "\n",
	} {
		_, ok := auth.HashSessionToken(v, 32)
		assert.False(t, ok, name)
	}

	tokens, err := auth.NewAPITokens([]byte(strings.Repeat("k", 32)))
	require.NoError(t, err)
	pat, _, err := tokens.New()
	require.NoError(t, err)
	_, ok = tokens.Hash(pat)
	require.True(t, ok)
	rest := pat[len(config.APITokenPrefix):]
	_, ok = tokens.Hash(config.APITokenPrefix + rest[:5] + "\n" + rest[5:])
	assert.False(t, ok, "Go's base64 skips newlines, so the length is checked first")
}
