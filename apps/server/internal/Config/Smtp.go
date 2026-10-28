package config

import (
	"errors"
	"fmt"
	"net/mail"
	"strconv"
	"strings"
)

const (
	EnvSMTPHost         = "DHOW_SMTP_HOST"
	EnvSMTPPort         = "DHOW_SMTP_PORT"
	EnvSMTPUsername     = "DHOW_SMTP_USERNAME"
	EnvSMTPPassword     = "DHOW_SMTP_PASSWORD"      //nolint:gosec // The name of an environment variable, not a credential.
	EnvSMTPPasswordFile = "DHOW_SMTP_PASSWORD_FILE" //nolint:gosec // The name of an environment variable, not a credential.
	EnvSMTPFrom         = "DHOW_SMTP_FROM"
	EnvSMTPTLS          = "DHOW_SMTP_TLS"
	EnvOrgCreation      = "DHOW_ORG_CREATION"

	CredentialSMTPPassword = "dhow-smtp-password" //nolint:gosec // The name of a systemd credential, not a credential.

	SMTPTLSStartTLS = "starttls"
	SMTPTLSImplicit = "tls"
	SMTPTLSNone     = "none"

	OrgCreationInstanceOwner = "instance-owner"
	OrgCreationAnyUser       = "any-user"

	defaultSMTPPort        = 587
	defaultSMTPImplicitTLS = 465
	maxSMTPHostLength      = 253
)

type SMTP struct {
	Host     string
	Port     int
	Username string
	Password string
	From     mail.Address
	TLS      string
}

func (s SMTP) Enabled() bool { return s.Host != "" }

func loadSMTP(getenv func(string) string) (SMTP, error) {
	host := strings.TrimSpace(getenv(EnvSMTPHost))
	password, err := secretValue(getenv, EnvSMTPPassword, CredentialSMTPPassword)
	if err != nil {
		return SMTP{}, fmt.Errorf("%s: %w", EnvSMTPPassword, err)
	}
	if host == "" {
		if password != "" || strings.TrimSpace(getenv(EnvSMTPUsername)) != "" || strings.TrimSpace(getenv(EnvSMTPFrom)) != "" {
			return SMTP{}, fmt.Errorf("%s: is required when other DHOW_SMTP_* variables are set", EnvSMTPHost)
		}
		return SMTP{}, nil
	}
	cfg := SMTP{Host: host, Username: strings.TrimSpace(getenv(EnvSMTPUsername)), Password: password}
	var errs []error
	if len(host) > maxSMTPHostLength || strings.ContainsAny(host, " \t\r\n/:@") {
		errs = append(errs, fmt.Errorf("%s: must be a host name or an IP address without a port", EnvSMTPHost))
	}
	if cfg.TLS, err = parseSMTPTLS(getenv(EnvSMTPTLS)); err != nil {
		errs = append(errs, fmt.Errorf("%s: %w", EnvSMTPTLS, err))
	}
	if cfg.Port, err = parseSMTPPort(getenv(EnvSMTPPort), cfg.TLS); err != nil {
		errs = append(errs, fmt.Errorf("%s: %w", EnvSMTPPort, err))
	}
	if cfg.From, err = parseSMTPFrom(getenv(EnvSMTPFrom)); err != nil {
		errs = append(errs, fmt.Errorf("%s: %w", EnvSMTPFrom, err))
	}
	if (cfg.Username == "") != (cfg.Password == "") {
		errs = append(errs, fmt.Errorf("%s and %s: set both or neither", EnvSMTPUsername, EnvSMTPPassword))
	}
	if cfg.TLS == SMTPTLSNone && cfg.Password != "" {
		errs = append(errs, fmt.Errorf("%s: none never sends a password; use starttls or tls with credentials", EnvSMTPTLS))
	}
	if len(errs) > 0 {
		return SMTP{}, errors.Join(errs...)
	}
	return cfg, nil
}

func parseSMTPTLS(v string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", SMTPTLSStartTLS:
		return SMTPTLSStartTLS, nil
	case SMTPTLSImplicit:
		return SMTPTLSImplicit, nil
	case SMTPTLSNone:
		return SMTPTLSNone, nil
	default:
		return "", errors.New("must be starttls, tls or none")
	}
}

func parseSMTPPort(v, tls string) (int, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		if tls == SMTPTLSImplicit {
			return defaultSMTPImplicitTLS, nil
		}
		return defaultSMTPPort, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 || n > 65535 {
		return 0, errors.New("must be a port between 1 and 65535")
	}
	return n, nil
}

func parseSMTPFrom(v string) (mail.Address, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return mail.Address{}, errors.New("is required when DHOW_SMTP_HOST is set, for example Dhow <dhow@example.com>")
	}
	if strings.ContainsAny(v, "\r\n") {
		return mail.Address{}, errors.New("must be one address")
	}
	addr, err := mail.ParseAddress(v)
	if err != nil {
		return mail.Address{}, errors.New("must be an address such as dhow@example.com or Dhow <dhow@example.com>")
	}
	return *addr, nil
}

func parseOrgCreation(v string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", OrgCreationInstanceOwner:
		return false, nil
	case OrgCreationAnyUser:
		return true, nil
	default:
		return false, errors.New("must be instance-owner or any-user")
	}
}
