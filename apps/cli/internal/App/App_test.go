package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	output "github.com/Khaymah/dhow/apps/cli/internal/Output"
	"github.com/Khaymah/dhow/internal/pipelinespec"
	"github.com/Khaymah/dhow/internal/pipelinespec/pipelinespectest"
)

func TestTheSameBadFilesGiveTheSameCodes(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range pipelinespectest.Invalid() {
		t.Run(tc.Name, func(t *testing.T) {
			path := filepath.Join(dir, strings.ReplaceAll(tc.Name, " ", "-")+".yml")
			require.NoError(t, os.WriteFile(path, []byte(tc.Source), 0o600))
			out, errOut, code := run([]string{"lint", "--format", "json", path}, nil)
			require.Empty(t, errOut)
			assert.Equal(t, ExitInvalid, code)
			var report output.Report
			require.NoError(t, json.Unmarshal([]byte(out), &report))
			_, want := pipelinespec.Check([]byte(tc.Source))
			got := make([]pipelinespec.Issue, 0, len(report.Issues))
			codes := []string{}
			for _, i := range report.Issues {
				got = append(got, pipelinespec.Issue{
					Code: i.Code, Severity: pipelinespec.Severity(i.Severity), Path: i.Path,
					Position: pipelinespec.Position{Line: i.Line, Column: i.Column}, Message: i.Message,
				})
				codes = append(codes, i.Code)
			}
			assert.Equal(t, want, got)
			assert.Contains(t, codes, tc.Code)
			assert.False(t, report.Valid)
		})
	}
}

func TestExitCodes(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "link.yml")
	require.NoError(t, os.Symlink(filepath.Join("testdata", "valid.dhow.yml"), link))
	unreadable := filepath.Join(dir, "secret.yml")
	require.NoError(t, os.WriteFile(unreadable, []byte("version: 1\n"), 0o000))
	cases := []struct {
		name string
		args []string
		code int
		err  string
	}{
		{"valid", []string{"lint", "testdata/valid.dhow.yml"}, ExitValid, ""},
		{"warnings only", []string{"lint", "testdata/warnings.dhow.yml"}, ExitValid, ""},
		{"errors", []string{"lint", "testdata/errors.dhow.yml"}, ExitInvalid, ""},
		{"missing file", []string{"lint", filepath.Join(dir, "nope.yml")}, ExitUsage, "the file does not exist"},
		{"folder without a pipeline", []string{"lint", dir}, ExitUsage, "the file does not exist"},
		{"not a regular file", []string{"lint", os.DevNull}, ExitUsage, "not a regular file"},
		{"symbolic link", []string{"lint", link}, ExitUsage, "this is a symbolic link"},
		{"no command", nil, ExitUsage, "Usage:"},
		{"unknown command", []string{"deploy"}, ExitUsage, "deploy is not a command"},
		{"unknown option", []string{"lint", "--strict"}, ExitUsage, "--strict is not an option"},
		{"option without value", []string{"lint", "--format"}, ExitUsage, "--format needs a value"},
		{"bad format", []string{"lint", "--format", "yaml"}, ExitUsage, "--format is text or json"},
		{"bad lang", []string{"lint", "--lang=fr"}, ExitUsage, "--lang is en or ar"},
		{"two paths", []string{"lint", "a.yml", "b.yml"}, ExitUsage, "one path only"},
	}
	if os.Geteuid() != 0 {
		cases = append(cases, struct {
			name string
			args []string
			code int
			err  string
		}{"no permission", []string{"lint", unreadable}, ExitUsage, "permission"})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, errOut, code := run(tc.args, nil)
			assert.Equal(t, tc.code, code)
			assert.Contains(t, errOut, tc.err)
		})
	}
}

func TestAFolderMeansItsPipelineFile(t *testing.T) {
	dir := t.TempDir()
	src, err := os.ReadFile(filepath.Join("testdata", "errors.dhow.yml"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".dhow.yml"), src, 0o600))
	out, _, code := run([]string{"lint", dir}, nil)
	assert.Equal(t, ExitInvalid, code)
	assert.Contains(t, out, filepath.Join(dir, ".dhow.yml")+":9:9: error RUN_EXPRESSION_NOT_ALLOWED")
	t.Chdir(dir)
	out, _, code = run([]string{"lint"}, nil)
	assert.Equal(t, ExitInvalid, code)
	assert.True(t, strings.HasPrefix(out, ".dhow.yml:"), out)
}

func TestOptionsMayFollowThePath(t *testing.T) {
	path := filepath.Join("testdata", "errors.dhow.yml")
	before, _, _ := run([]string{"lint", "--format", "json", "--lang", "ar", path}, nil)
	after, _, code := run([]string{"lint", path, "--lang=ar", "-format", "json"}, nil)
	assert.Equal(t, ExitInvalid, code)
	assert.Equal(t, before, after)
	_, errOut, code := run([]string{"lint", "--", "--format"}, nil)
	assert.Equal(t, ExitUsage, code)
	assert.Contains(t, errOut, "--format: the file does not exist")
}

func TestHelpAndVersion(t *testing.T) {
	for _, args := range [][]string{{"help"}, {"--help"}, {"-h"}, {"lint", "--help"}, {"lint", "--bogus", "-h"}} {
		out, errOut, code := run(args, nil)
		assert.Equal(t, ExitValid, code, args)
		assert.Empty(t, errOut)
		assert.Contains(t, out, "dhow lint [--format text|json] [--lang en|ar] [path]")
		assert.Contains(t, out, "Exit codes:")
	}
	out, _, code := run([]string{"help"}, map[string]string{"LANG": "ar_EG.UTF-8"})
	assert.Equal(t, ExitValid, code)
	assert.Contains(t, out, "أكواد الخروج:")
	out, _, code = run([]string{"version"}, nil)
	assert.Equal(t, ExitValid, code)
	assert.True(t, strings.HasPrefix(out, "dhow "), out)
}

func TestErrorsAreTranslated(t *testing.T) {
	_, errOut, code := run([]string{"lint", "--lang", "ar", "nope.yml"}, nil)
	assert.Equal(t, ExitUsage, code)
	assert.Contains(t, errOut, "الملف غير موجود")
	link := filepath.Join(t.TempDir(), ".dhow.yml")
	require.NoError(t, os.Symlink(os.DevNull, link))
	_, errOut, code = run([]string{"lint", "--lang", "ar", link}, nil)
	assert.Equal(t, ExitUsage, code)
	assert.Contains(t, errOut, "رابط رمزي")
	_, errOut, _ = run([]string{"lint", "--bogus", "--lang", "ar"}, nil)
	assert.Contains(t, errOut, "ليس خيارًا")
}

func TestHostileNamesCannotDriveTheTerminal(t *testing.T) {
	dir := t.TempDir()
	src := pipelinespectest.Job("    \"x\\e]0;owned\\a\\u202e\": 1\n    steps: [{run: x}]\n")
	path := filepath.Join(dir, "evil\x1b[2J.yml")
	require.NoError(t, os.WriteFile(path, []byte(src), 0o600))
	out, _, code := run([]string{"lint", path}, nil)
	assert.Equal(t, ExitInvalid, code)
	assert.Contains(t, out, "UNKNOWN_KEY")
	assert.NotContains(t, out, "\x1b")
	assert.NotContains(t, out, "\x07")
	assert.NotContains(t, out, "\u202e")
	assert.Contains(t, out, `\u001b`)
	assert.Contains(t, out, `\u202e`)
}
