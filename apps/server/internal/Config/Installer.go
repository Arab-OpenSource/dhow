package config

import (
	"os"
	"strings"
)

var localeVariables = []string{"LC_ALL", "LC_MESSAGES", "LANG"}

func LocaleFromEnv() string { return Locale(os.Getenv) }

func Locale(getenv func(string) string) string {
	for _, name := range localeVariables {
		if v := strings.TrimSpace(getenv(name)); v != "" {
			return v
		}
	}
	return ""
}

func ValidateSecretKey(v string) error {
	_, err := parseSecretKey(v)
	return err
}

func ValidateDatabaseURL(v string) error {
	_, err := parseDatabaseURL(v, false)
	return err
}

func ValidatePublicURL(v string) error {
	_, err := parsePublicURL(v, false)
	return err
}

func ValidateTrustedProxies(v string) error {
	_, err := parseTrustedProxies(v)
	return err
}

func ValidateListenAddr(v string) error {
	_, err := parseListenAddr(v)
	return err
}
