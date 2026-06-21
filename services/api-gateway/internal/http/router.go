package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/freight-platform/api-gateway/internal/config"
	gwmiddleware "github.com/freight-platform/api-gateway/internal/http/middleware"
	apperrors "github.com/freight-platform/api-gateway/internal/platform/errors"
	"github.com/freight-platform/api-gateway/internal/platform/respond"
	"github.com/freight-platform/shared-go/metrics"
	"github.com/freight-platform/shared-go/observability"
	sharedpprof "github.com/freight-platform/shared-go/pprof"
	sharedmiddleware "github.com/freight-platform/shared-go/middleware"
)

const serviceName = "api-gateway"

func NewRouter(log *slog.Logger, cfg config.Config, proxy *ProxyHandler) http.Handler {
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

	r.Handle("/api/*", proxy)
	r.Handle("/api", proxy)

	r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		respond.Error(w, apperrors.RouteNotFound("no route found for path"))
	})

	return r
}
