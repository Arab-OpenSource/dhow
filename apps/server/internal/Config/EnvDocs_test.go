package config_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	settingspec "github.com/Khaymah/dhow/apps/server/internal/Config/Settings"
)

type configurationDoc struct {
	path      string
	server    string
	dashboard string
	dev       string
}

var configurationDocs = []configurationDoc{
	{"../../../../docs/install/configuration.md", "## متغيرات السيرفر", "## الإعدادات من الواجهة", "## التطوير والاختبارات"},
	{"../../../../docs/en/install/configuration.md", "## Server variables", "## Dashboard settings", "## Development and tests"},
}

var (
	envName       = regexp.MustCompile(`^DHOW_[A-Z0-9_]+$`)
	documentedRow = regexp.MustCompile("(?m)^\\| `(DHOW_[A-Z0-9_]+)` \\|")
)

func declaredEnvVars(t *testing.T) []string {
	t.Helper()
	seen := map[string]bool{}
	for _, dir := range []string{".", "Settings"} {
		declaredIn(t, dir, seen)
	}
	return sortedKeys(seen)
}

func declaredIn(t *testing.T, dir string, seen map[string]bool) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	fset := token.NewFileSet()
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, 0)
		require.NoError(t, err)
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			if v, err := strconv.Unquote(lit.Value); err == nil && envName.MatchString(v) {
				seen[v] = true
			}
			return true
		})
	}
}

func sectionOf(t *testing.T, doc, path, heading string) string {
	t.Helper()
	start := strings.Index(doc, "\n"+heading+"\n")
	require.GreaterOrEqual(t, start, 0, "%s has no %q section", path, heading)
	section := doc[start+len(heading)+2:]
	if end := strings.Index(section, "\n## "); end >= 0 {
		section = section[:end]
	}
	return section
}

func readDoc(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(raw)
}

func documentedEnvVars(t *testing.T, cd configurationDoc) []string {
	t.Helper()
	doc := readDoc(t, cd.path)
	seen := map[string]bool{}
	for _, heading := range []string{cd.server, cd.dashboard, cd.dev} {
		for _, m := range documentedRow.FindAllStringSubmatch(sectionOf(t, doc, cd.path, heading), -1) {
			assert.False(t, seen[m[1]], "%s is listed twice in %s", m[1], cd.path)
			seen[m[1]] = true
		}
	}
	return sortedKeys(seen)
}

func TestDashboardSettingsAreDocumentedApartFromTheServerVariables(t *testing.T) {
	var want []string
	for _, k := range settingspec.Keys() {
		want = append(want, k.Env)
	}
	sort.Strings(want)
	for _, cd := range configurationDocs {
		doc := readDoc(t, cd.path)
		var got []string
		for _, m := range documentedRow.FindAllStringSubmatch(sectionOf(t, doc, cd.path, cd.dashboard), -1) {
			got = append(got, m[1])
		}
		sort.Strings(got)
		assert.Equal(t, want, got, "the %q table of %s lists exactly the settings the dashboard can change", cd.dashboard, cd.path)
		server := sectionOf(t, doc, cd.path, cd.server)
		for _, name := range want {
			assert.NotContains(t, server, "| `"+name+"` |", "%s is a dashboard setting, not a server variable, in %s", name, cd.path)
		}
	}
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func TestConfigurationDocListsEveryEnvVar(t *testing.T) {
	declared := declaredEnvVars(t)
	require.NotEmpty(t, declared)
	for _, cd := range configurationDocs {
		documented := documentedEnvVars(t, cd)
		for _, name := range declared {
			assert.Contains(t, documented, name, "%s is read by the config package but missing from %s", name, cd.path)
		}
		for _, name := range documented {
			assert.Contains(t, declared, name, "%s is documented in %s but the config package no longer declares it", name, cd.path)
		}
	}
}
