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
	podSvc *service.PODUploadService,
	attachmentSvc *service.AttachmentService,
	shipperSvc *service.ShipperDocumentService,
	carrierSvc *service.CarrierDocumentService,
) http.Handler {
	documentHandler := handlers.NewDocumentHandler(documentSvc)
	signingHandler := handlers.NewSigningHandler(signingSvc)
	podHandler := handlers.NewPODUploadHandler(podSvc)
	attachmentHandler := handlers.NewAttachmentHandler(attachmentSvc)
	shipperHandler := handlers.NewShipperDocumentHandler(shipperSvc)
	carrierHandler := handlers.NewCarrierDocumentHandler(carrierSvc)

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
		r.Post("/{id}/attachments", attachmentHandler.Create)
		r.Get("/{id}/attachments/{attachmentId}", attachmentHandler.Get)
		r.Get("/{id}/attachments/{attachmentId}/content", attachmentHandler.Content)
		r.Post("/{id}/attachments/{attachmentId}/finalize", attachmentHandler.Finalize)
		r.Post("/{id}/attachments/{attachmentId}/signatures", attachmentHandler.AttachSignature)
		r.Get("/{id}/attachments/{attachmentId}/signatures/{signatureId}", attachmentHandler.GetSignature)
		r.Post("/{id}/ready-for-signing", documentHandler.ReadyForSigning)
		r.Post("/{id}/signing-sessions", signingHandler.CreateSession)
		r.Post("/{id}/cancel", documentHandler.Cancel)
		r.Post("/{id}/archive", documentHandler.Archive)
	})

	r.Route("/v1/shipper/documents", func(r chi.Router) {
		r.Get("/", shipperHandler.List)
		r.Get("/{documentId}", shipperHandler.Get)
		r.Get("/{documentId}/attachments", shipperHandler.ListAttachments)
		r.Get("/{documentId}/attachments/{attachmentId}", shipperHandler.GetAttachment)
		r.Get("/{documentId}/attachments/{attachmentId}/content", shipperHandler.Content)
	})

	r.Route("/v1/carrier/documents", func(r chi.Router) {
		r.Get("/", carrierHandler.List)
		r.Get("/{documentId}", carrierHandler.Get)
		r.Get("/{documentId}/attachments", carrierHandler.ListAttachments)
		r.Get("/{documentId}/attachments/{attachmentId}", carrierHandler.GetAttachment)
		r.Get("/{documentId}/attachments/{attachmentId}/content", carrierHandler.Content)
	})

	r.Route("/v1/signing-sessions", func(r chi.Router) {
		r.Get("/{id}", signingHandler.GetSession)
		r.Post("/{id}/signatures", signingHandler.AddSignature)
	})

	r.Route("/internal/v1/pod-uploads", func(r chi.Router) {
		r.Post("/", podHandler.CreateIntent)
		r.Put("/{uploadId}/content", podHandler.UploadContent)
		r.Post("/{uploadId}/complete", podHandler.Complete)
	})

	return r
}
