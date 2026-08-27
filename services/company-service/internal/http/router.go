package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/company-service/internal/config"
	"github.com/freight-platform/company-service/internal/http/authcontext"
	"github.com/freight-platform/company-service/internal/http/handlers"
	"github.com/freight-platform/company-service/internal/repository"
	"github.com/freight-platform/company-service/internal/service"
	"github.com/freight-platform/shared-go/internalauth"
	"github.com/freight-platform/shared-go/metrics"
	"github.com/freight-platform/shared-go/observability"
	sharedpprof "github.com/freight-platform/shared-go/pprof"
)

const serviceName = "company-service"

func NewRouter(
	log *slog.Logger,
	db observability.DatabasePinger,
	cfg config.Config,
	companyRepo *repository.CompanyRepository,
	membershipRepo *repository.MembershipRepository,
	companyService *service.CompanyService,
	membershipService *service.MembershipService,
) http.Handler {
	authorizer := service.NewCompanyAuthorizer(membershipRepo)
	companyHandler := handlers.NewCompanyHandler(companyService, authorizer)
	membershipHandler := handlers.NewMembershipHandler(membershipService, authorizer)
	companyInternalHandler := handlers.NewCompanyInternalHandler(companyRepo)
	internalAuth := internalauth.Config{Token: cfg.InternalServiceToken, Environment: cfg.Environment}

	r := chi.NewRouter()
	observability.Mount(r, observability.MountOptions{
		ServiceName: serviceName,
		Log:         log,
		Metrics:     metrics.New(serviceName),
		DB:          db,
	})
	sharedpprof.Mount(r)

	r.Route("/v1/companies", func(r chi.Router) {
		r.Use(authcontext.Middleware)
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

	r.Route("/internal/v1/companies", func(r chi.Router) {
		r.Use(internalAuth.Middleware)
		r.Post("/batch-get", companyInternalHandler.BatchGet)
	})

	return r
}
