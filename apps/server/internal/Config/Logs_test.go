package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
)

func TestLogSettings(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		want    config.Logs
		wantErr string
	}{
		{"unset takes the defaults", nil, config.Logs{MaxBytes: 64 << 20, RetentionDays: 30, MaxStreams: 2000, ReadConns: 4}, ""},
		{"every value inside its bounds", map[string]string{
			config.EnvJobLogMaxBytes: "1048576", config.EnvLogRetentionDays: "3650", config.EnvSSEMaxStreams: "1",
			config.EnvLogReadConns: "20",
		}, config.Logs{MaxBytes: 1 << 20, RetentionDays: 3650, MaxStreams: 1, ReadConns: 20}, ""},
		{"no log read connection fails closed", map[string]string{config.EnvLogReadConns: "0"}, config.Logs{}, config.EnvLogReadConns},
		{"more log read connections than the main pool fails closed", map[string]string{config.EnvLogReadConns: "21"}, config.Logs{}, config.EnvLogReadConns},
		{"the cap stops at 1 GiB", map[string]string{config.EnvJobLogMaxBytes: "1073741825"}, config.Logs{}, config.EnvJobLogMaxBytes},
		{"the cap starts at 1 MiB", map[string]string{config.EnvJobLogMaxBytes: "65536"}, config.Logs{}, config.EnvJobLogMaxBytes},
		{"no unit suffixes", map[string]string{config.EnvJobLogMaxBytes: "64MiB"}, config.Logs{}, config.EnvJobLogMaxBytes},
		{"retention of zero days fails closed", map[string]string{config.EnvLogRetentionDays: "0"}, config.Logs{}, config.EnvLogRetentionDays},
		{"retention past ten years fails closed", map[string]string{config.EnvLogRetentionDays: "3651"}, config.Logs{}, config.EnvLogRetentionDays},
		{"no streams at all fails closed", map[string]string{config.EnvSSEMaxStreams: "0"}, config.Logs{}, config.EnvSSEMaxStreams},
		{"garbage stream limit fails closed", map[string]string{config.EnvSSEMaxStreams: "lots"}, config.Logs{}, config.EnvSSEMaxStreams},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := valid()
			for k, v := range tt.env {
				m[k] = v
			}
			cfg, err := config.Load(env(m))
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, cfg.Logs)
		})
	}
}

func TestTheOrganizationStreamShareStaysWellBelowTheInstanceLimit(t *testing.T) {
	assert.Equal(t, 200, config.DefaultLogs().OrgStreams())
	assert.Equal(t, 2*config.LogStreamsPerUser, config.Logs{MaxStreams: 40}.OrgStreams(), "two users of one organization always fit")
	assert.Less(t, config.Logs{MaxStreams: 100_000}.OrgStreams(), 100_000)
}

func TestLogAppendLimitMatchesTheRunnerBodyLimit(t *testing.T) {
	assert.Equal(t, config.RunnerBodyMaxBytes, config.LogAppendMaxBytes)
	assert.Less(t, config.LogChunkMaxBytes*16, config.LogAppendMaxBytes+1)
}
