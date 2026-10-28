package oidcauth

import (
	"slices"
	"strings"
)

const maxEmailLength = 254

func domainOf(address string) string {
	at := strings.LastIndexByte(address, '@')
	if at <= 0 || at == len(address)-1 {
		return ""
	}
	return address[at+1:]
}

func normalized(e Email) (Email, bool) {
	e.Address = strings.ToLower(strings.TrimSpace(e.Address))
	if e.Address == "" || len(e.Address) > maxEmailLength || strings.Count(e.Address, "@") != 1 || domainOf(e.Address) == "" {
		return Email{}, false
	}
	return e, true
}

func ordered(emails []Email) []Email {
	out := make([]Email, 0, len(emails))
	for _, e := range emails {
		if n, ok := normalized(e); ok {
			out = append(out, n)
		}
	}
	slices.SortStableFunc(out, func(a, b Email) int {
		return rank(b) - rank(a)
	})
	return out
}

func rank(e Email) int {
	score := 0
	if e.Verified {
		score += 2
	}
	if e.Primary {
		score++
	}
	return score
}

func AcceptedEmail(id Identity, allowed []string) (string, bool) {
	emails := ordered(id.Emails)
	if len(allowed) == 0 {
		for _, e := range emails {
			if e.Verified {
				return e.Address, true
			}
		}
		return "", true
	}
	for _, e := range emails {
		if e.Verified && slices.Contains(allowed, domainOf(e.Address)) {
			return e.Address, true
		}
	}
	return "", false
}
