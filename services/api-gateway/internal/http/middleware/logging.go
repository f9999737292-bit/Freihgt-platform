package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

const TargetServiceKey contextKey = "target_service"

func WithTargetService(ctx context.Context, service string) context.Context {
	return context.WithValue(ctx, TargetServiceKey, service)
}

func TargetServiceFromContext(ctx context.Context) string {
	if value, ok := ctx.Value(TargetServiceKey).(string); ok {
		return value
	}
	return ""
}

func Logging(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wrapped := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(wrapped, r)

			durationMS := time.Since(start).Milliseconds()
			log.Info("request completed",
				slog.String("request_id", RequestIDFromContext(r.Context())),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", wrapped.status),
				slog.Int64("duration_ms", durationMS),
				slog.String("remote_addr", r.RemoteAddr),
				slog.String("target_service", TargetServiceFromContext(r.Context())),
			)
		})
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(statusCode int) {
	w.status = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}
