package settingspec_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	settingspec "github.com/Khaymah/dhow/apps/server/internal/Config/Settings"
)

func TestTheSMTPHostRefusesLinkLocalAndMetadataButKeepsLocalRelays(t *testing.T) {
	tests := []struct {
		host    string
		blocked bool
	}{
		{"169.254.169.254", true},
		{"169.254.0.1", true},
		{"fe80::1", true},
		{"[fe80::1]", true},
		{"::ffff:169.254.169.254", true},
		{"fd00:ec2::254", true},
		{"100.100.100.200", true},
		{"metadata.google.internal", true},
		{"METADATA.GOOGLE.INTERNAL.", true},
		{"metadata", true},
		{"instance-data.ec2.internal", true},
		{"127.0.0.1", false},
		{"localhost", false},
		{"10.0.0.25", false},
		{"192.168.1.10", false},
		{"172.16.5.5", false},
		{"smtp.example.com", false},
		{"metadata.example.com", false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.blocked, settingspec.BlockedSMTPHost(tt.host), tt.host)
		if tt.blocked && settingspec.ValidateValue(settingspec.EnvSMTPHost, tt.host) == nil {
			assert.Contains(t, tt.host, ":", "a host without a colon is refused by the shared validator: %s", tt.host)
		}
	}
}

func TestOnlyLoopbackHostsCountAsThisMachine(t *testing.T) {
	for host, want := range map[string]bool{
		"localhost": true, "LOCALHOST.": true, "mail.localhost": true, "127.0.0.1": true, "127.8.9.10": true, "::1": true,
		"10.0.0.1": false, "relay.internal": false, "localhost.example.com": false, "": false,
	} {
		assert.Equal(t, want, settingspec.LoopbackHost(host), host)
	}
}
