package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/payment-service/internal/http/handlers"
	"github.com/freight-platform/payment-service/internal/service"
	"github.com/freight-platform/shared-go/metrics"
	"github.com/freight-platform/shared-go/observability"
	sharedpprof "github.com/freight-platform/shared-go/pprof"
)

const serviceName = "payment-service"

func NewRouter(
	log *slog.Logger,
	db observability.DatabasePinger,
	paymentSvc *service.PaymentService,
	actor *handlers.PaymentActorResolver,
) http.Handler {
	paymentHandler := handlers.NewPaymentHandler(paymentSvc, actor)

	r := chi.NewRouter()
	observability.Mount(r, observability.MountOptions{
		ServiceName: serviceName,
		Log:         log,
		Metrics:     metrics.New(serviceName),
		DB:          db,
	})
	sharedpprof.Mount(r)

	r.Route("/v1/payment-obligations", func(r chi.Router) {
		r.Get("/", paymentHandler.ListObligations)
		r.Get("/{id}", paymentHandler.GetObligation)
		r.Patch("/{id}/due-date", paymentHandler.PatchDueDate)
	})

	r.Route("/v1/payments", func(r chi.Router) {
		r.Post("/", paymentHandler.CreatePayment)
		r.Get("/", paymentHandler.ListPayments)
		r.Get("/{id}", paymentHandler.GetPayment)
		r.Post("/{id}/allocations", paymentHandler.CreateAllocation)
		r.Post("/{id}/reconcile", paymentHandler.ReconcilePayment)
	})

	r.Route("/internal/v1/payment-obligations", func(r chi.Router) {
		r.Post("/ensure", paymentHandler.EnsureObligation)
	})

	return r
}
