package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/shared-go/metrics"
	"github.com/freight-platform/shared-go/observability"
	sharedpprof "github.com/freight-platform/shared-go/pprof"
	"github.com/freight-platform/tracking-service/internal/http/handlers"
)

const serviceName = "tracking-service"

func NewRouter(
	log *slog.Logger,
	db observability.DatabasePinger,
	trackingHandler *handlers.TrackingHandler,
	etaHandler *handlers.ETAHandler,
	etaInternal *handlers.ETAInternalHandler,
	slotHandler *handlers.SlotHandler,
	slotInternal *handlers.SlotInternalHandler,
	ingestHandler *handlers.IngestHandler,
	internalHandler *handlers.InternalHandler,
	metricsCollector *metrics.Collector,
) http.Handler {
	r := chi.NewRouter()
	observability.Mount(r, observability.MountOptions{
		ServiceName: serviceName,
		Log:         log,
		Metrics:     metricsCollector,
		DB:          db,
	})
	sharedpprof.Mount(r)

	r.Route("/v1/shipments/{shipmentId}", func(r chi.Router) {
		r.Get("/tracking", trackingHandler.GetCurrent)
		r.Get("/tracking/locations", trackingHandler.ListLocations)
		r.Get("/eta", etaHandler.GetCurrent)
		r.Get("/eta/history", etaHandler.ListHistory)
		r.Get("/slots", slotHandler.GetCurrent)
		r.Get("/slots/history", slotHandler.ListHistory)
	})

	r.Route("/internal/v1/tracking", func(r chi.Router) {
		r.Post("/providers/{provider}/locations", ingestHandler.Ingest)
		r.Post("/providers/{provider}/eta", ingestHandler.IngestETA)
		r.Post("/providers/{provider}/slots", ingestHandler.IngestSlots)
		r.Post("/states/lookup", internalHandler.LookupStates)
		r.Post("/eta/lookup", etaInternal.LookupDelivery)
		r.Post("/slots/lookup", slotInternal.Lookup)
	})

	return r
}
