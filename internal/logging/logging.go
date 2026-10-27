package logging

import (
	"io"
	"log/slog"
	"strings"
)

const Redacted = "[REDACTED]"

var sensitiveParts = []string{
	"password", "passwd", "secret", "token", "authorization", "cookie", "apikey", "privatekey", "credential", "session",
}

func IsSensitiveKey(key string) bool {
	k := strings.ToLower(key)
	k = strings.NewReplacer("_", "", "-", "", ".", "").Replace(k)
	if k == "key" || strings.HasSuffix(k, "key") {
		return true
	}
	for _, p := range sensitiveParts {
		if strings.Contains(k, p) {
			return true
		}
	}
	return false
}

func New(w io.Writer, level slog.Leveler) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level:       level,
		ReplaceAttr: redact,
	}))
}

func redact(_ []string, a slog.Attr) slog.Attr {
	if a.Value.Kind() != slog.KindGroup && IsSensitiveKey(a.Key) {
		return slog.String(a.Key, Redacted)
	}
	return a
}
