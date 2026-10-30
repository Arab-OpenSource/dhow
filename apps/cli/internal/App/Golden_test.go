package app

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var update = flag.Bool("update", false, "rewrite the golden files in testdata")

func run(args []string, env map[string]string) (string, string, int) {
	var out, errOut bytes.Buffer
	code := Main(args, Env{Getenv: func(k string) string { return env[k] }, Out: &out, Err: &errOut})
	return out.String(), errOut.String(), code
}

func TestGoldenOutput(t *testing.T) {
	fixtures := map[string]int{"valid": ExitValid, "warnings": ExitValid, "errors": ExitInvalid, "unknown-key": ExitInvalid}
	for name, want := range fixtures {
		for _, format := range []string{FormatText, FormatJSON} {
			for _, lang := range []string{"en", "ar"} {
				golden := filepath.Join("testdata", name+"."+format+"."+lang+".golden")
				t.Run(golden, func(t *testing.T) {
					path := filepath.Join("testdata", name+".dhow.yml")
					out, errOut, code := run([]string{"lint", "--format", format, "--lang", lang, path}, nil)
					assert.Equal(t, want, code)
					assert.Empty(t, errOut)
					if *update {
						require.NoError(t, os.WriteFile(golden, []byte(out), 0o600))
					}
					expected, err := os.ReadFile(golden)
					require.NoError(t, err, "run go test ./apps/cli/internal/App -update to create it")
					assert.Equal(t, string(expected), out)
				})
			}
		}
	}
}

func TestJSONIsStable(t *testing.T) {
	path := filepath.Join("testdata", "errors.dhow.yml")
	first, _, _ := run([]string{"lint", "--format=json", path}, nil)
	for range 20 {
		again, _, _ := run([]string{"lint", "--format=json", path}, nil)
		require.Equal(t, first, again)
	}
	var report map[string]any
	require.NoError(t, json.Unmarshal([]byte(first), &report))
	keys := make([]string, 0, len(report))
	for k := range report {
		keys = append(keys, k)
	}
	assert.ElementsMatch(t, []string{"SchemaVersion", "File", "Lang", "Valid", "Errors", "Warnings", "Issues"}, keys)
	assert.InDelta(t, 1, report["SchemaVersion"], 0)
	issue, ok := report["Issues"].([]any)[0].(map[string]any)
	require.True(t, ok)
	ikeys := make([]string, 0, len(issue))
	for k := range issue {
		ikeys = append(ikeys, k)
	}
	assert.ElementsMatch(t, []string{"Code", "Severity", "Path", "Line", "Column", "Message"}, ikeys)
}

func TestValidFileGivesAnEmptyIssueList(t *testing.T) {
	out, _, code := run([]string{"lint", "--format", "json", filepath.Join("testdata", "valid.dhow.yml")}, nil)
	assert.Equal(t, ExitValid, code)
	assert.Contains(t, out, `"Issues": []`)
}

func TestLanguageComesFromTheLocale(t *testing.T) {
	path := filepath.Join("testdata", "errors.dhow.yml")
	arabic, _, _ := run([]string{"lint", "--lang", "ar", path}, nil)
	english, _, _ := run([]string{"lint", "--lang", "en", path}, nil)
	cases := []struct {
		env  map[string]string
		want string
	}{
		{map[string]string{"LANG": "ar_EG.UTF-8"}, arabic},
		{map[string]string{"LC_ALL": "ar_SA.utf8", "LANG": "en_US.UTF-8"}, arabic},
		{map[string]string{"LC_MESSAGES": "ar_EG.UTF-8"}, arabic},
		{map[string]string{"LC_ALL": "en_US.UTF-8", "LANG": "ar_EG.UTF-8"}, english},
		{map[string]string{"LANG": "ar_EG"}, english},
		{map[string]string{"LANG": "C"}, english},
		{nil, english},
	}
	for _, tc := range cases {
		out, _, _ := run([]string{"lint", path}, tc.env)
		assert.Equal(t, tc.want, out, "%v", tc.env)
	}
	out, _, _ := run([]string{"lint", "--lang", "en", path}, map[string]string{"LANG": "ar_EG.UTF-8"})
	assert.Equal(t, english, out)
}
