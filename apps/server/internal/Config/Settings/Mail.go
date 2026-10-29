package settingspec

import (
	"net/mail"
	"strconv"
	"strings"
)

func parseSMTP(raw Raw) (SMTP, []FieldError) {
	host := strings.TrimSpace(raw.Get(EnvSMTPHost))
	password := raw.Get(EnvSMTPPassword)
	username := strings.TrimSpace(raw.Get(EnvSMTPUsername))
	from := strings.TrimSpace(raw.Get(EnvSMTPFrom))
	var errs []FieldError
	add := func(e *FieldError) {
		if e != nil {
			errs = append(errs, *e)
		}
	}
	cfg := SMTP{Host: host, Username: username, Password: password}
	var e *FieldError
	cfg.TLS, e = parseSMTPTLS(raw.Get(EnvSMTPTLS))
	add(e)
	cfg.Port, e = parseSMTPPort(raw.Get(EnvSMTPPort), cfg.TLS)
	add(e)
	if host == "" {
		if password != "" || username != "" || from != "" {
			add(&FieldError{Env: EnvSMTPHost, Rule: RuleSMTPNeedHost, Message: "is required when other DHOW_SMTP_* variables are set"})
		}
		if len(errs) > 0 {
			return SMTP{}, errs
		}
		return cfg, nil
	}
	add(checkSMTPHost(host))
	add(checkText(EnvSMTPUsername, username, SMTPFieldMaxLength))
	add(checkText(EnvSMTPPassword, password, SMTPPasswordMaxLength))
	if from == "" {
		add(&FieldError{Env: EnvSMTPFrom, Rule: RuleSMTPFrom, Message: "is required when DHOW_SMTP_HOST is set, for example Dhow <dhow@example.com>"})
	} else {
		cfg.From, e = parseSMTPFrom(from)
		add(e)
	}
	if (cfg.Username == "") != (cfg.Password == "") {
		add(&FieldError{Env: EnvSMTPUsername, Rule: RuleSMTPPair, Message: "set both DHOW_SMTP_USERNAME and DHOW_SMTP_PASSWORD, or neither"})
	}
	if cfg.TLS == SMTPTLSNone && cfg.Password != "" {
		add(&FieldError{Env: EnvSMTPTLS, Rule: RuleSMTPPlain, Message: "none never sends a password; use starttls or tls with credentials"})
	}
	if len(errs) > 0 {
		return SMTP{}, errs
	}
	return cfg, nil
}

func checkSMTPHost(host string) *FieldError {
	if host == "" {
		return nil
	}
	if len(host) > SMTPHostMaxLength || strings.ContainsAny(host, " \t\r\n/:@") {
		return &FieldError{Env: EnvSMTPHost, Rule: RuleSMTPHost, Message: "must be a host name or an IP address without a port"}
	}
	if BlockedHost(host) {
		return &FieldError{Env: EnvSMTPHost, Rule: RuleSMTPHostBlocked, Message: "must not be a link-local or cloud metadata address"}
	}
	return nil
}

func checkText(env, v string, maxLength int) *FieldError {
	if len(v) > maxLength || strings.ContainsAny(v, "\r\n\x00") {
		return &FieldError{Env: env, Rule: RuleSMTPText, Message: "must be one line of at most " + strconv.Itoa(maxLength) + " bytes"}
	}
	return nil
}

func parseSMTPTLS(v string) (string, *FieldError) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", SMTPTLSStartTLS:
		return SMTPTLSStartTLS, nil
	case SMTPTLSImplicit:
		return SMTPTLSImplicit, nil
	case SMTPTLSNone:
		return SMTPTLSNone, nil
	default:
		return "", &FieldError{Env: EnvSMTPTLS, Rule: RuleSMTPTLS, Message: "must be starttls, tls or none"}
	}
}

func parseSMTPPort(v, tls string) (int, *FieldError) {
	v = strings.TrimSpace(v)
	if v == "" {
		if tls == SMTPTLSImplicit {
			return SMTPDefaultImplicitTLS, nil
		}
		return SMTPDefaultPort, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 || n > 65535 {
		return 0, &FieldError{Env: EnvSMTPPort, Rule: RuleSMTPPort, Message: "must be a port between 1 and 65535"}
	}
	return n, nil
}

func parseSMTPFrom(v string) (mail.Address, *FieldError) {
	v = strings.TrimSpace(v)
	if strings.ContainsAny(v, "\r\n") {
		return mail.Address{}, &FieldError{Env: EnvSMTPFrom, Rule: RuleSMTPFrom, Message: "must be one address"}
	}
	addr, err := mail.ParseAddress(v)
	if err != nil {
		return mail.Address{}, &FieldError{Env: EnvSMTPFrom, Rule: RuleSMTPFrom, Message: "must be an address such as dhow@example.com or Dhow <dhow@example.com>"}
	}
	return *addr, nil
}
