package logging

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
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
		if strings.Contains(out, leak) {
			t.Errorf("log leaks %q: %s", leak, out)
		}
	}
	for _, keep := range []string{`"user":"sara"`, `"path":"/api/v1/meta"`} {
		if !strings.Contains(out, keep) {
			t.Errorf("log lost %s: %s", keep, out)
		}
	}
}

func TestIsSensitiveKey(t *testing.T) {
	tests := map[string]bool{
		"password": true, "db_password": true, "secretKey": true, "access-token": true, "session_id": true,
		"key": true, "ssh.private_key": true, "user": false, "path": false, "status": false, "keyboard": false,
	}
	for k, want := range tests {
		if got := IsSensitiveKey(k); got != want {
			t.Errorf("IsSensitiveKey(%q) = %v, want %v", k, got, want)
		}
	}
}
