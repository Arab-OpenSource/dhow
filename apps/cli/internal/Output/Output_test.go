package output

import (
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Khaymah/dhow/internal/pipelinespec"
)

func pipelineCodes(t *testing.T) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join("..", "..", "..", "..", "internal", "pipelinespec", "codes.go"), nil, 0)
	require.NoError(t, err)
	var codes []string
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, v := range vs.Values {
				lit, ok := v.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				code, err := strconv.Unquote(lit.Value)
				require.NoError(t, err)
				codes = append(codes, code)
			}
		}
	}
	require.Greater(t, len(codes), 50)
	return codes
}

func TestEveryCodeHasAnArabicMessage(t *testing.T) {
	codes := pipelineCodes(t)
	for _, code := range codes {
		msg, ok := arCodes[code]
		if assert.True(t, ok, "%s has no Arabic message in Codes.Ar.go", code) {
			assert.NotEmpty(t, strings.TrimSpace(msg), code)
		}
	}
	assert.ElementsMatch(t, codes, slices.Collect(maps.Keys(arCodes)), "Codes.Ar.go lists a code that pipelinespec does not have")
}

func TestArabicMessagesAreClean(t *testing.T) {
	for code, msg := range arCodes {
		assert.True(t, utf8.ValidString(msg), code)
		for _, r := range msg {
			assert.False(t, r >= 0x200b && r <= 0x200f || r >= 0x202a && r <= 0x202e || r >= 0x2066 && r <= 0x2069,
				"%s holds an invisible direction character", code)
		}
	}
}

func TestBothCatalogsHaveTheSameKeys(t *testing.T) {
	assert.ElementsMatch(t, slices.Collect(maps.Keys(enMessages)), slices.Collect(maps.Keys(arMessages)))
	for k, en := range enMessages {
		assert.Equal(t, strings.Count(en, "%"), strings.Count(arMessages[k], "%"), "%s has different placeholders", k)
	}
}

func TestEnglishUsesTheServerMessage(t *testing.T) {
	_, issues := pipelinespec.Check([]byte("version: 2\non: {push: {}}\njobs: {}\n"))
	require.NotEmpty(t, issues)
	en, ar := NewCatalog(LangEn), NewCatalog(LangAr)
	for _, i := range issues {
		assert.Equal(t, i.Message, en.Issue(i))
		assert.Equal(t, arCodes[i.Code], ar.Issue(i))
	}
	assert.Equal(t, "fallback", ar.Issue(pipelinespec.Issue{Code: "NOT_A_CODE", Message: "fallback"}))
}

func TestChooseLang(t *testing.T) {
	env := func(kv ...string) func(string) string {
		return func(k string) string {
			for i := 0; i+1 < len(kv); i += 2 {
				if kv[i] == k {
					return kv[i+1]
				}
			}
			return ""
		}
	}
	cases := []struct {
		flag   string
		getenv func(string) string
		want   string
	}{
		{"ar", nil, LangAr},
		{"en", env("LANG", "ar_EG.UTF-8"), LangEn},
		{"", env("LANG", "ar_EG.UTF-8"), LangAr},
		{"", env("LANG", "AR_EG.UTF8"), LangAr},
		{"", env("LANG", "ar_EG.ISO-8859-6"), LangEn},
		{"", env("LC_ALL", "C", "LANG", "ar_EG.UTF-8"), LangEn},
		{"", env("LC_MESSAGES", "ar_SA.UTF-8", "LANG", "en_US.UTF-8"), LangAr},
		{"fr", env(), LangEn},
		{"", nil, LangEn},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, ChooseLang(tc.flag, tc.getenv), "%+v", tc.flag)
	}
}

func TestPrintable(t *testing.T) {
	cases := map[string]string{
		"jobs.build.steps[0].run": "jobs.build.steps[0].run",
		"خطوات/.dhow.yml":         "خطوات/.dhow.yml",
		"a b":                     "a b",
		"a\x1b[2Jb":               `a\u001b[2Jb`,
		"a\nb\tc":                 `a\u000ab\u0009c`,
		"x\u202ey":                `x\u202ey`,
		"bad\xffbyte":             `bad\xffbyte`,
		"\U000e0001":              `\U000e0001`,
		`C:\work\.dhow.yml`:       `C:\\work\\.dhow.yml`,
		"a\u202eb\u009b31m":       `a\u202eb\u009b31m`,
		`\u001b`:                  `\\u001b`,
	}
	for in, want := range cases {
		assert.Equal(t, want, Printable(in), strconv.Quote(in))
	}
}
