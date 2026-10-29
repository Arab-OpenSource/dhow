package config

import (
	"time"

	settingspec "github.com/Khaymah/dhow/apps/server/internal/Config/Settings"
)

const (
	InstanceSettingsSealLabel = "dhow/instance-settings/v1"
	SettingsReloadRetryMin    = time.Second
	SettingsReloadRetryMax    = 30 * time.Second
)

func loadSettings(getenv func(string) string) (settingspec.Raw, []error) {
	raw, fieldErrs := settingspec.FromEnv(getenv, func(k settingspec.Key) (string, error) {
		if k.Env != EnvSMTPPassword {
			return getenv(k.Env), nil
		}
		return secretValue(getenv, EnvSMTPPassword, CredentialSMTPPassword)
	})
	var errs []error
	for _, e := range fieldErrs {
		errs = append(errs, e)
	}
	for _, k := range settingspec.Keys() {
		v, ok := raw[k.Env]
		if !ok {
			continue
		}
		if e := settingspec.ValidateValue(k.Env, v); e != nil {
			errs = append(errs, e)
		}
	}
	if len(errs) > 0 {
		return nil, errs
	}
	return raw, nil
}

func (c *Config) SetEnvSetting(env, value string) {
	if c.Settings == nil {
		c.Settings = settingspec.Raw{}
	}
	c.Settings[env] = value
}
