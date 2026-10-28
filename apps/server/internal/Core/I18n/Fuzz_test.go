package i18n_test

import (
	"strings"
	"testing"

	i18n "github.com/Khaymah/dhow/apps/server/internal/Core/I18n"
)

func FuzzParseAcceptLanguage(f *testing.F) {
	for _, seed := range []string{"", "en", "ar-EG,ar;q=0.9,en;q=0.8", "en-US;q=0.5, ar;q=0.6", "fr, de", "en;q=abc", "en;q=-1", "*", ";;;,,,", strings.Repeat("en,", 2000)} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, header string) {
		got := i18n.Parse(header)
		if got != i18n.Ar && got != i18n.En {
			t.Fatalf("Parse(%q) = %q, which the server cannot answer in", header, got)
		}
		if got == i18n.En && !strings.Contains(strings.ToLower(header), "en") {
			t.Fatalf("Parse(%q) chose English without an English tag", header)
		}
		if i18n.Translate(got, "errors.rateLimited") == "errors.rateLimited" {
			t.Fatalf("the chosen locale %q has no catalog", got)
		}
	})
}
