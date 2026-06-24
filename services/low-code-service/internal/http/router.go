package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/freight-platform/low-code-service/internal/http/handlers"
	lcmiddleware "github.com/freight-platform/low-code-service/internal/http/middleware"
	"github.com/freight-platform/low-code-service/internal/platform/database"
	"github.com/freight-platform/low-code-service/internal/service"
	"github.com/freight-platform/shared-go/metrics"
	sharedmiddleware "github.com/freight-platform/shared-go/middleware"
)

const serviceName = "low-code-service"

func NewRouter(
	log *slog.Logger,
	readiness *database.ReadinessChecker,
	formTemplateSvc *service.FormTemplateService,
	customFieldValueSvc *service.CustomFieldValueService,
	auditSvc *service.AuditService,
	adminFormTemplateSvc *service.AdminFormTemplateService,
	adminAuth lcmiddleware.AdminAuthConfig,
) http.Handler {
	metricsCollector := metrics.New(serviceName)

	r := chi.NewRouter()
	r.Use(sharedmiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(sharedmiddleware.Recover(log, serviceName))
	r.Use(sharedmiddleware.AccessLog(log, serviceName))
	r.Use(metricsCollector.Middleware)

	r.Get("/health", handlers.Health())
	r.Get("/ready", handlers.Ready(readiness))
	r.Handle("/metrics", metricsCollector.Handler())

	formTemplateHandler := handlers.NewFormTemplateHandler(formTemplateSvc)
	customFieldValueHandler := handlers.NewCustomFieldValueHandler(customFieldValueSvc)
	adminCustomFieldValueHandler := handlers.NewAdminCustomFieldValueHandler(customFieldValueSvc)
	auditHandler := handlers.NewAuditHandler(auditSvc)
	r.Route("/v1/low-code", func(r chi.Router) {
		r.Get("/form-templates", formTemplateHandler.List)
		r.Get("/form-templates/active", formTemplateHandler.ListActive)
		r.Get("/form-templates/{id}", formTemplateHandler.GetByID)
		r.Get("/custom-field-values", customFieldValueHandler.Get)
		r.Put("/custom-field-values", customFieldValueHandler.Put)
		r.Get("/audit-events", auditHandler.List)

	adminGuard := lcmiddleware.RequireLowCodeAdmin(adminAuth)

		r.Route("/admin/custom-field-values", func(r chi.Router) {
			r.Use(adminGuard)
			r.Post("/migrate-to-active", adminCustomFieldValueHandler.MigrateToActive)
			r.Post("/migration-preview", adminCustomFieldValueHandler.MigrationPreview)
			r.Post("/batch-migration-preview", adminCustomFieldValueHandler.BatchMigrationPreview)
			r.Post("/batch-migrate-to-active", adminCustomFieldValueHandler.BatchMigrateToActive)
		})

		adminFormTemplateHandler := handlers.NewAdminFormTemplateHandler(adminFormTemplateSvc)
		r.Route("/admin/form-templates", func(r chi.Router) {
			r.Use(adminGuard)
			r.Post("/", adminFormTemplateHandler.Create)
			r.Post("/import-preview", adminFormTemplateHandler.ImportPreview)
			r.Post("/import", adminFormTemplateHandler.Import)
			r.Get("/", adminFormTemplateHandler.List)
			r.Get("/{id}", adminFormTemplateHandler.GetByID)
			r.Put("/{id}", adminFormTemplateHandler.Update)
			r.Post("/{id}/publish", adminFormTemplateHandler.Publish)
			r.Post("/{id}/clone-to-draft", adminFormTemplateHandler.CloneToDraft)
			r.Get("/{id}/export", adminFormTemplateHandler.Export)
		})
	})

	return r
}
