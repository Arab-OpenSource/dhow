package installer

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMainFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"no command", nil, "expected one of"},
		{"unknown command", []string{"upgrade"}, "expected one of"},
		{"unknown flag", []string{"install", "--force"}, "flag provided but not defined"},
		{"fix is doctor only", []string{"install", "--fix"}, "flag provided but not defined"},
		{"purge is uninstall only", []string{"doctor", "--purge"}, "flag provided but not defined"},
		{"extra argument", []string{"doctor", "now"}, "unexpected argument"},
		{"bad lang", []string{"install", "--lang", "fr", "--dry-run"}, "--lang must be en or ar"},
		{"help", []string{"install", "--help"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var errOut bytes.Buffer
			app, _ := newApp(debianHost(), "")
			err := Main(context.Background(), tt.args, app, &errOut)
			if tt.wantErr == "" {
				require.NoError(t, err)
				assert.Contains(t, errOut.String(), "-answers")
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestMainRunsADryRunInArabic(t *testing.T) {
	f := debianHost()
	f.root = false
	f.put("/root/a.yaml", ciAnswers, 0o600)
	app, out := newApp(f, "")
	require.NoError(t, Main(context.Background(), []string{"install", "--answers", "/root/a.yaml", "--dry-run", "--lang", "ar"}, app, &bytes.Buffer{}))
	assert.Contains(t, out.String(), "الخطة:")
	assert.Contains(t, out.String(), "تجربة بس: مفيش حاجة اتغيّرت.")
}
