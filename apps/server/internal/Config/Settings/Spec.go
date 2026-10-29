package settingspec

import "strings"

const (
	EnvSMTPHost              = "DHOW_SMTP_HOST"
	EnvSMTPPort              = "DHOW_SMTP_PORT"
	EnvSMTPTLS               = "DHOW_SMTP_TLS"
	EnvSMTPUsername          = "DHOW_SMTP_USERNAME"
	EnvSMTPPassword          = "DHOW_SMTP_PASSWORD" //nolint:gosec // The name of an environment variable, not a credential.
	EnvSMTPFrom              = "DHOW_SMTP_FROM"
	EnvOrgCreation           = "DHOW_ORG_CREATION"
	EnvBreachedPasswordCheck = "DHOW_BREACHED_PASSWORD_CHECK" //nolint:gosec // The name of an environment variable, not a credential.
	EnvRateLimitPublic       = "DHOW_RATE_LIMIT_PUBLIC"
	EnvRateLimitInvitesUser  = "DHOW_RATE_LIMIT_INVITES_USER"
	EnvRateLimitInvitesOrg   = "DHOW_RATE_LIMIT_INVITES_ORG"
	EnvLogRetentionDays      = "DHOW_LOG_RETENTION_DAYS"
	EnvJobLogMaxBytes        = "DHOW_JOB_LOG_MAX_BYTES"
)

type Group string

const (
	GroupMail          Group = "mail"
	GroupOrganizations Group = "organizations"
	GroupPasswords     Group = "passwords"
	GroupRateLimits    Group = "rate-limits"
	GroupLogs          Group = "logs"
)

func Groups() []Group {
	return []Group{GroupMail, GroupOrganizations, GroupPasswords, GroupRateLimits, GroupLogs}
}

func ParseGroup(v string) (Group, bool) {
	for _, g := range Groups() {
		if string(g) == v {
			return g, true
		}
	}
	return "", false
}

type Kind uint8

const (
	KindString Kind = iota + 1
	KindInt
	KindBool
)

type Key struct {
	Group   Group
	Field   string
	Env     string
	Kind    Kind
	Secret  bool
	Default string
}

var keys = []Key{
	{GroupMail, "Host", EnvSMTPHost, KindString, false, ""},
	{GroupMail, "Port", EnvSMTPPort, KindInt, false, ""},
	{GroupMail, "Tls", EnvSMTPTLS, KindString, false, SMTPTLSStartTLS},
	{GroupMail, "Username", EnvSMTPUsername, KindString, false, ""},
	{GroupMail, "Password", EnvSMTPPassword, KindString, true, ""},
	{GroupMail, "From", EnvSMTPFrom, KindString, false, ""},
	{GroupOrganizations, "Creation", EnvOrgCreation, KindString, false, OrgCreationInstanceOwner},
	{GroupPasswords, "BreachedCheck", EnvBreachedPasswordCheck, KindBool, false, "true"},
	{GroupRateLimits, "Public", EnvRateLimitPublic, KindInt, false, "300"},
	{GroupRateLimits, "InvitesPerUser", EnvRateLimitInvitesUser, KindInt, false, "50"},
	{GroupRateLimits, "InvitesPerOrganization", EnvRateLimitInvitesOrg, KindInt, false, "200"},
	{GroupLogs, "RetentionDays", EnvLogRetentionDays, KindInt, false, "30"},
	{GroupLogs, "JobLogMaxBytes", EnvJobLogMaxBytes, KindInt, false, "67108864"},
}

func Keys() []Key { return append([]Key(nil), keys...) }

func KeysOf(g Group) []Key {
	var out []Key
	for _, k := range keys {
		if k.Group == g {
			out = append(out, k)
		}
	}
	return out
}

func KeyByEnv(env string) (Key, bool) {
	for _, k := range keys {
		if k.Env == env {
			return k, true
		}
	}
	return Key{}, false
}

func KeyByField(g Group, field string) (Key, bool) {
	for _, k := range keys {
		if k.Group == g && k.Field == field {
			return k, true
		}
	}
	return Key{}, false
}

type Raw map[string]string

func (r Raw) Get(env string) string { return r[env] }

func (r Raw) Has(env string) bool {
	_, ok := r[env]
	return ok
}

func (r Raw) Clone() Raw {
	out := make(Raw, len(r))
	for k, v := range r {
		out[k] = v
	}
	return out
}

func FromEnv(getenv func(string) string, secret func(Key) (string, error)) (Raw, []FieldError) {
	out := Raw{}
	var errs []FieldError
	for _, k := range keys {
		v := getenv(k.Env)
		if k.Secret && secret != nil {
			s, err := secret(k)
			if err != nil {
				errs = append(errs, FieldError{Env: k.Env, Rule: RuleSecretSource, Message: err.Error()})
				continue
			}
			v = s
		}
		if strings.TrimSpace(v) == "" {
			continue
		}
		out[k.Env] = v
	}
	return out, errs
}
