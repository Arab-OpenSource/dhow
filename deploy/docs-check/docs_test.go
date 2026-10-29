package docscheck_test

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const root = "../.."

var rootPairs = [][2]string{
	{"README.md", "README.en.md"},
	{"CONTRIBUTING.md", "CONTRIBUTING.en.md"},
	{"SECURITY.md", "SECURITY.en.md"},
}

var (
	scheme       = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*:`)
	englishFirst = regexp.MustCompile(`^\[English\]\(([^)]+)\)$`)
	arabicFirst  = regexp.MustCompile(`^\[العربية\]\(([^)]+)\)$`)
)

type pair struct {
	arabic  string
	english string
}

func markdownUnder(t *testing.T, dir string, skip string) map[string]bool {
	t.Helper()
	found := map[string]bool{}
	base := filepath.Join(root, dir)
	err := filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(base, p)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if skip != "" && rel == skip {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(rel, ".md") {
			found[rel] = true
		}
		return nil
	})
	require.NoError(t, err)
	return found
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func allPairs(t *testing.T) []pair {
	t.Helper()
	var out []pair
	for _, rel := range sortedKeys(markdownUnder(t, "docs", "en")) {
		out = append(out, pair{arabic: "docs/" + rel, english: "docs/en/" + rel})
	}
	for _, p := range rootPairs {
		out = append(out, pair{arabic: p[0], english: p[1]})
	}
	return out
}

func read(t *testing.T, rel string) page {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	require.NoError(t, err, "read %s", rel)
	return parsePage(string(raw))
}

func exists(rel string) bool {
	_, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
	return err == nil
}

func isEnglish(rel string) bool {
	return strings.HasPrefix(rel, "docs/en/") || strings.HasSuffix(rel, ".en.md")
}

func isArabicDoc(rel string) bool {
	if strings.HasPrefix(rel, "docs/") {
		return !isEnglish(rel)
	}
	for _, p := range rootPairs {
		if rel == p[0] {
			return true
		}
	}
	return false
}

func TestBothVersionsHaveTheSameFiles(t *testing.T) {
	arabic := markdownUnder(t, "docs", "en")
	english := markdownUnder(t, "docs/en", "")
	for _, rel := range sortedKeys(arabic) {
		assert.True(t, english[rel], "docs/%s has no English version: add docs/en/%s", rel, rel)
	}
	for _, rel := range sortedKeys(english) {
		assert.True(t, arabic[rel], "docs/en/%s has no Arabic version: add docs/%s", rel, rel)
	}
	for _, p := range rootPairs {
		assert.True(t, exists(p[0]), "%s is missing next to %s", p[0], p[1])
		assert.True(t, exists(p[1]), "%s is missing next to %s", p[1], p[0])
	}
}

func TestBothVersionsHaveTheSameStructure(t *testing.T) {
	for _, p := range allPairs(t) {
		if !exists(p.arabic) || !exists(p.english) {
			continue
		}
		ar, en := read(t, p.arabic), read(t, p.english)
		for level := 1; level <= 6; level++ {
			assert.Equal(t, en.headings[level], ar.headings[level],
				"%s and %s have a different number of level-%d headings", p.arabic, p.english, level)
		}
		assert.Equal(t, len(en.tables), len(ar.tables), "%s and %s have a different number of tables", p.arabic, p.english)
		for i := 0; i < len(en.tables) && i < len(ar.tables); i++ {
			assert.Equal(t, en.tables[i], ar.tables[i], "table %d has a different number of rows in %s and %s", i+1, p.arabic, p.english)
		}
		assert.Equal(t, len(en.blocks), len(ar.blocks), "%s and %s have a different number of code blocks", p.arabic, p.english)
		for i := 0; i < len(en.blocks) && i < len(ar.blocks); i++ {
			assert.Equal(t, en.blocks[i], ar.blocks[i], "code block %d differs between %s and %s: code blocks are byte-identical", i+1, p.arabic, p.english)
		}
	}
}

func TestFirstLineLinksTheOtherVersion(t *testing.T) {
	for _, p := range allPairs(t) {
		for _, side := range []struct {
			file, other string
			want        *regexp.Regexp
		}{{p.arabic, p.english, englishFirst}, {p.english, p.arabic, arabicFirst}} {
			if !exists(side.file) {
				continue
			}
			m := side.want.FindStringSubmatch(read(t, side.file).first)
			if !assert.NotNil(t, m, "%s must start with a link to %s (%s)", side.file, side.other, side.want) {
				continue
			}
			assert.Equal(t, side.other, path.Join(path.Dir(side.file), m[1]), "the first line of %s links to the wrong page", side.file)
		}
	}
}

func checkLink(t *testing.T, file string, l link) {
	t.Helper()
	if scheme.MatchString(l.target) {
		return
	}
	target, anchor, _ := strings.Cut(l.target, "#")
	resolved := file
	if target != "" {
		resolved = path.Join(path.Dir(file), target)
		if !exists(resolved) {
			t.Errorf("%s:%d links to %s, which does not exist", file, l.line, l.target)
			return
		}
	}
	if l.line > 1 && isEnglish(file) && isArabicDoc(resolved) {
		t.Errorf("%s:%d links to the Arabic page %s: link to its English version", file, l.line, resolved)
	}
	if l.line > 1 && !isEnglish(file) && isEnglish(resolved) {
		t.Errorf("%s:%d links to the English page %s: link to its Arabic version", file, l.line, resolved)
	}
	if anchor != "" && strings.HasSuffix(resolved, ".md") && !read(t, resolved).slugs[anchor] {
		t.Errorf("%s:%d links to #%s, which is not a heading in %s", file, l.line, anchor, resolved)
	}
}

func TestEveryLinkResolvesInItsVersion(t *testing.T) {
	for _, p := range allPairs(t) {
		for _, file := range []string{p.arabic, p.english} {
			if !exists(file) {
				continue
			}
			for _, l := range read(t, file).links {
				checkLink(t, file, l)
			}
		}
	}
}

func TestSlugsFollowGitHub(t *testing.T) {
	cases := map[string]string{
		"Quick start":                                      "quick-start",
		"5. Sessions and tokens (ADR-0033)":                "5-sessions-and-tokens-adr-0033",
		"Browsers and http://localhost":                    "browsers-and-httplocalhost",
		"A chain of proxies: Cloudflare in front of nginx": "a-chain-of-proxies-cloudflare-in-front-of-nginx",
		"`DHOW_SECRET_KEY` and `--flag`":                   "dhow_secret_key-and---flag",
		"البدء السريع":                                     "البدء-السريع",
		"ماذا يكتب؟ «التثبيت»، والتحقق":                    "ماذا-يكتب-التثبيت-والتحقق",
		"[الإعدادات](configuration.md) من اللوحة":          "الإعدادات-من-اللوحة",
	}
	for heading, want := range cases {
		assert.Equal(t, want, slugOf(heading), heading)
	}
	p := parsePage("# Setup\n\n## Setup\n\n```sh\n# not a heading\n```\n")
	assert.True(t, p.slugs["setup"])
	assert.True(t, p.slugs["setup-1"])
	assert.Equal(t, 1, p.headings[1])
	assert.Equal(t, 1, p.headings[2])
	assert.Len(t, p.blocks, 1)
}
