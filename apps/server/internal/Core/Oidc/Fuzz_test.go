package oidcauth_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	oidcauth "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Oidc"
)

func FuzzEmailVerifiedClaim(f *testing.F) {
	for _, seed := range []string{`true`, `false`, `"true"`, `"True"`, `1`, `"1"`, `null`, ``, `{"a":1}`, `[true]`} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		got := oidcauth.Truthy(json.RawMessage(raw))
		var b bool
		var s string
		want := (json.Unmarshal(raw, &b) == nil && b) || (json.Unmarshal(raw, &s) == nil && s == "true")
		if got != want {
			t.Fatalf("email_verified %q read as %t", raw, got)
		}
	})
}

func FuzzAcceptedEmail(f *testing.F) {
	f.Add("sara@example.com", true, "OTHER@Example.com", false, "example.com")
	f.Add("sara@sub.example.com", true, "", false, "example.com")
	f.Add("sara@EXAMPLE.com", true, "x@evil.com", true, "example.com")
	f.Add("a@b@example.com", true, "", false, "example.com")
	f.Add(" sara@example.com ", false, "s@example.com", true, "")
	f.Fuzz(func(t *testing.T, first string, firstVerified bool, second string, secondVerified bool, domain string) {
		id := oidcauth.Identity{Issuer: "https://idp.example", Subject: "1", Emails: []oidcauth.Email{
			{Address: first, Verified: firstVerified, Primary: true}, {Address: second, Verified: secondVerified},
		}}
		var allowed []string
		if domain != "" {
			allowed = []string{strings.ToLower(domain)}
		}
		email, ok := oidcauth.AcceptedEmail(id, allowed)
		if !ok {
			if len(allowed) == 0 {
				t.Fatal("without a domain list every identity is accepted")
			}
			return
		}
		if email == "" {
			if len(allowed) != 0 {
				t.Fatal("a domain list accepted an identity without an email")
			}
			return
		}
		if email != strings.ToLower(strings.TrimSpace(email)) || strings.Count(email, "@") != 1 {
			t.Fatalf("accepted %q, which is not one normalized address", email)
		}
		verified := false
		for _, e := range id.Emails {
			if e.Verified && strings.ToLower(strings.TrimSpace(e.Address)) == email {
				verified = true
			}
		}
		if !verified {
			t.Fatalf("accepted %q, which is not a verified email of the identity", email)
		}
		if len(allowed) > 0 && !slices.Contains(allowed, email[strings.LastIndexByte(email, '@')+1:]) {
			t.Fatalf("accepted %q outside the allowed domain %q", email, domain)
		}
	})
}
