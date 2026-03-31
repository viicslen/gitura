// Package logger provides a shared structured logger for the gitura application.
// It uses [log/slog] with a text handler writing to stderr.
// The log level is controlled by the GITURA_LOG_LEVEL environment variable:
//
//	GITURA_LOG_LEVEL=debug  — verbose (default in dev)
//	GITURA_LOG_LEVEL=info   — normal
//	GITURA_LOG_LEVEL=warn   — warnings and errors only
//	GITURA_LOG_LEVEL=error  — errors only
package logger

import (
	"log/slog"
	"os"
	"strings"
)

// L is the application-wide logger. Use it directly:
//
//	logger.L.Info("message", "key", value)
//	logger.L.Debug("polling", "status", result.Status)
var L *slog.Logger

func init() {
	level := parseLevel(os.Getenv("GITURA_LOG_LEVEL"))
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level:     level,
		AddSource: level == slog.LevelDebug,
	})
	L = slog.New(handler)
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelDebug // default to debug so dev builds are verbose
	}
}
