package http

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/freight-platform/analytics-service/internal/config"
	"github.com/freight-platform/analytics-service/internal/platform/metrics"
	"github.com/freight-platform/analytics-service/internal/platform/respond"
	sharedmiddleware "github.com/freight-platform/shared-go/middleware"
	"github.com/freight-platform/shared-go/observability"
)

const serviceName = "analytics-service"

func NewRouter(log *slog.Logger, cfg config.Config, source FoundationSource) http.Handler {
	stats := metrics.New()
	handler := NewHandler(log, source, stats)
	r := chi.NewRouter()
	r.Use(sharedmiddleware.RequestID)
	r.Use(chimiddleware.Recoverer)
	r.Get("/health", observability.HealthHandler(serviceName))
	r.Get("/ready", func(w http.ResponseWriter, req *http.Request) {
		if cfg.InternalToken == "" || cfg.ShipmentURL == "" || cfg.Environment == "" {
			respond.JSON(w, http.StatusServiceUnavailable, map[string]any{"status": "not_ready", "service": serviceName})
			return
		}
		ctx, cancel := context.WithTimeout(req.Context(), cfg.SourceTimeout)
		defer cancel()
		if err := source.Ready(ctx); err != nil {
			respond.JSON(w, http.StatusServiceUnavailable, map[string]any{"status": "not_ready", "service": serviceName})
			return
		}
		respond.JSON(w, http.StatusOK, map[string]any{"status": "ok", "service": serviceName})
	})
	r.Handle("/metrics", stats.Handler())
	r.With(requireGateway(cfg.InternalToken)).Get("/v1/analytics/kpis/{kpiId}", handler.GetKPI)
	return r
}
