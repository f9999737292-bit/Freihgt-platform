package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/freight-platform/network-optimizer-service/internal/http/handlers"
	"github.com/freight-platform/network-optimizer-service/internal/service"
	sharedmiddleware "github.com/freight-platform/shared-go/middleware"
	"github.com/freight-platform/shared-go/observability"
)

const serviceName = "network-optimizer-service"

func NewRouter(log *slog.Logger, svc *service.Service, ready func(http.ResponseWriter, *http.Request)) http.Handler {
	r := chi.NewRouter()
	r.Use(sharedmiddleware.RequestID)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			req.Body = http.MaxBytesReader(w, req.Body, 1<<20)
			next.ServeHTTP(w, req)
		})
	})
	h := handlers.New(log, svc)
	r.Get("/health", observability.HealthHandler(serviceName))
	if ready == nil {
		ready = func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}
	}
	r.Get("/ready", ready)
	r.Handle("/metrics", promhttp.Handler())

	r.Post("/v1/network/load-opportunities", h.CreateLoad)
	r.Get("/v1/network/load-opportunities", h.ListOwnLoads)
	r.Get("/v1/network/load-opportunities/{id}", h.GetOwnLoad)
	r.Patch("/v1/network/load-opportunities/{id}", h.UpdateLoad)
	r.Post("/v1/network/load-opportunities/{id}/publish", h.PublishLoad)
	r.Post("/v1/network/load-opportunities/{id}/withdraw", h.WithdrawLoad)

	r.Post("/v1/network/capacities", h.CreateCapacity)
	r.Get("/v1/network/capacities", h.ListOwnCapacities)
	r.Get("/v1/network/capacities/{id}", h.GetOwnCapacity)
	r.Patch("/v1/network/capacities/{id}", h.UpdateCapacity)
	r.Post("/v1/network/capacities/{id}/withdraw", h.WithdrawCapacity)

	r.Get("/v1/network/marketplace/load-opportunities", h.ListMarketplaceLoads)
	r.Get("/v1/network/marketplace/load-opportunities/{id}", h.GetMarketplaceLoad)
	r.Get("/v1/network/marketplace/capacities", h.ListMarketplaceCapacities)
	r.Get("/v1/network/marketplace/capacities/{id}", h.GetMarketplaceCapacity)
	return r
}
