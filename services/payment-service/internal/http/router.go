package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/payment-service/internal/config"
	"github.com/freight-platform/payment-service/internal/http/handlers"
	"github.com/freight-platform/payment-service/internal/service"
	"github.com/freight-platform/shared-go/internalauth"
	"github.com/freight-platform/shared-go/metrics"
	"github.com/freight-platform/shared-go/observability"
	sharedpprof "github.com/freight-platform/shared-go/pprof"
)

const serviceName = "payment-service"

func NewRouter(
	log *slog.Logger,
	db observability.DatabasePinger,
	cfg config.Config,
	paymentSvc *service.PaymentService,
	actor *handlers.PaymentActorResolver,
) http.Handler {
	paymentHandler := handlers.NewPaymentHandler(paymentSvc, actor)
	internalAuth := internalauth.Config{Token: cfg.InternalServiceToken, Environment: cfg.Environment}

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

	r.Route("/v1/payment-allocations", func(r chi.Router) {
		r.Post("/{id}/void", paymentHandler.VoidAllocation)
	})

	r.Route("/v1/payments", func(r chi.Router) {
		r.Post("/", paymentHandler.CreatePayment)
		r.Get("/", paymentHandler.ListPayments)
		r.Get("/{id}", paymentHandler.GetPayment)
		r.Post("/{id}/allocations", paymentHandler.CreateAllocation)
		r.Post("/{id}/reconcile", paymentHandler.ReconcilePayment)
		r.Post("/{id}/void", paymentHandler.VoidPayment)
	})

	r.Route("/internal/v1", func(r chi.Router) {
		r.Use(internalAuth.Middleware)
		r.Route("/payment-obligations", func(r chi.Router) {
			r.Post("/ensure", paymentHandler.EnsureObligation)
		})
		r.Route("/billing-registers", func(r chi.Router) {
			r.Post("/{id}/ensure-paid-projection", paymentHandler.EnsurePaidProjection)
		})
	})

	return r
}
