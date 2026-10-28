package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
)

func TestRunnerMaxPolls(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    int
		wantErr bool
	}{
		{"unset defaults to 25 per database connection", "", 500, false},
		{"a small instance", "50", 50, false},
		{"the upper bound", "10000", config.RunnerMaxPollsUpperBound, false},
		{"zero fails closed", "0", 0, true},
		{"negative fails closed", "-1", 0, true},
		{"above the bound fails closed", "10001", 0, true},
		{"garbage fails closed", "many", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := valid()
			m[config.EnvRunnerMaxPolls] = tt.value
			cfg, err := config.Load(env(m))
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), config.EnvRunnerMaxPolls)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, cfg.RunnerMaxPolls)
		})
	}
}
