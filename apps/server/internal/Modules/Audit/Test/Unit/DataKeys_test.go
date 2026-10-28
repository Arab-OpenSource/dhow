package unit_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	eventbus "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/EventBus"
	audit "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Audit"
)

const modulesDir = "../../.."

var selectorKeys = map[string]string{"eventbus.SeverityKey": eventbus.SeverityKey}

func stringMap(t ast.Expr) bool {
	m, ok := t.(*ast.MapType)
	if !ok {
		return false
	}
	k, kok := m.Key.(*ast.Ident)
	v, vok := m.Value.(*ast.Ident)
	return kok && vok && k.Name == "string" && v.Name == "string"
}

func keyOf(t *testing.T, e ast.Expr, where string) (string, bool) {
	switch k := e.(type) {
	case *ast.BasicLit:
		if k.Kind != token.STRING {
			return "", false
		}
		v, err := strconv.Unquote(k.Value)
		require.NoError(t, err)
		return v, true
	case *ast.SelectorExpr:
		pkg, ok := k.X.(*ast.Ident)
		if !ok || pkg.Name != "eventbus" {
			return "", false
		}
		name := pkg.Name + "." + k.Sel.Name
		v, ok := selectorKeys[name]
		require.True(t, ok, "%s: the audit data key %s is a constant this test does not know", where, name)
		return v, true
	}
	return "", false
}

func usedDataKeys(t *testing.T) map[string][]string {
	t.Helper()
	used := map[string][]string{}
	err := filepath.WalkDir(modulesDir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == "Test" || d.Name() == "Gen") {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(token.NewFileSet(), p, nil, 0)
		require.NoError(t, err)
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.CompositeLit:
				if !stringMap(x.Type) {
					return true
				}
				for _, e := range x.Elts {
					if k, ok := keyOf(t, e.(*ast.KeyValueExpr).Key, p); ok {
						used[k] = append(used[k], p)
					}
				}
			case *ast.AssignStmt:
				for _, l := range x.Lhs {
					if ie, ok := l.(*ast.IndexExpr); ok {
						if k, ok := keyOf(t, ie.Index, p); ok {
							used[k] = append(used[k], p)
						}
					}
				}
			}
			return true
		})
		return nil
	})
	require.NoError(t, err)
	return used
}

func TestEveryAuditFieldTheModulesWriteIsAllowed(t *testing.T) {
	used := usedDataKeys(t)
	require.Contains(t, used, "RevokedSessions", "the scan found the module sources")
	keys := make([]string, 0, len(used))
	for k := range used {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		assert.True(t, audit.AllowedDataKey(k), "%s is written by %v but would be dropped from the audit log", k, used[k])
	}
}

func TestEveryAllowedFieldIsStillWritten(t *testing.T) {
	used := usedDataKeys(t)
	for _, k := range audit.AllowedDataKeys() {
		assert.Contains(t, used, k, "%s is allowed in the audit log but no module writes it any more", k)
	}
}

func TestNoAllowedFieldNamesASecret(t *testing.T) {
	for _, k := range audit.AllowedDataKeys() {
		lower := strings.ToLower(k)
		for _, bad := range []string{"token", "secret", "password", "passwd", "cookie", "credential", "authorization", "privatekey", "apikey"} {
			assert.NotContains(t, lower, bad, "%s", k)
		}
		assert.NotEqual(t, "code", lower)
		assert.NotEqual(t, "key", lower)
		assert.NotEqual(t, "session", lower)
	}
}
