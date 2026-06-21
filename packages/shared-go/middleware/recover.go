package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recover catches panics, logs them, and returns a generic 500 response.
func Recover(log *slog.Logger, serviceName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					log.Error("panic recovered",
						slog.String("level", "error"),
						slog.String("service", serviceName),
						slog.String("request_id", RequestIDFromContext(r.Context())),
						slog.String("method", r.Method),
						slog.String("path", r.URL.Path),
						slog.Any("panic", recovered),
						slog.String("stack", string(debug.Stack())),
						slog.String("message", "panic recovered"),
					)

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					_ = json.NewEncoder(w).Encode(map[string]any{
						"error": map[string]any{
							"code":    "INTERNAL_ERROR",
							"message": "internal server error",
							"details": map[string]any{},
						},
					})
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
