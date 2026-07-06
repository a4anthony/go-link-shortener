// Package logger builds the application's structured slog logger. Production
// emits JSON; dev mode emits the same JSON handler (kept consistent so logs are
// machine-parseable everywhere) at the configured level.
package logger

import (
	"log/slog"
	"os"
	"strings"
)

// New returns a slog.Logger writing JSON to stdout at the given level
// (debug|info|warn|error). Unknown levels fall back to info.
func New(level string) *slog.Logger {
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: parseLevel(level),
	})
	return slog.New(h)
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
