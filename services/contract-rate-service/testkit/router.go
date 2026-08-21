//go:build integration

package testkit

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/contract-rate-service/internal/config"
	"github.com/freight-platform/contract-rate-service/internal/http/handlers"
	"github.com/freight-platform/contract-rate-service/internal/service"
	"github.com/freight-platform/shared-go/internalauth"
)

func newIntegrationRouter(
	cfg config.Config,
	contractSvc *service.ContractService,
	rateCardSvc *service.RateCardService,
	rateLineSvc *service.RateLineService,
	rateComponentSvc *service.RateComponentService,
	resolutionSvc *service.ResolutionService,
	actors *handlers.ActorResolver,
) http.Handler {
	contractHandler := handlers.NewContractHandler(contractSvc, actors)
	rateCardHandler := handlers.NewRateCardHandler(rateCardSvc, actors)
	rateLineHandler := handlers.NewRateLineHandler(rateLineSvc, actors)
	rateComponentHandler := handlers.NewRateComponentHandler(rateComponentSvc, actors)
	resolutionHandler := handlers.NewResolutionHandler(resolutionSvc, actors)
	internalAuth := internalauth.Config{Token: cfg.InternalServiceToken, Environment: cfg.Environment}

	r := chi.NewRouter()
	r.Route("/internal/v1", func(r chi.Router) {
		r.Use(internalAuth.Middleware)
		r.Route("/transport-contracts", func(r chi.Router) {
			r.Post("/", contractHandler.Create)
			r.Get("/", contractHandler.List)
			r.Get("/{id}", contractHandler.Get)
			r.Patch("/{id}", contractHandler.Patch)
			r.Post("/{id}/activate", contractHandler.Activate)
			r.Post("/{id}/suspend", contractHandler.Suspend)
			r.Post("/{id}/reactivate", contractHandler.Reactivate)
			r.Post("/{id}/terminate", contractHandler.Terminate)
			r.Post("/{id}/cancel", contractHandler.Cancel)
			r.Post("/{contractId}/rate-cards", rateCardHandler.Create)
			r.Get("/{contractId}/rate-cards", rateCardHandler.List)
		})
		r.Route("/rate-cards", func(r chi.Router) {
			r.Get("/{id}", rateCardHandler.Get)
			r.Post("/{id}/versions", rateCardHandler.CreateVersion)
			r.Get("/{id}/versions", rateCardHandler.ListVersions)
		})
		r.Route("/rate-card-versions", func(r chi.Router) {
			r.Get("/{versionId}", rateCardHandler.GetVersion)
			r.Patch("/{versionId}", rateCardHandler.PatchVersion)
			r.Delete("/{versionId}", rateCardHandler.DiscardVersion)
			r.Post("/{versionId}/activate", rateCardHandler.ActivateVersion)
			r.Post("/{versionId}/rate-lines", rateLineHandler.Create)
			r.Get("/{versionId}/rate-lines", rateLineHandler.List)
		})
		r.Route("/rate-lines", func(r chi.Router) {
			r.Get("/{id}", rateLineHandler.Get)
			r.Patch("/{id}", rateLineHandler.Patch)
			r.Delete("/{id}", rateLineHandler.Delete)
			r.Post("/{lineId}/components", rateComponentHandler.Create)
			r.Get("/{lineId}/components", rateComponentHandler.List)
		})
		r.Route("/rate-components", func(r chi.Router) {
			r.Patch("/{id}", rateComponentHandler.Patch)
			r.Delete("/{id}", rateComponentHandler.Delete)
		})
		r.Post("/rates/resolve", resolutionHandler.Resolve)
	})
	return r
}
