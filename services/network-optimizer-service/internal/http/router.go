package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/freight-platform/network-optimizer-service/internal/http/handlers"
	"github.com/freight-platform/network-optimizer-service/internal/reference"
	"github.com/freight-platform/network-optimizer-service/internal/service"
	sharedmiddleware "github.com/freight-platform/shared-go/middleware"
	"github.com/freight-platform/shared-go/observability"
)

const serviceName = "network-optimizer-service"

func NewRouter(log *slog.Logger, svc *service.Service, ready func(http.ResponseWriter, *http.Request)) http.Handler {
	return NewRouterWithCatalog(log, svc, ready, nil)
}

func NewRouterWithCatalog(log *slog.Logger, svc *service.Service, ready func(http.ResponseWriter, *http.Request), catalog reference.Catalog) http.Handler {
	r := chi.NewRouter()
	r.Use(sharedmiddleware.RequestID)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			req.Body = http.MaxBytesReader(w, req.Body, 1<<20)
			next.ServeHTTP(w, req)
		})
	})
	h := handlers.New(log, svc)
	if catalog != nil {
		h.UseCatalog(catalog)
	}
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

	r.Post("/v1/network/shipments/{shipmentId}/predicted-capacity", h.GeneratePrediction)
	r.Get("/v1/network/predicted-capacities", h.ListPredictions)
	r.Get("/v1/network/predicted-capacities/{id}", h.GetPrediction)
	r.Post("/v1/network/predicted-capacities/{id}/refresh", h.RefreshPrediction)
	r.Post("/v1/network/predicted-capacities/{id}/activate", h.ActivatePrediction)

	r.Get("/v1/network/marketplace/load-opportunities", h.ListMarketplaceLoads)
	r.Get("/v1/network/marketplace/load-opportunities/{id}", h.GetMarketplaceLoad)
	r.Get("/v1/network/marketplace/capacities", h.ListMarketplaceCapacities)
	r.Get("/v1/network/marketplace/capacities/{id}", h.GetMarketplaceCapacity)

	r.Get("/v1/network/compatibility/cargo-types", h.ListCargoTypes)
	r.Get("/v1/network/compatibility/equipment-types", h.ListEquipmentTypes)
	r.Get("/v1/network/compatibility/pallet-types", h.ListPalletTypes)
	r.Get("/v1/network/compatibility/packaging-types", h.ListPackagingTypes)
	r.Get("/v1/network/compatibility/rule-sets", h.ListRuleSets)
	r.Post("/v1/network/compatibility/rule-sets", h.CreateRuleSet)
	r.Post("/v1/network/compatibility/rule-sets/{id}/rules", h.AddCompatibilityRule)
	r.Delete("/v1/network/compatibility/rule-sets/{id}/rules/{ruleCode}", h.RemoveCompatibilityRule)
	r.Post("/v1/network/compatibility/rule-sets/{id}/activate", h.ActivateRuleSet)
	r.Post("/v1/network/compatibility/rule-sets/{id}/retire", h.RetireRuleSet)
	r.Post("/v1/network/compatibility/cargo-equipment/evaluate", h.EvaluateCargoEquipment)
	r.Post("/v1/network/compatibility/groupage/evaluate", h.EvaluateGroupage)
	return r
}
