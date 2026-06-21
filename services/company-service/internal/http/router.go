package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/company-service/internal/http/handlers"
	"github.com/freight-platform/company-service/internal/service"
	"github.com/freight-platform/shared-go/metrics"
	"github.com/freight-platform/shared-go/observability"
	sharedpprof "github.com/freight-platform/shared-go/pprof"
)

const serviceName = "company-service"

func NewRouter(
	log *slog.Logger,
	db observability.DatabasePinger,
	companyService *service.CompanyService,
	membershipService *service.MembershipService,
) http.Handler {
	companyHandler := handlers.NewCompanyHandler(companyService)
	membershipHandler := handlers.NewMembershipHandler(membershipService)

	r := chi.NewRouter()
	observability.Mount(r, observability.MountOptions{
		ServiceName: serviceName,
		Log:         log,
		Metrics:     metrics.New(serviceName),
		DB:          db,
	})
	sharedpprof.Mount(r)

	r.Route("/v1/companies", func(r chi.Router) {
		r.Post("/", companyHandler.Create)
		r.Get("/", companyHandler.List)
		r.Get("/{company_id}/members", membershipHandler.ListMembers)
		r.Post("/{company_id}/members", membershipHandler.AddMember)
		r.Patch("/{company_id}/members/{membership_id}", membershipHandler.UpdateMember)
		r.Delete("/{company_id}/members/{membership_id}", membershipHandler.RemoveMember)
		r.Get("/{id}", companyHandler.GetByID)
		r.Patch("/{id}", companyHandler.Update)
		r.Delete("/{id}", companyHandler.Delete)
	})

	return r
}
