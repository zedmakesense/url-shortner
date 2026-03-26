package logger

import (
	"log/slog"
	"os"

	"github.com/zedmakesense/url-shortner/internal/config"
)

func NewLogger(cfg config.LogConfig) *slog.Logger {
	var handler slog.Handler

	opts := &slog.HandlerOptions{
		Level:     cfg.Level,
		AddSource: cfg.AddSource,
	}
	if cfg.Format == "text" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}
	return slog.New(handler)
}
