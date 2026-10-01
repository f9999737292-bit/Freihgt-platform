package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/shared-go/internalauth"
	"github.com/freight-platform/shared-go/metrics"
	"github.com/freight-platform/shared-go/observability"
	sharedpprof "github.com/freight-platform/shared-go/pprof"
	"github.com/freight-platform/shipment-service/internal/http/handlers"
	"github.com/freight-platform/shipment-service/internal/service"
)

const serviceName = "shipment-service"

func NewRouter(
	log *slog.Logger,
	db observability.DatabasePinger,
	shipmentSvc *service.ShipmentService,
	orderExecutionSvc *service.OrderExecutionService,
	statusHistorySvc *service.StatusHistoryService,
	statusSummarySvc *service.StatusSummaryService,
	driverSvc *service.DriverService,
	vehicleSvc *service.VehicleService,
	driverOpsSvc *service.DriverOperationsService,
	driverTaskSvc *service.DriverTaskService,
	driverStopSvc *service.DriverStopService,
	evidenceSvc *service.ExecutionEvidenceService,
	internalToken string,
	trackingContext handlers.TrackingContextReader,
) http.Handler {
	shipmentHandler := handlers.NewShipmentHandler(shipmentSvc)
	orderExecutionHandler := handlers.NewOrderExecutionHandler(orderExecutionSvc)
	statusHistoryHandler := handlers.NewStatusHistoryHandler(statusHistorySvc)
	statusSummaryHandler := handlers.NewStatusSummaryHandler(statusSummarySvc)
	driverHandler := handlers.NewDriverHandler(driverSvc)
	vehicleHandler := handlers.NewVehicleHandler(vehicleSvc)
	driverOpsHandler := handlers.NewDriverOperationsHandler(driverOpsSvc)
	driverTaskHandler := handlers.NewDriverTaskHandler(driverTaskSvc)
	driverStopHandler := handlers.NewDriverStopHandler(driverStopSvc)
	internalTaskHandler := handlers.NewInternalDriverTaskHandler(driverTaskSvc, internalToken)
	ownershipHandler := handlers.NewOwnershipInternalHandler(shipmentSvc)
	planningHandler := handlers.NewPlanningInternalHandler(shipmentSvc)
	predictionInputHandler := handlers.NewPredictionInputHandler(shipmentSvc)
	vehicleCapabilityHandler := handlers.NewVehicleCapabilityHandler(vehicleSvc)
	evidenceHandler := handlers.NewExecutionEvidenceHandler(evidenceSvc)
	trackingContextHandler := handlers.NewTrackingContextHandler(trackingContext)
	var successorHandler *handlers.SuccessorRevisionHandler
	if writer, ok := trackingContext.(handlers.SuccessorRevisionWriter); ok {
		successorHandler = handlers.NewSuccessorRevisionHandler(writer)
	}
	internalAuth := internalauth.Config{Token: internalToken}

	r := chi.NewRouter()
	observability.Mount(r, observability.MountOptions{
		ServiceName: serviceName,
		Log:         log,
		Metrics:     metrics.New(serviceName),
		DB:          db,
	})
	sharedpprof.Mount(r)

	r.Route("/v1/order-execution", func(r chi.Router) {
		r.Get("/buyer/transport-orders", orderExecutionHandler.ListBuyerOrders)
		r.Get("/carrier/transport-orders", orderExecutionHandler.ListCarrierOrders)
		r.Route("/transport-orders/{id}", func(r chi.Router) {
			r.Post("/execute", orderExecutionHandler.Execute)
			r.Get("/", orderExecutionHandler.GetExecution)
			r.Post("/start", orderExecutionHandler.StartExecution)
		})
	})
	r.Get("/v1/carrier/transport-orders", orderExecutionHandler.ListCarrierOrders)

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

	r.Route("/v1/driver/me", func(r chi.Router) {
		r.Get("/", driverOpsHandler.GetMe)
		r.Get("/shipments", driverOpsHandler.ListShipments)
		r.Get("/shipments/{id}", driverOpsHandler.GetShipment)
		r.Post("/shipments/{id}/events", driverOpsHandler.RecordEvent)
		r.Post("/shipments/{id}/exceptions", driverOpsHandler.ReportException)
		r.Post("/shipments/{id}/delays", driverOpsHandler.ReportDelay)
		r.Get("/tasks", driverTaskHandler.ListTasks)
		r.Get("/tasks/{taskId}", driverTaskHandler.GetTask)
		r.Post("/tasks/{taskId}/read", driverTaskHandler.MarkRead)
		r.Post("/tasks/{taskId}/acknowledge", driverTaskHandler.Acknowledge)
		r.Post("/tasks/{taskId}/responses", driverTaskHandler.SubmitResponse)
		r.Post("/devices", driverTaskHandler.RegisterDevice)
		r.Delete("/devices/{deviceId}", driverTaskHandler.RevokeDevice)
		r.Get("/stops", driverStopHandler.List)
		r.Post("/stops/{stopId}/arrive", driverStopHandler.Arrive)
		r.Post("/stops/{stopId}/start-service", driverStopHandler.StartService)
		r.Post("/stops/{stopId}/complete", driverStopHandler.Complete)
		r.Post("/stops/{stopId}/actions/{actionId}/confirm", driverStopHandler.Confirm)
		r.Post("/stops/{stopId}/actions/{actionId}/fail", driverStopHandler.Fail)
		r.Post("/stops/{stopId}/actions/{actionId}/delivery-disposition", driverStopHandler.ReportDisposition)
	})

	r.Route("/v1/vehicles", func(r chi.Router) {
		r.Post("/", vehicleHandler.Create)
		r.Get("/", vehicleHandler.List)
		r.Get("/{id}", vehicleHandler.GetByID)
	})

	r.Route("/internal/v1/shipments", func(r chi.Router) {
		r.Get("/status-summary", statusSummaryHandler.GetStatusSummary)
		r.Get("/{shipmentId}/status-history", statusHistoryHandler.List)
		r.With(internalAuth.Middleware).Get("/{shipmentId}/ownership", ownershipHandler.GetShipment)
		r.With(internalAuth.Middleware).Get("/{shipmentId}/planning-locations", planningHandler.GetShipment)
		r.With(internalAuth.Middleware).Get("/{shipmentId}/prediction-input", predictionInputHandler.Get)
		r.With(internalAuth.Middleware).Get("/{shipmentId}/execution-context", evidenceHandler.GetExecutionContext)
		r.With(internalAuth.Middleware).Get("/{shipmentId}/onboard-cargo", evidenceHandler.GetOnboardCargo)
	})

	r.Route("/internal/v1/vehicles", func(r chi.Router) {
		r.With(internalAuth.Middleware).Get("/{id}/capability", vehicleCapabilityHandler.Get)
	})

	r.With(internalAuth.Middleware).Get(
		"/internal/v1/transport-executions/{executionId}/stops/{stopId}/tracking-context",
		trackingContextHandler.Get,
	)
	if successorHandler != nil {
		r.With(internalAuth.Middleware).Post(
			"/internal/v1/transport-executions/{executionId}/successor-revision",
			successorHandler.Create,
		)
	}
	if dispositionWriter, ok := trackingContext.(handlers.DeliveryDispositionWriter); ok {
		dispositionHandler := handlers.NewDeliveryDispositionHandler(dispositionWriter)
		r.With(internalAuth.Middleware).Post(
			"/internal/v1/transport-executions/{executionId}/delivery-dispositions",
			dispositionHandler.Record,
		)
		r.With(internalAuth.Middleware).Get(
			"/internal/v1/transport-executions/{executionId}/delivery-dispositions",
			dispositionHandler.List,
		)
		r.With(internalAuth.Middleware).Post(
			"/internal/v1/delivery-dispositions/{caseId}/authorize-return",
			dispositionHandler.AuthorizeReturn,
		)
		r.With(internalAuth.Middleware).Post(
			"/internal/v1/delivery-dispositions/{caseId}/authorize-redirect",
			dispositionHandler.AuthorizeRedirect,
		)
		r.With(internalAuth.Middleware).Post(
			"/internal/v1/delivery-dispositions/{caseId}/hold",
			dispositionHandler.Hold,
		)
		r.With(internalAuth.Middleware).Post(
			"/internal/v1/delivery-dispositions/{caseId}/complete",
			dispositionHandler.Complete,
		)
	}

	r.Route("/internal/v1/driver", func(r chi.Router) {
		r.Post("/tasks", internalTaskHandler.CreateTask)
		r.Post("/tasks/{taskId}/cancel", internalTaskHandler.CancelTask)
	})

	return r
}
