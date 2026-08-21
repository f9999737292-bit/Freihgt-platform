package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/freight-cost-service/internal/config"
	"github.com/freight-platform/freight-cost-service/internal/http/handlers"
	fcmetrics "github.com/freight-platform/freight-cost-service/internal/platform/metrics"
	"github.com/freight-platform/freight-cost-service/internal/service"
	"github.com/freight-platform/shared-go/internalauth"
	"github.com/freight-platform/shared-go/metrics"
	"github.com/freight-platform/shared-go/observability"
	sharedpprof "github.com/freight-platform/shared-go/pprof"
)

const serviceName = "freight-cost-service"

func NewRouter(
	log *slog.Logger,
	cfg config.Config,
	costSvc *service.CostService,
	domainMetrics *fcmetrics.Metrics,
) http.Handler {
	costHandler := handlers.NewCostHandler(costSvc)
	internalAuth := internalauth.Config{Token: cfg.InternalServiceToken, Environment: cfg.Environment}

	r := chi.NewRouter()
	if domainMetrics != nil {
		r.Use(domainMetrics.Middleware)
	}
	observability.Mount(r, observability.MountOptions{
		ServiceName: serviceName,
		Log:         log,
		Metrics:     metrics.New(serviceName),
		DB:          nil,
	})
	sharedpprof.Mount(r)

	r.Route("/internal/v1/freight-cost", func(r chi.Router) {
		r.Use(internalAuth.Middleware)
		r.Get("/transport-orders/{transportOrderId}", costHandler.GetTransportOrderCostSummary)
	})

	return r
}
