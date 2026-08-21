package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/transport-order-service/internal/config"
	"github.com/freight-platform/transport-order-service/internal/http/handlers"
	"github.com/freight-platform/transport-order-service/internal/service"
	"github.com/freight-platform/shared-go/internalauth"
	"github.com/freight-platform/shared-go/metrics"
	"github.com/freight-platform/shared-go/observability"
	sharedpprof "github.com/freight-platform/shared-go/pprof"
)

const serviceName = "transport-order-service"

func NewRouter(
	log *slog.Logger,
	db observability.DatabasePinger,
	cfg config.Config,
	svc *service.TransportOrderService,
	pricedSvc *service.PricedTransportOrderService,
	snapshotReadSvc *service.RateSnapshotReadService,
) http.Handler {
	handler := handlers.NewHandler(svc)
	pricedHandler := handlers.NewPricedTransportOrderHandler(pricedSvc)
	snapshotInternalHandler := handlers.NewRateSnapshotInternalHandler(snapshotReadSvc)
	internalAuth := internalauth.Config{Token: cfg.InternalServiceToken, Environment: cfg.Environment}

	r := chi.NewRouter()
	observability.Mount(r, observability.MountOptions{
		ServiceName: serviceName,
		Log:         log,
		Metrics:     metrics.New(serviceName),
		DB:          db,
	})
	sharedpprof.Mount(r)

	r.Route("/v1/locations", func(r chi.Router) {
		r.Post("/", handler.CreateLocation)
		r.Get("/", handler.ListLocations)
		r.Get("/{id}", handler.GetLocation)
	})

	r.Route("/v1/cargoes", func(r chi.Router) {
		r.Post("/", handler.CreateCargo)
		r.Get("/{id}", handler.GetCargo)
	})

	r.Route("/v1/transport-orders", func(r chi.Router) {
		r.Post("/", pricedHandler.CreatePricedTransportOrder)
		r.Get("/", handler.ListTransportOrders)
		r.Get("/{id}", handler.GetTransportOrder)
		r.Patch("/{id}", handler.UpdateTransportOrder)
		r.Post("/{id}/submit", handler.SubmitTransportOrder)
		r.Post("/{id}/cancel", handler.CancelTransportOrder)
	})

	r.Route("/internal/v1", func(r chi.Router) {
		r.Use(internalAuth.Middleware)
		r.Post("/transport-orders/from-award-scope", pricedHandler.CreateFromAwardScope)
		r.Get("/transport-orders/{transportOrderId}/rate-snapshot", snapshotInternalHandler.GetRateSnapshot)
	})

	return r
}
