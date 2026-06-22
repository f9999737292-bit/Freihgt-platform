package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/freight-platform/low-code-service/internal/http/handlers"
	"github.com/freight-platform/low-code-service/internal/platform/database"
	"github.com/freight-platform/shared-go/metrics"
	sharedmiddleware "github.com/freight-platform/shared-go/middleware"
)

const serviceName = "low-code-service"

func NewRouter(log *slog.Logger, readiness *database.ReadinessChecker) http.Handler {
	metricsCollector := metrics.New(serviceName)

	r := chi.NewRouter()
	r.Use(sharedmiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(sharedmiddleware.Recover(log, serviceName))
	r.Use(sharedmiddleware.AccessLog(log, serviceName))
	r.Use(metricsCollector.Middleware)

	r.Get("/health", handlers.Health())
	r.Get("/ready", handlers.Ready(readiness))
	r.Handle("/metrics", metricsCollector.Handler())

	return r
}
