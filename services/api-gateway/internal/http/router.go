package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/freight-platform/api-gateway/internal/config"
	"github.com/freight-platform/api-gateway/internal/controltower"
	"github.com/freight-platform/api-gateway/internal/fleetrbac"
	gwmiddleware "github.com/freight-platform/api-gateway/internal/http/middleware"
	apperrors "github.com/freight-platform/api-gateway/internal/platform/errors"
	"github.com/freight-platform/api-gateway/internal/platform/respond"
	"github.com/freight-platform/api-gateway/internal/shipmentevents"
	"github.com/freight-platform/api-gateway/internal/shipmentrbac"
	"github.com/freight-platform/shared-go/metrics"
	sharedmiddleware "github.com/freight-platform/shared-go/middleware"
	"github.com/freight-platform/shared-go/observability"
	sharedpprof "github.com/freight-platform/shared-go/pprof"
)

const serviceName = "api-gateway"

func NewRouter(log *slog.Logger, cfg config.Config, proxy *ProxyHandler, controlTower *controltower.Handler, shipmentEvents *shipmentevents.Handler) http.Handler {
	metricsCollector := metrics.New(serviceName)

	r := chi.NewRouter()
	r.Use(sharedmiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(sharedmiddleware.Recover(log, serviceName))
	r.Use(sharedmiddleware.AccessLog(log, serviceName))
	r.Use(metricsCollector.Middleware)
	r.Use(gwmiddleware.MaxBodySize(cfg.MaxRequestBodyBytes))
	r.Use(gwmiddleware.RateLimit(cfg.RateLimitEnabled, cfg.RateLimitRPS, cfg.RateLimitBurst, serviceName))
	r.Use(gwmiddleware.CORS(cfg.CORSAllowedOrigins))
	r.Use(gwmiddleware.Auth(cfg.AuthEnabled, cfg.JWTSecret))

	sharedpprof.Mount(r)

	r.Get("/health", observability.HealthHandler(serviceName))
	r.Get("/ready", func(w http.ResponseWriter, req *http.Request) {
		status, httpStatus, services := ReadyStatus(req.Context(), cfg)
		respond.JSON(w, httpStatus, map[string]any{
			"status":   status,
			"services": services,
		})
	})
	r.Handle("/metrics", metricsCollector.Handler())

	r.Get("/routes", func(w http.ResponseWriter, _ *http.Request) {
		items := make([]map[string]string, 0, len(proxy.Routes()))
		for _, route := range proxy.Routes() {
			items = append(items, map[string]string{
				"prefix":  route.Prefix,
				"service": route.Service,
				"target":  RouteTarget(route),
			})
		}
		respond.JSON(w, http.StatusOK, map[string]any{"routes": items})
	})

	openAPI := NewOpenAPIHandler(cfg.OpenAPIDir)
	openAPI.RegisterRoutes(r)

	if controlTower != nil {
		r.Get("/api/v1/control-tower/summary", controlTower.Summary)
		r.Post("/api/v1/control-tower/critical-events/{eventId}/acknowledge", controlTower.AcknowledgeCriticalEvent)
		r.Post("/api/v1/control-tower/critical-events/{eventId}/assign", controlTower.AssignCriticalEvent)
		r.Post("/api/v1/control-tower/critical-events/{eventId}/resolve", controlTower.ResolveCriticalEvent)
		r.Post("/api/v1/control-tower/critical-events/{eventId}/reopen", controlTower.ReopenCriticalEvent)
		r.Patch("/api/v1/control-tower/critical-events/{eventId}/exception", controlTower.UpdateCriticalEventException)
		r.Get("/api/v1/control-tower/critical-events/{eventId}/actions", controlTower.GetCriticalEventActions)
		r.Get("/api/v1/control-tower/risks", controlTower.ListRisks)
		r.Get("/api/v1/control-tower/risks/{riskId}", controlTower.GetRisk)
		r.Post("/api/v1/control-tower/risks/{riskId}/acknowledge", controlTower.AcknowledgeRisk)
		r.Post("/api/v1/control-tower/risks/{riskId}/mitigate", controlTower.MitigateRisk)
	}

	if shipmentEvents != nil {
		r.Get("/api/v1/shipments/{shipmentId}/events", shipmentEvents.Events)
	}

	fleetGuard := fleetrbac.NewGuard(cfg, proxy)
	r.Get("/api/v1/drivers", fleetGuard.WithPolicy(fleetrbac.PolicyView))
	r.Get("/api/v1/drivers/{id}", fleetGuard.WithPolicy(fleetrbac.PolicyView))
	r.Post("/api/v1/drivers", fleetGuard.WithPolicy(fleetrbac.PolicyCreate))
	r.Get("/api/v1/vehicles", fleetGuard.WithPolicy(fleetrbac.PolicyView))
	r.Get("/api/v1/vehicles/{id}", fleetGuard.WithPolicy(fleetrbac.PolicyView))
	r.Post("/api/v1/vehicles", fleetGuard.WithPolicy(fleetrbac.PolicyCreate))
	r.Post("/api/v1/shipments/{id}/assign-driver", fleetGuard.WithPolicy(fleetrbac.PolicyAssign))
	r.Post("/api/v1/shipments/{id}/assign-vehicle", fleetGuard.WithPolicy(fleetrbac.PolicyAssign))

	shipmentGuard := shipmentrbac.NewGuard(cfg, proxy)
	r.Post("/api/v1/shipments/from-transport-order", shipmentGuard.WithPolicy(shipmentrbac.PolicyCreate))
	r.Post("/api/v1/shipments/from-bid", shipmentGuard.WithPolicy(shipmentrbac.PolicyCreate))
	r.Post("/api/v1/shipments/{id}/accept", shipmentGuard.WithPolicy(shipmentrbac.PolicyAccept))
	r.Patch("/api/v1/shipments/{id}/status", shipmentGuard.WithPolicy(shipmentrbac.PolicyUpdateStatus))
	r.Post("/api/v1/shipments/{id}/cancel", shipmentGuard.WithPolicy(shipmentrbac.PolicyCancel))

	r.Handle("/api/*", proxy)
	r.Handle("/api", proxy)

	r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		respond.Error(w, apperrors.RouteNotFound("no route found for path"))
	})

	return r
}
