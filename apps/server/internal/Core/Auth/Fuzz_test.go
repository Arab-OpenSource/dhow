package auth_test

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/go-webauthn/webauthn/protocol"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	auth "github.com/Khaymah/dhow/apps/server/internal/Core/Auth"
)

const fuzzTokenBytes = 32

var fuzzSecret = bytes.Repeat([]byte{0x5a, 0x17, 0x93, 0xc4}, 8)

func FuzzHashSessionToken(f *testing.F) {
	good, _, err := auth.NewSessionToken(fuzzTokenBytes)
	if err != nil {
		f.Fatal(err)
	}
	for _, seed := range []string{good, "", "=", good + "=", good[:42], strings.ToUpper(good), "AAAA", good + "A", strings.Repeat("A", 4096)} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, token string) {
		hash, ok := auth.HashSessionToken(token, fuzzTokenBytes)
		raw, err := base64.RawURLEncoding.Strict().DecodeString(token)
		want := err == nil && len(raw) == fuzzTokenBytes && base64.RawURLEncoding.EncodeToString(raw) == token
		if ok != want {
			t.Fatalf("HashSessionToken(%q) ok=%t, want %t", token, ok, want)
		}
		if !ok {
			if hash != nil {
				t.Fatalf("a refused token still has a hash")
			}
			return
		}
		if len(hash) != 32 || !bytes.Equal(hash, auth.HashBytes(raw)) {
			t.Fatalf("the hash of %q is not SHA-256 of its bytes", token)
		}
		if base64.RawURLEncoding.EncodeToString(raw) != token {
			t.Fatalf("%q is accepted but is not the one canonical spelling of its bytes", token)
		}
	})
}

func FuzzAPITokenHash(f *testing.F) {
	tokens, err := auth.NewAPITokens(fuzzSecret)
	if err != nil {
		f.Fatal(err)
	}
	good, _, err := tokens.New()
	if err != nil {
		f.Fatal(err)
	}
	for _, seed := range []string{good, config.APITokenPrefix, "dhow_rat_" + good[len(config.APITokenPrefix):], strings.ToUpper(good), good + "=", " " + good, ""} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, token string) {
		hash, ok := tokens.Hash(token)
		if !ok {
			return
		}
		rest, prefixed := strings.CutPrefix(token, config.APITokenPrefix)
		raw, err := base64.RawURLEncoding.Strict().DecodeString(rest)
		if !prefixed || err != nil || len(raw) != config.APITokenBytes || len(hash) != 32 || base64.RawURLEncoding.EncodeToString(raw) != rest {
			t.Fatalf("Hash accepted %q, which is not dhow_pat_ and 32 bytes of base64url", token)
		}
		again, _ := tokens.Hash(token)
		if !bytes.Equal(hash, again) {
			t.Fatalf("the hash of %q is not stable", token)
		}
	})
}

func FuzzAuthorizationHeader(f *testing.F) {
	f.Add("Bearer dhow_pat_x", "")
	f.Add("bearer abc", "Basic dXNlcjpwYXNz")
	f.Add("Bearer a", "Bearer b")
	f.Add("Bearer", "")
	f.Add("BEARER    spaced   ", "")
	f.Add("Basic abc", "")
	f.Fuzz(func(t *testing.T, first, second string) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		values := []string{}
		for _, v := range []string{first, second} {
			if v != "" {
				r.Header.Add("Authorization", v)
				values = append(values, v)
			}
		}
		token, present := auth.Authorization(r)
		bearers := 0
		for _, v := range values {
			if len(v) >= len("Bearer ") && strings.EqualFold(v[:len("Bearer ")], "Bearer ") {
				bearers++
			}
		}
		if present != (bearers > 0) {
			t.Fatalf("present=%t with %d bearer values in %q", present, bearers, values)
		}
		if bearers > 1 && token != "" {
			t.Fatalf("two bearer values must never pick one: got %q", token)
		}
		if token != strings.TrimSpace(token) {
			t.Fatalf("the token %q keeps surrounding spaces", token)
		}
	})
}

func FuzzTotp(f *testing.F) {
	secret := bytes.Repeat([]byte{7}, 20)
	at := time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC)
	f.Add(auth.TotpCodeAt(secret, auth.TotpStep(at)), at.Unix(), int64(0))
	f.Add(" 123 456 ", at.Unix(), auth.TotpStep(at))
	f.Add("12345a", int64(0), int64(-1))
	f.Add("١٢٣٤٥٦", at.Unix(), int64(0))
	f.Add("1234567", at.Unix(), int64(0))
	f.Fuzz(func(t *testing.T, code string, unix, lastStep int64) {
		normalized, ok := auth.NormalizeTotpCode(code)
		if ok {
			if len(normalized) != auth.TotpDigits {
				t.Fatalf("normalized %q to %q", code, normalized)
			}
			for _, r := range normalized {
				if r < '0' || r > '9' {
					t.Fatalf("normalized %q keeps %q", code, r)
				}
			}
		}
		now := time.Unix(unix%(1<<40), 0)
		step, matched := auth.MatchTotp(secret, code, now, lastStep)
		if !matched {
			return
		}
		current := auth.TotpStep(now)
		if !ok || step <= lastStep || step < current-auth.TotpSkewSteps || step > current+auth.TotpSkewSteps {
			t.Fatalf("matched step %d for %q at %d after %d", step, code, current, lastStep)
		}
		if auth.TotpCodeAt(secret, step) != normalized {
			t.Fatalf("matched %q to a step whose code differs", code)
		}
	})
}

func FuzzRecoveryCode(f *testing.F) {
	codes, err := auth.NewRecoveryCodes(2)
	if err != nil {
		f.Fatal(err)
	}
	for _, seed := range append(codes, strings.ToLower(codes[0]), "oooo-iiii-llll-0000", "UUUU-UUUU-UUUU-UUUU", "", strings.Repeat("-", 70)) {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, code string) {
		normalized, ok := auth.NormalizeRecoveryCode(code)
		if !ok {
			return
		}
		if len(normalized) != 16 || strings.ContainsAny(normalized, "-ILOU ") || normalized != strings.ToUpper(normalized) {
			t.Fatalf("normalized %q to %q", code, normalized)
		}
		again, ok := auth.NormalizeRecoveryCode(normalized)
		if !ok || again != normalized {
			t.Fatalf("normalizing is not idempotent for %q", code)
		}
	})
}

func FuzzEmailAndName(f *testing.F) {
	for _, seed := range []string{"Owner@Example.com", " a@b.co ", "a@b@c", "\"a b\"@example.com", "a@[127.0.0.1]", "<a@b.co>", "a@b.co, c@d.co", "سارة@example.com", "a@b.co\n", ""} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, in string) {
		normalized := auth.NormalizeEmail(in)
		if auth.NormalizeEmail(normalized) != normalized {
			t.Fatalf("NormalizeEmail is not idempotent for %q", in)
		}
		if auth.ValidEmail(normalized) {
			if len(normalized) > 254 || strings.Count(normalized, "@") != 1 || strings.ContainsAny(normalized, " \t\r\n<>,;\"") ||
				strings.ToLower(normalized) != normalized {
				t.Fatalf("ValidEmail accepted %q", normalized)
			}
		}
		if auth.ValidDisplayName(in, 100) {
			n := utf8.RuneCountInString(in)
			if n < 1 || n > 100 || strings.TrimSpace(in) != in {
				t.Fatalf("ValidDisplayName accepted %q", in)
			}
			for _, r := range in {
				if unicode.IsControl(r) {
					t.Fatalf("ValidDisplayName accepted a control character in %q", in)
				}
			}
		}
	})
}

func FuzzPasswordLength(f *testing.F) {
	policy := auth.NewPasswordPolicy(auth.NoBreachCheck{}, nil, config.PasswordMinLength, config.PasswordMaxLength)
	for _, seed := range []string{"", "short", "correct horse battery staple", strings.Repeat("ك", 128), strings.Repeat("a", 129), "\xff\xfe\xfd\xfc\xfb\xfa\xf9\xf8\xf7\xf6\xf5\xf4"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, password string) {
		n := utf8.RuneCountInString(password)
		ae := policy.ValidateLength("Password", password)
		if (ae == nil) != (n >= config.PasswordMinLength && n <= config.PasswordMaxLength) {
			t.Fatalf("length %d judged %v", n, ae)
		}
		if ae != nil {
			for _, d := range ae.Details {
				if d.Field != "Password" || d.Message != "errors.passwordLength" {
					t.Fatalf("the error detail %+v is not the fixed length message", d)
				}
			}
		}
	})
}

func FuzzWebAuthnResponses(f *testing.F) {
	for _, seed := range []string{
		`{"id":"AAAA","rawId":"AAAA","type":"public-key","response":{"clientDataJSON":"e30","authenticatorData":"AAAA","signature":"AAAA"}}`,
		`{"id":"AAAA","type":"public-key"}`,
		`{"id":"AAAA","rawId":"AAAA","type":"public-key","response":{"clientDataJSON":"e30","attestationObject":"oA"}}`,
		`null`, `[]`, `{"response":{"clientDataJSON":"eyJ0eXBlIjoid2ViYXV0aG4uZ2V0In0"}}`, ``,
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, body []byte) {
		if parsed, err := protocol.ParseCredentialRequestResponseBytes(body); err == nil && parsed == nil {
			t.Fatal("an assertion parsed without an error but is nil")
		}
		if parsed, err := protocol.ParseCredentialCreationResponseBytes(body); err == nil && parsed == nil {
			t.Fatal("an attestation parsed without an error but is nil")
		}
	})
}
