package auth_test

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	auth "github.com/Khaymah/dhow/apps/server/internal/Core/Auth"
)

var rfcSecret = []byte("12345678901234567890")

func TestHOTPMatchesTheRFC4226Vectors(t *testing.T) {
	want := []string{"755224", "287082", "359152", "969429", "338314", "254676", "287922", "162583", "399871", "520489"}
	for counter, code := range want {
		assert.Equal(t, code, auth.HOTP(rfcSecret, uint64(counter), 6), "counter %d", counter)
	}
}

func TestTOTPMatchesTheRFC6238SHA1Vectors(t *testing.T) {
	tests := []struct {
		unix int64
		code string
	}{
		{59, "94287082"},
		{1111111109, "07081804"},
		{1111111111, "14050471"},
		{1234567890, "89005924"},
		{2000000000, "69279037"},
		{20000000000, "65353130"},
	}
	for _, tt := range tests {
		step := auth.TotpStep(time.Unix(tt.unix, 0))
		assert.Equal(t, tt.code, auth.HOTP(rfcSecret, uint64(step), 8), "T=%d", tt.unix)
		assert.Equal(t, tt.code[2:], auth.TotpCodeAt(rfcSecret, step), "the 6-digit code is the same value mod 10^6")
	}
}

func TestMatchTotpAllowsOneStepOfSkewOnly(t *testing.T) {
	now := time.Unix(1111111111, 0)
	step := auth.TotpStep(now)
	tests := []struct {
		name   string
		offset int64
		ok     bool
	}{
		{"previous step", -1, true},
		{"current step", 0, true},
		{"next step", 1, true},
		{"two steps back", -2, false},
		{"two steps ahead", 2, false},
		{"ten steps back", -10, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := auth.MatchTotp(rfcSecret, auth.TotpCodeAt(rfcSecret, step+tt.offset), now, auth.NoTotpStep)
			assert.Equal(t, tt.ok, ok)
			if tt.ok {
				assert.Equal(t, step+tt.offset, got)
			}
		})
	}
}

func TestMatchTotpRefusesStepsAtOrBeforeTheLastAcceptedOne(t *testing.T) {
	now := time.Unix(1234567890, 0)
	step := auth.TotpStep(now)
	code := auth.TotpCodeAt(rfcSecret, step)
	_, ok := auth.MatchTotp(rfcSecret, code, now, step)
	assert.False(t, ok, "a replayed code is refused")
	_, ok = auth.MatchTotp(rfcSecret, auth.TotpCodeAt(rfcSecret, step-1), now, step)
	assert.False(t, ok, "an older code is refused after a newer one was used")
	got, ok := auth.MatchTotp(rfcSecret, auth.TotpCodeAt(rfcSecret, step+1), now, step)
	assert.True(t, ok, "the next step is still accepted")
	assert.Equal(t, step+1, got)
}

func TestMatchTotpRejectsMalformedCodes(t *testing.T) {
	now := time.Unix(59, 0)
	for _, code := range []string{"", "12345", "1234567", "abcdef", "12 34 5x", "١٢٣٤٥٦"} {
		_, ok := auth.MatchTotp(rfcSecret, code, now, auth.NoTotpStep)
		assert.False(t, ok, code)
	}
	code := auth.TotpCodeAt(rfcSecret, auth.TotpStep(now))
	_, ok := auth.MatchTotp(rfcSecret, code[:3]+" "+code[3:], now, auth.NoTotpStep)
	assert.True(t, ok, "spaces inside the code are ignored")
}

func TestTotpSecretAndURI(t *testing.T) {
	secret, err := auth.NewTotpSecret()
	require.NoError(t, err)
	assert.Len(t, secret, auth.TotpSecretBytes)
	other, err := auth.NewTotpSecret()
	require.NoError(t, err)
	assert.NotEqual(t, secret, other)

	text := auth.TotpSecretText(secret)
	assert.Len(t, text, 32)
	assert.NotContains(t, text, "=")

	uri, err := url.Parse(auth.TotpURI("Dhow", "owner@example.com", secret))
	require.NoError(t, err)
	assert.Equal(t, "otpauth", uri.Scheme)
	assert.Equal(t, "totp", uri.Host)
	assert.True(t, strings.HasPrefix(uri.Path, "/Dhow:owner@example.com"), uri.Path)
	assert.Equal(t, text, uri.Query().Get("secret"))
	assert.Equal(t, "Dhow", uri.Query().Get("issuer"))
	assert.Equal(t, "6", uri.Query().Get("digits"))
	assert.Equal(t, "30", uri.Query().Get("period"))
}
