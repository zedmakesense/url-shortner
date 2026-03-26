package logger

import (
	"log/slog"
	"os"
)

func NewFromEnv() *slog.Logger {
	var handler slog.Handler
	var level slog.Level
	var addSource bool

	addSourceString := getEnv("LOG_ADDSOURCE", "true")
	if addSourceString == "false" {
		addSource = false
	} else {
		addSource = true
	}

	levelString := getEnv("LOG_LEVEL", "debug")
	switch levelString {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: addSource,
	}
	if getEnv("LOG_FORMAT", "text") == "text" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}
	return slog.New(handler)
}

func getEnv(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}
