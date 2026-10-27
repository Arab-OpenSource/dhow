package config

import (
	"log/slog"
	"os"

	"github.com/Arab-OpenSource/dhow/internal/logging"
)

func NewLogger(cfg Config) *slog.Logger {
	return logging.New(os.Stdout, cfg.LogLevel)
}
