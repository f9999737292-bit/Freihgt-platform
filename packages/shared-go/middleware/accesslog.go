package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

// AccessLog writes structured JSON access logs for each HTTP request.
func AccessLog(log *slog.Logger, serviceName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)

			attrs := []slog.Attr{
				slog.String("level", "info"),
				slog.String("service", serviceName),
				slog.String("request_id", RequestIDFromContext(r.Context())),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", ww.Status()),
				slog.Int64("duration_ms", time.Since(start).Milliseconds()),
				slog.String("remote_addr", r.RemoteAddr),
				slog.String("user_agent", r.UserAgent()),
				slog.String("message", "request completed"),
			}

			if target := TargetServiceFromContext(r.Context()); target != "" {
				attrs = append(attrs, slog.String("target_service", target))
			}

			log.LogAttrs(r.Context(), slog.LevelInfo, "request completed", attrs...)
		})
	}
}

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
