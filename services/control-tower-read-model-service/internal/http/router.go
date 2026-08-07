package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/control-tower-read-model-service/internal/consumer"
	"github.com/freight-platform/control-tower-read-model-service/internal/http/handlers"
	"github.com/freight-platform/control-tower-read-model-service/internal/repository"
	"github.com/freight-platform/shared-go/metrics"
	"github.com/freight-platform/shared-go/observability"
	sharedpprof "github.com/freight-platform/shared-go/pprof"
)

const serviceName = "control-tower-read-model-service"

func NewRouter(
	log *slog.Logger,
	db observability.DatabasePinger,
	repo *repository.ProjectionRepository,
	freshness *consumer.Freshness,
) http.Handler {
	statusHandler := handlers.NewStatusHandler(repo, freshness)

	r := chi.NewRouter()
	observability.Mount(r, observability.MountOptions{
		ServiceName: serviceName,
		Log:         log,
		Metrics:     metrics.New(serviceName),
		DB:          db,
	})
	sharedpprof.Mount(r)

	r.Route("/internal/v1/control-tower", func(r chi.Router) {
		r.Get("/shipments/{shipmentId}/status", statusHandler.GetShipmentStatus)
		r.Get("/status-summary", statusHandler.GetStatusSummary)
		r.Get("/shipments/statuses", statusHandler.ListShipmentStatuses)
	})

	return r
}
