package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
)

func writeSecret(t *testing.T, dir, name, content string, mode os.FileMode) string {
	t.Helper()
	p := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	require.NoError(t, os.Chmod(p, mode))
	return p
}

func TestReadSecretFile(t *testing.T) {
	tests := []struct {
		name    string
		content string
		mode    os.FileMode
		want    string
		wantErr error
	}{
		{"plain", "value", 0o600, "value", nil},
		{"one trailing newline is trimmed", "value\n", 0o600, "value", nil},
		{"a trailing CRLF is trimmed", "value\r\n", 0o600, "value", nil},
		{"only one newline is trimmed", "value\n\n", 0o600, "value\n", nil},
		{"leading space is kept", " value", 0o400, " value", nil},
		{"group readable is allowed", "value", 0o640, "value", nil},
		{"world readable is rejected", "value", 0o644, "", config.ErrSecretFileShared},
		{"world writable is rejected", "value", 0o602, "", config.ErrSecretFileShared},
		{"world executable is rejected", "value", 0o601, "", config.ErrSecretFileShared},
		{"empty is rejected", "", 0o600, "", config.ErrSecretFileEmpty},
		{"a lone newline is empty", "\n", 0o600, "", config.ErrSecretFileEmpty},
		{"too large is rejected", strings.Repeat("a", config.MaxSecretFileBytes+1), 0o600, "", config.ErrSecretFileTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := writeSecret(t, t.TempDir(), "secret", tt.content, tt.mode)
			got, err := config.ReadSecretFile(p)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				if tt.content != "" {
					assert.NotContains(t, err.Error(), tt.content)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestReadSecretFileRejectsMissingAndDirectories(t *testing.T) {
	dir := t.TempDir()
	_, err := config.ReadSecretFile(filepath.Join(dir, "missing"))
	require.Error(t, err)
	_, err = config.ReadSecretFile(dir)
	require.ErrorIs(t, err, config.ErrSecretFileNotFile)
}

type secretCase struct {
	name     string
	env      map[string]string
	files    map[string]os.FileMode
	creds    map[string]string
	wantDB   string
	wantErrs []string
}

func secretCases() []secretCase {
	return []secretCase{
		{name: "values from variables", wantDB: goodDB},
		{
			name:  "both from files",
			env:   map[string]string{config.EnvSecretKey: "", config.EnvDatabaseURL: "", config.EnvSecretKeyFile: "key", config.EnvDatabaseURLFile: "db"},
			files: map[string]os.FileMode{"key": 0o600, "db": 0o400}, wantDB: goodDB,
		},
		{
			name:     "variable and file together",
			env:      map[string]string{config.EnvSecretKeyFile: "key"},
			files:    map[string]os.FileMode{"key": 0o600},
			wantErrs: []string{config.EnvSecretKey + ": is set more than once (" + config.EnvSecretKey + ", " + config.EnvSecretKeyFile + ")"},
		},
		{
			name:     "world-readable files",
			env:      map[string]string{config.EnvSecretKey: "", config.EnvDatabaseURL: "", config.EnvSecretKeyFile: "key", config.EnvDatabaseURLFile: "db"},
			files:    map[string]os.FileMode{"key": 0o644, "db": 0o604},
			wantErrs: []string{config.EnvSecretKey + ": ", config.EnvDatabaseURL + ": ", "readable or writable by other users", "chmod 600"},
		},
		{
			name:     "missing file",
			env:      map[string]string{config.EnvSecretKey: "", config.EnvSecretKeyFile: "nope"},
			wantErrs: []string{config.EnvSecretKey + ": read secret file"},
		},
		{
			name:  "systemd credentials",
			env:   map[string]string{config.EnvSecretKey: "", config.EnvDatabaseURL: ""},
			creds: map[string]string{config.CredentialSecretKey: goodKeyHex + "\n", config.CredentialDatabaseURL: goodDB + "\n"}, wantDB: goodDB,
		},
		{
			name:  "a credentials directory without dhow credentials falls back to variables",
			creds: map[string]string{"other": "x"}, wantDB: goodDB,
		},
		{
			name:     "variable and systemd credential together",
			env:      map[string]string{config.EnvSecretKey: ""},
			creds:    map[string]string{config.CredentialSecretKey: goodKeyHex, config.CredentialDatabaseURL: goodDB},
			wantErrs: []string{config.EnvDatabaseURL + ": is set more than once (" + config.EnvDatabaseURL + ", the systemd credential " + config.CredentialDatabaseURL + ")"},
		},
		{
			name:     "file contents are still validated",
			env:      map[string]string{config.EnvSecretKey: "", config.EnvSecretKeyFile: "weak"},
			files:    map[string]os.FileMode{"weak": 0o600},
			wantErrs: []string{config.EnvSecretKey + ": looks like a placeholder"},
		},
	}
}

func fileContent(name string) string {
	switch name {
	case "key":
		return goodKeyHex + "\n"
	case "db":
		return goodDB + "\n"
	default:
		return "change-me-placeholder-secret-value-0123456789"
	}
}

func TestLoadSecretSources(t *testing.T) {
	for _, tt := range secretCases() {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			m := valid()
			for k, v := range tt.env {
				m[k] = v
				if strings.HasSuffix(k, config.FileSuffix) {
					m[k] = filepath.Join(dir, v)
				}
			}
			for name, mode := range tt.files {
				writeSecret(t, dir, name, fileContent(name), mode)
			}
			if tt.creds != nil {
				credDir := filepath.Join(dir, "credentials")
				require.NoError(t, os.Mkdir(credDir, 0o700))
				for name, v := range tt.creds {
					writeSecret(t, credDir, name, v, 0o400)
				}
				m[config.EnvCredentialsDirectory] = credDir
			}
			cfg, err := config.Load(env(m))
			if tt.wantErrs != nil {
				require.Error(t, err)
				for _, want := range tt.wantErrs {
					assert.Contains(t, err.Error(), want)
				}
				assert.NotContains(t, err.Error(), goodKeyHex)
				assert.NotContains(t, err.Error(), "pw@")
				return
			}
			require.NoError(t, err)
			assert.Len(t, cfg.SecretKey, config.MinSecretKeyBytes)
			assert.Equal(t, tt.wantDB, cfg.DatabaseURL)
		})
	}
}
