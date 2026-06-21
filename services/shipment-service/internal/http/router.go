package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/shipment-service/internal/http/handlers"
	"github.com/freight-platform/shipment-service/internal/service"
	"github.com/freight-platform/shared-go/metrics"
	"github.com/freight-platform/shared-go/observability"
	sharedpprof "github.com/freight-platform/shared-go/pprof"
)

const serviceName = "shipment-service"

func NewRouter(
	log *slog.Logger,
	db observability.DatabasePinger,
	shipmentSvc *service.ShipmentService,
	driverSvc *service.DriverService,
	vehicleSvc *service.VehicleService,
) http.Handler {
	shipmentHandler := handlers.NewShipmentHandler(shipmentSvc)
	driverHandler := handlers.NewDriverHandler(driverSvc)
	vehicleHandler := handlers.NewVehicleHandler(vehicleSvc)

	r := chi.NewRouter()
	observability.Mount(r, observability.MountOptions{
		ServiceName: serviceName,
		Log:         log,
		Metrics:     metrics.New(serviceName),
		DB:          db,
	})
	sharedpprof.Mount(r)

	r.Route("/v1/shipments", func(r chi.Router) {
		r.Post("/from-transport-order", shipmentHandler.CreateFromTransportOrder)
		r.Post("/from-bid", shipmentHandler.CreateFromBid)
		r.Get("/", shipmentHandler.List)
		r.Get("/{id}", shipmentHandler.GetByID)
		r.Post("/{id}/assign-driver", shipmentHandler.AssignDriver)
		r.Post("/{id}/assign-vehicle", shipmentHandler.AssignVehicle)
		r.Post("/{id}/accept", shipmentHandler.Accept)
		r.Patch("/{id}/status", shipmentHandler.UpdateStatus)
		r.Post("/{id}/cancel", shipmentHandler.Cancel)
	})

	r.Route("/v1/drivers", func(r chi.Router) {
		r.Post("/", driverHandler.Create)
		r.Get("/", driverHandler.List)
		r.Get("/{id}", driverHandler.GetByID)
	})

	r.Route("/v1/vehicles", func(r chi.Router) {
		r.Post("/", vehicleHandler.Create)
		r.Get("/", vehicleHandler.List)
		r.Get("/{id}", vehicleHandler.GetByID)
	})

	return r
}
