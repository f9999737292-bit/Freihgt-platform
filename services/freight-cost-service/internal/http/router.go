package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/freight-cost-service/internal/config"
	"github.com/freight-platform/freight-cost-service/internal/http/handlers"
	fcmetrics "github.com/freight-platform/freight-cost-service/internal/platform/metrics"
	"github.com/freight-platform/freight-cost-service/internal/repository"
	"github.com/freight-platform/freight-cost-service/internal/service"
	"github.com/freight-platform/shared-go/internalauth"
	"github.com/freight-platform/shared-go/metrics"
	"github.com/freight-platform/shared-go/observability"
	sharedpprof "github.com/freight-platform/shared-go/pprof"
)

const serviceName = "freight-cost-service"

func NewRouter(
	log *slog.Logger,
	db observability.DatabasePinger,
	cfg config.Config,
	costSvc *service.CostService,
	ingestSvc *service.IngestService,
	rebuildSvc *service.RebuildService,
	derivedSvc *service.DerivedProjectionService,
	workspaceSvc *service.WorkspaceService,
	mappingRepo *repository.ChargeCodeMappingRepository,
	domainMetrics *fcmetrics.Metrics,
) http.Handler {
	costHandler := handlers.NewCostHandler(costSvc)
	sourceHandler := handlers.NewSourceEventHandler(ingestSvc, rebuildSvc)
	varianceHandler := handlers.NewVarianceHandler(derivedSvc, mappingRepo)
	workspaceHandler := handlers.NewWorkspaceHandler(workspaceSvc)
	internalAuth := internalauth.Config{Token: cfg.InternalServiceToken, Environment: cfg.Environment}

	r := chi.NewRouter()
	if domainMetrics != nil {
		r.Use(domainMetrics.Middleware)
	}
	observability.Mount(r, observability.MountOptions{
		ServiceName: serviceName,
		Log:         log,
		Metrics:     metrics.New(serviceName),
		DB:          db,
	})
	sharedpprof.Mount(r)

	r.Route("/internal/v1/freight-cost", func(r chi.Router) {
		r.Use(internalAuth.Middleware)
		r.Get("/transport-orders/{transportOrderId}", costHandler.GetTransportOrderCostSummary)
		r.Post("/source-events", sourceHandler.PostSourceEvent)
		r.Post("/transport-orders/{transportOrderId}/rebuild", sourceHandler.RebuildTransportOrder)
		r.Post("/transport-orders/{transportOrderId}/reconcile", varianceHandler.ReconcileTransportOrder)
		r.Post("/transport-orders/{transportOrderId}/reclassify-attribution", varianceHandler.ReclassifyAttribution)
		r.Put("/charge-code-mappings", varianceHandler.PutChargeCodeMapping)
	})

	r.Route("/internal/v1/freight-costs", func(r chi.Router) {
		r.Use(internalAuth.Middleware)
		r.Get("/", workspaceHandler.List)
		r.Get("/summary", workspaceHandler.Summary)
		r.Get("/transport-orders/{transportOrderId}", workspaceHandler.Detail)
		r.Get("/transport-orders/{transportOrderId}/variance-detail", workspaceHandler.VarianceDetail)
		r.Get("/accessorials/summary", workspaceHandler.AccessorialSummary)
		r.Get("/carriers/performance", workspaceHandler.CarrierPerformance)
		r.Get("/lanes/performance", workspaceHandler.LanePerformance)
	})

	return r
}
