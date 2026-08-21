//go:build integration

package contractratepublic

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/api-gateway/internal/config"
	"github.com/freight-platform/api-gateway/internal/contractrates"
	gwmiddleware "github.com/freight-platform/api-gateway/internal/http/middleware"
	"github.com/freight-platform/api-gateway/internal/ratesrbac"
)

func newTestGateway(log *slog.Logger, cfg config.Config) http.Handler {
	r := chi.NewRouter()
	r.Use(gwmiddleware.Auth(cfg.AuthEnabled, cfg.JWTSecret))

	contractRateHandler := contractrates.NewHandler(log, cfg)
	ratesGuard := ratesrbac.NewGuard(cfg, contractRateHandler)

	r.Get("/api/v1/transport-contracts", ratesGuard.WithPolicy(ratesrbac.PolicyRead))
	r.Post("/api/v1/transport-contracts", ratesGuard.WithPolicy(ratesrbac.PolicyCreateContract))
	r.Get("/api/v1/transport-contracts/{id}", ratesGuard.WithPolicy(ratesrbac.PolicyRead))
	r.Patch("/api/v1/transport-contracts/{id}", ratesGuard.WithPolicy(ratesrbac.PolicyEditContract))
	r.Post("/api/v1/transport-contracts/{id}/activate", ratesGuard.WithPolicy(ratesrbac.PolicyContractLifecycle))
	r.Post("/api/v1/transport-contracts/{id}/suspend", ratesGuard.WithPolicy(ratesrbac.PolicyContractLifecycle))
	r.Post("/api/v1/transport-contracts/{id}/reactivate", ratesGuard.WithPolicy(ratesrbac.PolicyContractLifecycle))
	r.Post("/api/v1/transport-contracts/{id}/terminate", ratesGuard.WithPolicy(ratesrbac.PolicyContractLifecycle))
	r.Post("/api/v1/transport-contracts/{id}/cancel", ratesGuard.WithPolicy(ratesrbac.PolicyContractLifecycle))
	r.Get("/api/v1/transport-contracts/{contractId}/rate-cards", ratesGuard.WithPolicy(ratesrbac.PolicyRead))
	r.Post("/api/v1/transport-contracts/{contractId}/rate-cards", ratesGuard.WithPolicy(ratesrbac.PolicyCreateRateCard))
	r.Get("/api/v1/rate-cards/{id}", ratesGuard.WithPolicy(ratesrbac.PolicyRead))
	r.Get("/api/v1/rate-cards/{id}/versions", ratesGuard.WithPolicy(ratesrbac.PolicyRead))
	r.Post("/api/v1/rate-cards/{id}/versions", ratesGuard.WithPolicy(ratesrbac.PolicyEditDraftRateVersion))
	r.Get("/api/v1/rate-card-versions/{id}", ratesGuard.WithPolicy(ratesrbac.PolicyRead))
	r.Patch("/api/v1/rate-card-versions/{id}", ratesGuard.WithPolicy(ratesrbac.PolicyEditDraftRateVersion))
	r.Delete("/api/v1/rate-card-versions/{id}", ratesGuard.WithPolicy(ratesrbac.PolicyEditDraftRateVersion))
	r.Post("/api/v1/rate-card-versions/{id}/activate", ratesGuard.WithPolicy(ratesrbac.PolicyActivateRateVersion))
	r.Get("/api/v1/rate-card-versions/{id}/rate-lines", ratesGuard.WithPolicy(ratesrbac.PolicyRead))
	r.Post("/api/v1/rate-card-versions/{id}/rate-lines", ratesGuard.WithPolicy(ratesrbac.PolicyEditDraftRate))
	r.Get("/api/v1/rate-lines/{id}", ratesGuard.WithPolicy(ratesrbac.PolicyRead))
	r.Patch("/api/v1/rate-lines/{id}", ratesGuard.WithPolicy(ratesrbac.PolicyEditDraftRate))
	r.Delete("/api/v1/rate-lines/{id}", ratesGuard.WithPolicy(ratesrbac.PolicyEditDraftRate))
	r.Get("/api/v1/rate-lines/{id}/components", ratesGuard.WithPolicy(ratesrbac.PolicyRead))
	r.Post("/api/v1/rate-lines/{id}/components", ratesGuard.WithPolicy(ratesrbac.PolicyEditDraftRate))
	r.Patch("/api/v1/rate-components/{id}", ratesGuard.WithPolicy(ratesrbac.PolicyEditDraftRate))
	r.Delete("/api/v1/rate-components/{id}", ratesGuard.WithPolicy(ratesrbac.PolicyEditDraftRate))
	r.Post("/api/v1/rates/resolve", ratesGuard.WithPolicy(ratesrbac.PolicySimulate))

	return r
}
