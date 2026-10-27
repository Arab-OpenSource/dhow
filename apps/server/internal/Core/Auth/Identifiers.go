package auth

import (
	"net/mail"
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxEmailLength = 254

func NormalizeEmail(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}

func ValidEmail(normalized string) bool {
	if normalized == "" || len(normalized) > maxEmailLength || strings.ContainsAny(normalized, " \t\r\n<>,;\"") {
		return false
	}
	addr, err := mail.ParseAddress(normalized)
	return err == nil && addr.Address == normalized && strings.Count(normalized, "@") == 1
}

func ValidDisplayName(v string, maxRunes int) bool {
	n := utf8.RuneCountInString(v)
	if n < 1 || n > maxRunes || strings.TrimSpace(v) != v {
		return false
	}
	for _, r := range v {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
