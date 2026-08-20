package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/contract-rate-service/internal/config"
	"github.com/freight-platform/contract-rate-service/internal/http/handlers"
	"github.com/freight-platform/contract-rate-service/internal/service"
	"github.com/freight-platform/shared-go/internalauth"
	"github.com/freight-platform/shared-go/metrics"
	"github.com/freight-platform/shared-go/observability"
	sharedpprof "github.com/freight-platform/shared-go/pprof"
)

const serviceName = "contract-rate-service"

func NewRouter(
	log *slog.Logger,
	db observability.DatabasePinger,
	cfg config.Config,
	contractSvc *service.ContractService,
	rateCardSvc *service.RateCardService,
	actors *handlers.ActorResolver,
) http.Handler {
	contractHandler := handlers.NewContractHandler(contractSvc, actors)
	rateCardHandler := handlers.NewRateCardHandler(rateCardSvc, actors)
	internalAuth := internalauth.Config{Token: cfg.InternalServiceToken, Environment: cfg.Environment}

	r := chi.NewRouter()
	observability.Mount(r, observability.MountOptions{
		ServiceName: serviceName,
		Log:         log,
		Metrics:     metrics.New(serviceName),
		DB:          db,
	})
	sharedpprof.Mount(r)

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
		})
	})

	return r
}
