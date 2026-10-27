package logging

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRedactsSensitiveKeys(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, slog.LevelDebug)
	log.Info("event",
		"password", "hunter2",
		"api_key", "ak_live_123",
		"Authorization", "Bearer abc",
		"DHOW_SECRET_KEY", "deadbeef",
		slog.Group("req", "cookie", "sid=xyz", "path", "/api/v1/meta"),
		"user", "sara",
	)
	out := buf.String()
	for _, leak := range []string{"hunter2", "ak_live_123", "Bearer abc", "deadbeef", "sid=xyz"} {
		assert.NotContains(t, out, leak)
	}
	for _, keep := range []string{`"user":"sara"`, `"path":"/api/v1/meta"`} {
		assert.Contains(t, out, keep)
	}
}

func TestIsSensitiveKey(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{"password", true},
		{"db_password", true},
		{"secretKey", true},
		{"access-token", true},
		{"session_id", true},
		{"key", true},
		{"ssh.private_key", true},
		{"user", false},
		{"path", false},
		{"status", false},
		{"keyboard", false},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			assert.Equal(t, tt.want, IsSensitiveKey(tt.key))
		})
	}
}
