package alerts

import (
	"net/netip"
	"strings"
	"time"

	eventbus "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/EventBus"
	i18n "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/I18n"
	mailer "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Mail"
)

const (
	ownerKey       = "Owner"
	keySubject     = "mail.alert.subject"
	keyBody        = "mail.alert.body"
	keyUnknown     = "mail.alert.unknown"
	keyChange      = "mail.alert.change."
	mailDateLayout = "2006-01-02 15:04"
	ipv4Prefix     = 24
	ipv6Prefix     = 48
)

func changes() map[string]string {
	return map[string]string{
		eventbus.AuthPasswordChanged:          "password_changed",
		eventbus.AuthTotpEnabled:              "totp_enabled",
		eventbus.AuthTotpDisabled:             "totp_disabled",
		eventbus.AuthPasskeyAdded:             "passkey_added",
		eventbus.AuthPasskeyRemoved:           "passkey_removed",
		eventbus.AuthRecoveryCodesRegenerated: "recovery_codes_regenerated",
		eventbus.AuthIdentityLinked:           "identity_linked",
		eventbus.AuthIdentityUnlinked:         "identity_unlinked",
		eventbus.AuthApiTokenCreated:          "api_token_created",
		eventbus.IdentityMemberMfaReset:       "mfa_reset",
	}
}

func Events() []string {
	return []string{
		eventbus.AuthPasswordChanged,
		eventbus.AuthTotpEnabled,
		eventbus.AuthTotpDisabled,
		eventbus.AuthPasskeyAdded,
		eventbus.AuthPasskeyRemoved,
		eventbus.AuthRecoveryCodesRegenerated,
		eventbus.AuthIdentityLinked,
		eventbus.AuthIdentityUnlinked,
		eventbus.AuthApiTokenCreated,
		eventbus.IdentityMemberMfaReset,
	}
}

func Changes() []string {
	byEvent := changes()
	out := make([]string, 0, len(byEvent))
	for _, name := range Events() {
		out = append(out, byEvent[name])
	}
	return out
}

type Details struct {
	To        string
	Name      string
	Change    string
	At        time.Time
	IP        string
	UserAgent string
	Link      string
}

func Message(locale i18n.Locale, d Details) mailer.Message {
	unknown := i18n.Translate(locale, keyUnknown)
	change := i18n.Translate(locale, keyChange+d.Change)
	r := strings.NewReplacer(
		"{name}", d.Name, "{change}", change, "{time}", d.At.UTC().Format(mailDateLayout),
		"{ip}", orUnknown(ApproximateIP(d.IP), unknown), "{browser}", orUnknown(Browser(d.UserAgent), unknown), "{link}", d.Link,
	)
	subject := strings.NewReplacer("{change}", change).Replace(i18n.Translate(locale, keySubject))
	return mailer.Message{To: d.To, Subject: oneLine(subject), Body: r.Replace(i18n.Translate(locale, keyBody))}
}

func orUnknown(v, unknown string) string {
	if v == "" {
		return unknown
	}
	return v
}

func oneLine(v string) string { return strings.NewReplacer("\r", " ", "\n", " ").Replace(v) }

func ApproximateIP(raw string) string {
	addr, err := netip.ParseAddr(raw)
	if err != nil {
		return ""
	}
	addr = addr.Unmap()
	bits := ipv6Prefix
	if addr.Is4() {
		bits = ipv4Prefix
	}
	prefix, err := addr.Prefix(bits)
	if err != nil {
		return ""
	}
	return prefix.String()
}

type marker struct{ token, label string }

func browsers() []marker {
	return []marker{
		{"Edg/", "Edge"},
		{"OPR/", "Opera"},
		{"Firefox/", "Firefox"},
		{"FxiOS/", "Firefox"},
		{"CriOS/", "Chrome"},
		{"Chrome/", "Chrome"},
		{"Safari/", "Safari"},
		{"curl/", "curl"},
		{"dhow/", "Dhow CLI"},
	}
}

func systems() []marker {
	return []marker{
		{"Windows", "Windows"},
		{"Android", "Android"},
		{"iPhone", "iOS"},
		{"iPad", "iPadOS"},
		{"CrOS", "ChromeOS"},
		{"Mac OS X", "macOS"},
		{"Macintosh", "macOS"},
		{"Linux", "Linux"},
	}
}

func first(ua string, ms []marker) string {
	for _, m := range ms {
		if strings.Contains(ua, m.token) {
			return m.label
		}
	}
	return ""
}

func Browser(ua string) string {
	b, sys := first(ua, browsers()), first(ua, systems())
	switch {
	case b != "" && sys != "":
		return b + " · " + sys
	case b != "":
		return b
	default:
		return sys
	}
}
