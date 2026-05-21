package logger

import (
	"log/slog"
	"os"
	"strings"
)

// New returns a JSON slog logger tagged with the given service name.
// Log level can be overridden via LOG_LEVEL env (debug|info|warn|error); defaults to info.
func New(serviceName string) *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToLower(os.Getenv("LOG_LEVEL")) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	return slog.New(h).With("service", serviceName)
}
