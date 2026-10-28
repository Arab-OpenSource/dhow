package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
)

func TestLocale(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"none", map[string]string{}, ""},
		{"lang", map[string]string{"LANG": "ar_EG.UTF-8"}, "ar_EG.UTF-8"},
		{"messages wins over lang", map[string]string{"LANG": "en_US.UTF-8", "LC_MESSAGES": "ar_EG.UTF-8"}, "ar_EG.UTF-8"},
		{"lc_all wins", map[string]string{"LANG": "ar_EG.UTF-8", "LC_ALL": "C"}, "C"},
		{"blank is skipped", map[string]string{"LC_ALL": " ", "LANG": "en_GB.UTF-8"}, "en_GB.UTF-8"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, config.Locale(env(tt.env)))
		})
	}
}

func TestExportedValidators(t *testing.T) {
	assert.NoError(t, config.ValidateSecretKey(goodKeyHex))
	assert.Error(t, config.ValidateSecretKey("changeme"))
	assert.NoError(t, config.ValidateDatabaseURL(goodDB))
	assert.NoError(t, config.ValidateDatabaseURL("postgres://dhow:pw@/dhow?host=/var/run/postgresql&sslmode=require"))
	assert.Error(t, config.ValidateDatabaseURL("postgres://dhow:pw@db/dhow?sslmode=disable"))
	assert.NoError(t, config.ValidatePublicURL("https://dhow.example.com"))
	assert.Error(t, config.ValidatePublicURL("http://dhow.example.com"))
	assert.NoError(t, config.ValidateTrustedProxies("127.0.0.1/32,::1"))
	assert.Error(t, config.ValidateTrustedProxies("nope"))
	assert.NoError(t, config.ValidateListenAddr("127.0.0.1:8080"))
	assert.Error(t, config.ValidateListenAddr("8080"))
}
