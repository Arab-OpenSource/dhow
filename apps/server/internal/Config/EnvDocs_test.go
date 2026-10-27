package config_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const configurationDoc = "../../../../docs/install/configuration.md"

var (
	envName       = regexp.MustCompile(`^DHOW_[A-Z0-9_]+$`)
	documentedRow = regexp.MustCompile("(?m)^\\| `(DHOW_[A-Z0-9_]+)` \\|")
)

var documentedSections = []string{"## Server variables", "## Development and tests"}

func declaredEnvVars(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(".")
	require.NoError(t, err)
	fset := token.NewFileSet()
	seen := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, e.Name(), nil, 0)
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
	return sortedKeys(seen)
}

func documentedEnvVars(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile(configurationDoc)
	require.NoError(t, err)
	doc := string(raw)
	seen := map[string]bool{}
	for _, heading := range documentedSections {
		start := strings.Index(doc, "\n"+heading+"\n")
		require.GreaterOrEqual(t, start, 0, "%s has no %q section", configurationDoc, heading)
		section := doc[start+len(heading)+2:]
		if end := strings.Index(section, "\n## "); end >= 0 {
			section = section[:end]
		}
		for _, m := range documentedRow.FindAllStringSubmatch(section, -1) {
			assert.False(t, seen[m[1]], "%s is listed twice in %s", m[1], configurationDoc)
			seen[m[1]] = true
		}
	}
	return sortedKeys(seen)
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
	documented := documentedEnvVars(t)
	for _, name := range declared {
		assert.Contains(t, documented, name, "%s is read by the config package but missing from %s", name, configurationDoc)
	}
	for _, name := range documented {
		assert.Contains(t, declared, name, "%s is documented in %s but the config package no longer declares it", name, configurationDoc)
	}
}
