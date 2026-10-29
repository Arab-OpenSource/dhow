package settingspec

import (
	"errors"
	"net/mail"
	"strconv"
	"strings"
	"time"
)

const (
	SMTPTLSStartTLS = "starttls"
	SMTPTLSImplicit = "tls"
	SMTPTLSNone     = "none"

	OrgCreationInstanceOwner = "instance-owner"
	OrgCreationAnyUser       = "any-user"

	SMTPDefaultPort        = 587
	SMTPDefaultImplicitTLS = 465
	SMTPHostMaxLength      = 253
	SMTPFieldMaxLength     = 320
	SMTPPasswordMaxLength  = 1024

	RateLimitPublicWindow       = 10 * time.Minute
	RateLimitPublicDefault      = 300
	RateLimitInviteWindow       = time.Hour
	RateLimitInvitesUserDefault = 50
	RateLimitInvitesOrgDefault  = 200
	RateLimitUpperBound         = 100_000

	JobLogMaxBytesDefault = 64 << 20
	JobLogMaxBytesMin     = 1 << 20
	JobLogMaxBytesMax     = 1 << 30
	LogRetentionDefault   = 30
	LogRetentionMax       = 3650
)

const (
	RuleSecretSource = "secretSource"
	RuleBool         = "bool"
	RuleSMTPHost     = "smtpHost"
	RuleSMTPPort     = "smtpPort"
	RuleSMTPTLS      = "smtpTls"
	RuleSMTPFrom     = "smtpFrom"
	RuleSMTPText     = "smtpText"
	RuleSMTPNeedHost = "smtpNeedHost"
	RuleSMTPPair     = "smtpPair"
	RuleSMTPPlain    = "smtpPlain"
	RuleOrgCreation  = "orgCreation"
	RuleRateLimit    = "rateLimit"
	RuleRetention    = "retentionDays"
	RuleLogMaxBytes  = "jobLogMaxBytes"
)

type FieldError struct {
	Env     string
	Rule    string
	Message string
}

func (e FieldError) Error() string { return e.Env + ": " + e.Message }

func Join(errs []FieldError) error {
	out := make([]error, 0, len(errs))
	for _, e := range errs {
		out = append(out, e)
	}
	return errors.Join(out...)
}

type SMTP struct {
	Host     string
	Port     int
	Username string
	Password string
	From     mail.Address
	TLS      string
}

func (s SMTP) Enabled() bool { return s.Host != "" }

type RateLimits struct {
	Public      int
	InvitesUser int
	InvitesOrg  int
}

func (r RateLimits) WithDefaults() RateLimits {
	if r.Public <= 0 {
		r.Public = RateLimitPublicDefault
	}
	if r.InvitesUser <= 0 {
		r.InvitesUser = RateLimitInvitesUserDefault
	}
	if r.InvitesOrg <= 0 {
		r.InvitesOrg = RateLimitInvitesOrgDefault
	}
	return r
}

type Logs struct {
	RetentionDays  int
	JobLogMaxBytes int64
}

type Settings struct {
	Mail                  SMTP
	OrgCreation           string
	BreachedPasswordCheck bool
	RateLimits            RateLimits
	Logs                  Logs
}

func (s Settings) AnyUserCreatesOrganizations() bool { return s.OrgCreation == OrgCreationAnyUser }

func Defaults() Settings {
	s, _ := Parse(Raw{})
	return s
}

func Parse(raw Raw) (Settings, []FieldError) {
	var out Settings
	var errs []FieldError
	add := func(e *FieldError) {
		if e != nil {
			errs = append(errs, *e)
		}
	}
	var mailErrs []FieldError
	out.Mail, mailErrs = parseSMTP(raw)
	errs = append(errs, mailErrs...)
	var e *FieldError
	out.OrgCreation, e = parseOrgCreation(raw.Get(EnvOrgCreation))
	add(e)
	out.BreachedPasswordCheck, e = parseBool(EnvBreachedPasswordCheck, raw.Get(EnvBreachedPasswordCheck), true)
	add(e)
	out.RateLimits.Public, e = boundedInt(EnvRateLimitPublic, RuleRateLimit, raw.Get(EnvRateLimitPublic), RateLimitPublicDefault, 1, RateLimitUpperBound)
	add(e)
	out.RateLimits.InvitesUser, e = boundedInt(EnvRateLimitInvitesUser, RuleRateLimit, raw.Get(EnvRateLimitInvitesUser), RateLimitInvitesUserDefault, 1, RateLimitUpperBound)
	add(e)
	out.RateLimits.InvitesOrg, e = boundedInt(EnvRateLimitInvitesOrg, RuleRateLimit, raw.Get(EnvRateLimitInvitesOrg), RateLimitInvitesOrgDefault, 1, RateLimitUpperBound)
	add(e)
	out.Logs.RetentionDays, e = boundedInt(EnvLogRetentionDays, RuleRetention, raw.Get(EnvLogRetentionDays), LogRetentionDefault, 1, LogRetentionMax)
	add(e)
	var maxBytes int
	maxBytes, e = boundedInt(EnvJobLogMaxBytes, RuleLogMaxBytes, raw.Get(EnvJobLogMaxBytes), JobLogMaxBytesDefault, JobLogMaxBytesMin, JobLogMaxBytesMax)
	add(e)
	out.Logs.JobLogMaxBytes = int64(maxBytes)
	if len(errs) > 0 {
		return Settings{}, errs
	}
	return out, nil
}

func ValidateValue(env, value string) *FieldError {
	switch env {
	case EnvSMTPHost:
		return checkSMTPHost(strings.TrimSpace(value))
	case EnvSMTPPort:
		_, e := parseSMTPPort(value, SMTPTLSStartTLS)
		return e
	case EnvSMTPTLS:
		_, e := parseSMTPTLS(value)
		return e
	case EnvSMTPUsername:
		return checkText(env, value, SMTPFieldMaxLength)
	case EnvSMTPPassword:
		return checkText(env, value, SMTPPasswordMaxLength)
	case EnvSMTPFrom:
		if strings.TrimSpace(value) == "" {
			return nil
		}
		_, e := parseSMTPFrom(value)
		return e
	}
	_, errs := Parse(Raw{env: value})
	for _, e := range errs {
		if e.Env == env {
			return &e
		}
	}
	return nil
}

func parseBool(env, v string, def bool) (bool, *FieldError) {
	v = strings.TrimSpace(v)
	if v == "" {
		return def, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, &FieldError{Env: env, Rule: RuleBool, Message: "must be true or false"}
	}
	return b, nil
}

func boundedInt(env, rule, v string, def, lowest, highest int) (int, *FieldError) {
	v = strings.TrimSpace(v)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < lowest || n > highest {
		return 0, &FieldError{Env: env, Rule: rule, Message: "must be a whole number from " + strconv.Itoa(lowest) + " to " + strconv.Itoa(highest)}
	}
	return n, nil
}

func parseOrgCreation(v string) (string, *FieldError) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", OrgCreationInstanceOwner:
		return OrgCreationInstanceOwner, nil
	case OrgCreationAnyUser:
		return OrgCreationAnyUser, nil
	default:
		return "", &FieldError{Env: EnvOrgCreation, Rule: RuleOrgCreation, Message: "must be instance-owner or any-user"}
	}
}
