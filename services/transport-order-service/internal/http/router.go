package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/transport-order-service/internal/http/handlers"
	"github.com/freight-platform/transport-order-service/internal/service"
	"github.com/freight-platform/shared-go/metrics"
	"github.com/freight-platform/shared-go/observability"
	sharedpprof "github.com/freight-platform/shared-go/pprof"
)

const serviceName = "transport-order-service"

func NewRouter(log *slog.Logger, db observability.DatabasePinger, svc *service.TransportOrderService) http.Handler {
	handler := handlers.NewHandler(svc)

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
		r.Post("/", handler.CreateTransportOrder)
		r.Get("/", handler.ListTransportOrders)
		r.Get("/{id}", handler.GetTransportOrder)
		r.Patch("/{id}", handler.UpdateTransportOrder)
		r.Post("/{id}/submit", handler.SubmitTransportOrder)
		r.Post("/{id}/cancel", handler.CancelTransportOrder)
	})

	return r
}
