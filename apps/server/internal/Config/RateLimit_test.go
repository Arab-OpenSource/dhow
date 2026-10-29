package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
)

func TestRateLimitsDefaultWhenUnset(t *testing.T) {
	cfg, err := resolve(valid())
	require.NoError(t, err)
	assert.Equal(t, config.RateLimits{
		Public: config.RateLimitPublicDefault, InvitesUser: config.RateLimitInvitesUserDefault, InvitesOrg: config.RateLimitInvitesOrgDefault,
	}, cfg.RateLimits)
}

func TestRateLimitValues(t *testing.T) {
	names := []string{config.EnvRateLimitPublic, config.EnvRateLimitInvitesUser, config.EnvRateLimitInvitesOrg}
	tests := []struct {
		name    string
		value   string
		want    int
		wantErr bool
	}{
		{"one is the smallest", "1", 1, false},
		{"surrounding spaces are fine", " 40 ", 40, false},
		{"the upper bound", "100000", config.RateLimitUpperBound, false},
		{"zero would turn the limit off, so it fails closed", "0", 0, true},
		{"negative fails closed", "-5", 0, true},
		{"above the bound fails closed", "100001", 0, true},
		{"garbage fails closed", "lots", 0, true},
		{"a duration is not a count", "10m", 0, true},
	}
	for _, name := range names {
		for _, tt := range tests {
			t.Run(name+"/"+tt.name, func(t *testing.T) {
				m := valid()
				m[name] = tt.value
				cfg, err := resolve(m)
				if tt.wantErr {
					require.Error(t, err)
					assert.Contains(t, err.Error(), name)
					return
				}
				require.NoError(t, err)
				got := map[string]int{
					config.EnvRateLimitPublic:      cfg.RateLimits.Public,
					config.EnvRateLimitInvitesUser: cfg.RateLimits.InvitesUser,
					config.EnvRateLimitInvitesOrg:  cfg.RateLimits.InvitesOrg,
				}
				assert.Equal(t, tt.want, got[name])
			})
		}
	}
}

func TestRateLimitsWithDefaultsFillsOnlyTheMissingOnes(t *testing.T) {
	got := config.RateLimits{InvitesUser: 3}.WithDefaults()
	assert.Equal(t, config.RateLimits{Public: config.RateLimitPublicDefault, InvitesUser: 3, InvitesOrg: config.RateLimitInvitesOrgDefault}, got)
}
