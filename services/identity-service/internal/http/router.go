package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	authmiddleware "github.com/freight-platform/identity-service/internal/http/middleware"
	"github.com/freight-platform/identity-service/internal/http/handlers"
	"github.com/freight-platform/identity-service/internal/platform/security"
	"github.com/freight-platform/identity-service/internal/service"
	"github.com/freight-platform/shared-go/metrics"
	"github.com/freight-platform/shared-go/observability"
)

const serviceName = "identity-service"

func NewRouter(
	log *slog.Logger,
	db observability.DatabasePinger,
	jwtService *security.JWTService,
	authService *service.AuthService,
	userService *service.UserService,
	roleService *service.RoleService,
	membershipService *service.MembershipService,
) http.Handler {
	authHandler := handlers.NewAuthHandler(authService, roleService)
	userHandler := handlers.NewUserHandler(userService, roleService)
	roleHandler := handlers.NewRoleHandler(roleService)
	membershipHandler := handlers.NewMembershipHandler(membershipService)

	r := chi.NewRouter()
	observability.Mount(r, observability.MountOptions{
		ServiceName: serviceName,
		Log:         log,
		Metrics:     metrics.New(serviceName),
		DB:          db,
	})

	r.Route("/v1/auth", func(r chi.Router) {
		r.Post("/login", authHandler.Login)
		r.With(authmiddleware.Auth(jwtService)).Get("/me", authHandler.Me)
	})

	r.Route("/v1/users", func(r chi.Router) {
		r.Post("/", userHandler.Create)
		r.Get("/", userHandler.List)
		r.Get("/{user_id}/companies", membershipHandler.ListUserCompanies)
		r.Post("/{user_id}/companies/{company_id}/roles", membershipHandler.AssignCompanyRole)
		r.Delete("/{user_id}/companies/{company_id}/roles/{role_id}", membershipHandler.RemoveCompanyRole)
		r.Post("/{id}/roles", userHandler.AssignRole)
		r.Get("/{id}/roles", userHandler.ListRoles)
		r.Get("/{id}", userHandler.GetByID)
		r.Patch("/{id}", userHandler.Update)
		r.Delete("/{id}", userHandler.Delete)
	})

	r.Route("/v1/roles", func(r chi.Router) {
		r.Get("/", roleHandler.List)
		r.Get("/{role_id}/permissions", roleHandler.ListPermissions)
	})

	return r
}
