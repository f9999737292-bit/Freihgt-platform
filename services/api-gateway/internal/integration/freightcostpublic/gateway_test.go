//go:build integration

package freightcostpublic

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/api-gateway/internal/config"
	"github.com/freight-platform/api-gateway/internal/freightcost"
	gwmiddleware "github.com/freight-platform/api-gateway/internal/http/middleware"
	"github.com/freight-platform/api-gateway/internal/freightcostrbac"
)

func newTestGateway(log *slog.Logger, cfg config.Config) http.Handler {
	r := chi.NewRouter()
	r.Use(gwmiddleware.Auth(cfg.AuthEnabled, cfg.JWTSecret))

	freightCostHandler := freightcost.NewHandler(log, cfg)
	freightCostGuard := freightcostrbac.NewGuard(cfg, freightCostHandler)

	r.Get("/api/v1/freight-costs", freightCostGuard.WithPolicy(freightcostrbac.PolicyRead))
	r.Get("/api/v1/freight-costs/summary", freightCostGuard.WithPolicy(freightcostrbac.PolicyRead))
	r.Get("/api/v1/freight-costs/transport-orders/{transportOrderId}", freightCostGuard.WithPolicy(freightcostrbac.PolicyRead))
	r.Get("/api/v1/freight-costs/transport-orders/{transportOrderId}/variance-detail", freightCostGuard.WithPolicy(freightcostrbac.PolicyBuyerAnalytics))
	r.Get("/api/v1/freight-costs/accessorials/summary", freightCostGuard.WithPolicy(freightcostrbac.PolicyBuyerAnalytics))
	r.Get("/api/v1/freight-costs/carriers/performance", freightCostGuard.WithPolicy(freightcostrbac.PolicyRead))
	r.Get("/api/v1/freight-costs/lanes/performance", freightCostGuard.WithPolicy(freightcostrbac.PolicyRead))

	return r
}
