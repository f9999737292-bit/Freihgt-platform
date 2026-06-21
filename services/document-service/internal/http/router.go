package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/document-service/internal/http/handlers"
	"github.com/freight-platform/document-service/internal/service"
	"github.com/freight-platform/shared-go/metrics"
	"github.com/freight-platform/shared-go/observability"
)

const serviceName = "document-service"

func NewRouter(
	log *slog.Logger,
	db observability.DatabasePinger,
	documentSvc *service.DocumentService,
	signingSvc *service.SigningService,
) http.Handler {
	documentHandler := handlers.NewDocumentHandler(documentSvc)
	signingHandler := handlers.NewSigningHandler(signingSvc)

	r := chi.NewRouter()
	observability.Mount(r, observability.MountOptions{
		ServiceName: serviceName,
		Log:         log,
		Metrics:     metrics.New(serviceName),
		DB:          db,
	})

	r.Route("/v1/documents", func(r chi.Router) {
		r.Post("/", documentHandler.Create)
		r.Get("/", documentHandler.List)
		r.Get("/{id}", documentHandler.GetByID)
		r.Post("/{id}/versions", documentHandler.CreateVersion)
		r.Post("/{id}/files", documentHandler.AddFile)
		r.Post("/{id}/ready-for-signing", documentHandler.ReadyForSigning)
		r.Post("/{id}/signing-sessions", signingHandler.CreateSession)
		r.Post("/{id}/cancel", documentHandler.Cancel)
		r.Post("/{id}/archive", documentHandler.Archive)
	})

	r.Route("/v1/signing-sessions", func(r chi.Router) {
		r.Get("/{id}", signingHandler.GetSession)
		r.Post("/{id}/signatures", signingHandler.AddSignature)
	})

	return r
}
