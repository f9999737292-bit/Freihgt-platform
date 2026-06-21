package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"
)

// New creates a JSON structured logger with service context.
func New(serviceName, level, environment string) *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: lvl,
		ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
			switch attr.Key {
			case slog.TimeKey:
				if t, ok := attr.Value.Any().(time.Time); ok {
					return slog.String("timestamp", t.UTC().Format(time.RFC3339))
				}
			case slog.LevelKey:
				return slog.String("level", strings.ToLower(attr.Value.String()))
			}
			return attr
		},
	})

	return slog.New(handler).With(
		slog.String("service", serviceName),
		slog.String("environment", environment),
	)
}

// WithRequestID adds request_id to log context when available.
func WithRequestID(ctx context.Context, log *slog.Logger, requestID string) *slog.Logger {
	if requestID == "" {
		return log
	}
	return log.With(slog.String("request_id", requestID))
}
