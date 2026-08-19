package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/billing-register-service/internal/http/handlers"
	"github.com/freight-platform/billing-register-service/internal/service"
	"github.com/freight-platform/shared-go/metrics"
	"github.com/freight-platform/shared-go/observability"
	sharedpprof "github.com/freight-platform/shared-go/pprof"
)

const serviceName = "billing-register-service"

func NewRouter(
	log *slog.Logger,
	db observability.DatabasePinger,
	registerSvc *service.BillingRegisterService,
	closingSvc *service.ClosingDocumentService,
	settlementSvc *service.FreightSettlementService,
) http.Handler {
	registerHandler := handlers.NewBillingRegisterHandler(registerSvc)
	closingHandler := handlers.NewClosingDocumentHandler(closingSvc)
	settlementHandler := handlers.NewFreightSettlementHandler(settlementSvc)

	r := chi.NewRouter()
	observability.Mount(r, observability.MountOptions{
		ServiceName: serviceName,
		Log:         log,
		Metrics:     metrics.New(serviceName),
		DB:          db,
	})
	sharedpprof.Mount(r)

	r.Route("/v1/billing-registers", func(r chi.Router) {
		r.Post("/", registerHandler.Create)
		r.Get("/", registerHandler.List)
		r.Get("/{id}", registerHandler.GetByID)
		r.Post("/{id}/items", registerHandler.AddItem)
		r.Get("/{id}/items", registerHandler.ListItems)
		r.Post("/{id}/settlements", registerHandler.IncludeSettlement)
		r.Delete("/{register_id}/settlements/{settlementId}", registerHandler.RemoveSettlement)
		r.Delete("/{register_id}/items/{item_id}", registerHandler.DeleteItem)
		r.Post("/{id}/calculate", registerHandler.Calculate)
		r.Post("/{id}/approve", registerHandler.Approve)
		r.Post("/{id}/closing-document-package", closingHandler.CreatePackage)
		r.Post("/{id}/invoices", closingHandler.CreateInvoice)
		r.Post("/{id}/acts", closingHandler.CreateAct)
		r.Post("/{id}/vat-invoices", closingHandler.CreateVATInvoice)
		r.Post("/{id}/upd", closingHandler.CreateUPD)
		r.Post("/{id}/mark-sent-to-edo", registerHandler.MarkSentToEDO)
		r.Post("/{id}/mark-signed", registerHandler.MarkSigned)
		r.Post("/{id}/mark-paid", registerHandler.MarkPaid)
		r.Post("/{id}/close", registerHandler.Close)
	})

	r.Route("/v1/freight-settlements", func(r chi.Router) {
		r.Post("/", settlementHandler.Create)
		r.Get("/", settlementHandler.List)
		r.Get("/{id}", settlementHandler.GetByID)
		r.Post("/{id}/accessorials", settlementHandler.ProposeAccessorial)
		r.Post("/{id}/accessorials/{accessorialId}/approve", settlementHandler.ApproveAccessorial)
		r.Post("/{id}/accessorials/{accessorialId}/reject", settlementHandler.RejectAccessorial)
		r.Post("/{id}/disputes", settlementHandler.RaiseDispute)
		r.Post("/{id}/disputes/{disputeId}/resolve", settlementHandler.ResolveDispute)
		r.Post("/{id}/submit-for-review", settlementHandler.SubmitForReview)
		r.Post("/{id}/approve", settlementHandler.Approve)
		r.Post("/{id}/mark-documents-ready", settlementHandler.MarkDocumentsReady)
		r.Post("/{id}/mark-ready-for-payment", settlementHandler.MarkReadyForPayment)
		r.Post("/{id}/include-in-register", settlementHandler.IncludeInRegister)
	})

	return r
}
