package unit_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	eventbus "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/EventBus"
	i18n "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/I18n"
	alerts "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Alerts"
)

func TestApproximateIP(t *testing.T) {
	tests := []struct{ in, want string }{
		{"203.0.113.7", "203.0.113.0/24"},
		{"::ffff:203.0.113.7", "203.0.113.0/24"},
		{"2001:db8:1234:5678::1", "2001:db8:1234::/48"},
		{"", ""},
		{"not an address", ""},
		{"203.0.113.7:4444", ""},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, alerts.ApproximateIP(tt.in), tt.in)
	}
}

func TestBrowser(t *testing.T) {
	tests := []struct{ ua, want string }{
		{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0 Safari/537.36 Edg/140.0", "Edge · Windows"},
		{"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_5) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Safari/605.1.15", "Safari · macOS"},
		{"Mozilla/5.0 (Linux; Android 15) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0 Mobile Safari/537.36", "Chrome · Android"},
		{"Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/140.0 Mobile/15E148 Safari/604.1", "Chrome · iOS"},
		{"Mozilla/5.0 (X11; Linux x86_64; rv:140.0) Gecko/20100101 Firefox/140.0", "Firefox · Linux"},
		{"curl/8.9.1", "curl"},
		{"dhow-test-agent", ""},
		{"", ""},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, alerts.Browser(tt.ua), tt.ua)
	}
}

func TestEveryAlertedEventIsAuditedAndTranslated(t *testing.T) {
	audited := eventbus.AuditedEvents()
	for _, name := range alerts.Events() {
		assert.True(t, slices.Contains(audited, name), "%s is audited as well as alerted", name)
	}
	for _, c := range alerts.Changes() {
		key := "mail.alert.change." + c
		for _, l := range []i18n.Locale{i18n.Ar, i18n.En} {
			assert.NotEqual(t, key, i18n.Translate(l, key), "%s has a %s text", key, l)
		}
	}
	assert.Len(t, alerts.Changes(), len(alerts.Events()))
}

func TestEveryMessageFillsItsPlaceholdersAndKeepsTheSubjectOnOneLine(t *testing.T) {
	at := time.Date(2026, 10, 28, 9, 5, 0, 0, time.FixedZone("Cairo", 3*3600))
	for _, l := range []i18n.Locale{i18n.Ar, i18n.En} {
		for _, c := range alerts.Changes() {
			m := alerts.Message(l, alerts.Details{To: "a@example.com", Name: "Mona", Change: c, At: at, Link: "https://dhow.example/?stack=account"})
			assert.NotContains(t, m.Subject+m.Body, "{", "%s %s", l, c)
			assert.NotContains(t, m.Subject, "\n")
			assert.Equal(t, "a@example.com", m.To)
			assert.Contains(t, m.Body, "2026-10-28 06:05")
			assert.Contains(t, m.Body, "https://dhow.example/?stack=account")
			if c == "mfa_reset" {
				assert.NotContains(t, m.Body, i18n.Translate(l, "mail.alert.unknown"), "the admin's network and browser are not shown")
				continue
			}
			assert.Contains(t, m.Body, i18n.Translate(l, "mail.alert.unknown"), "an unknown network and browser are said plainly")
		}
	}
	m := alerts.Message(i18n.En, alerts.Details{Name: "Mona\r\nBcc: x@example.com", Change: "password_changed"})
	assert.False(t, strings.Contains(m.Subject, "\r") || strings.Contains(m.Subject, "\n"), "the name never reaches a header")
}

func TestTheMfaResetAlertLeavesOutTheAdminsNetworkAndBrowser(t *testing.T) {
	m := alerts.Message(i18n.En, alerts.Details{
		Name: "Mona", Change: "mfa_reset", IP: "198.51.100.23", UserAgent: "Mozilla/5.0 (X11; Linux x86_64) Firefox/140.0",
	})
	assert.NotContains(t, m.Body, "198.51.100")
	assert.NotContains(t, m.Body, "Firefox")
	assert.NotContains(t, m.Body, "change your password")
}
