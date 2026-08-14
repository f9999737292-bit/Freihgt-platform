package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/control-tower-read-model-service/internal/consumer"
	"github.com/freight-platform/control-tower-read-model-service/internal/http/handlers"
	"github.com/freight-platform/control-tower-read-model-service/internal/repository"
	"github.com/freight-platform/shared-go/metrics"
	"github.com/freight-platform/shared-go/observability"
	sharedpprof "github.com/freight-platform/shared-go/pprof"
)

const serviceName = "control-tower-read-model-service"

func NewRouter(
	log *slog.Logger,
	db observability.DatabasePinger,
	repo *repository.ProjectionRepository,
	ackRepo *repository.AckRepository,
	workflowRepo *repository.WorkflowRepository,
	riskRepo *repository.RiskRepository,
	workItemRepo *repository.WorkItemRepository,
	viewRepo *repository.ViewRepository,
	handoffRepo *repository.HandoffRepository,
	freshness *consumer.Freshness,
) http.Handler {
	statusHandler := handlers.NewStatusHandler(repo, freshness)
	ackHandler := handlers.NewAckHandler(ackRepo, workflowRepo)
	riskHandler := handlers.NewRiskHandler(riskRepo)
	workspaceHandler := handlers.NewWorkspaceHandler(workItemRepo, viewRepo, handoffRepo)

	r := chi.NewRouter()
	observability.Mount(r, observability.MountOptions{
		ServiceName: serviceName,
		Log:         log,
		Metrics:     metrics.New(serviceName),
		DB:          db,
	})
	sharedpprof.Mount(r)

	r.Route("/internal/v1/control-tower", func(r chi.Router) {
		r.Get("/shipments/{shipmentId}/status", statusHandler.GetShipmentStatus)
		r.Get("/status-summary", statusHandler.GetStatusSummary)
		r.Get("/shipments/statuses", statusHandler.ListShipmentStatuses)
		r.Post("/critical-events/{eventId}/acknowledge", ackHandler.AcknowledgeCriticalEvent)
		r.Post("/critical-events/{eventId}/assign", ackHandler.AssignCriticalEvent)
		r.Post("/critical-events/{eventId}/resolve", ackHandler.ResolveCriticalEvent)
		r.Post("/critical-events/{eventId}/reopen", ackHandler.ReopenCriticalEvent)
		r.Get("/critical-events/{eventId}/actions", ackHandler.ListCriticalEventActions)
		r.Post("/critical-events/acknowledgements/lookup", ackHandler.LookupAcknowledgements)
		r.Post("/critical-events/workflows/ensure", ackHandler.EnsureExceptionWorkflows)
		r.Post("/critical-events/workflows/lookup", ackHandler.LookupWorkflows)
		r.Patch("/critical-events/{eventId}/exception", ackHandler.UpdateException)
		r.Post("/risks/sync", riskHandler.SyncRisks)
		r.Get("/risks/kpi", riskHandler.GetRiskKPI)
		r.Get("/risks", riskHandler.ListRisks)
		r.Get("/risks/{riskKey}", riskHandler.GetRisk)
		r.Post("/risks/{riskKey}/acknowledge", riskHandler.AcknowledgeRisk)
		r.Post("/risks/{riskKey}/mitigate", riskHandler.MitigateRisk)

		r.Get("/work-items", workspaceHandler.ListWorkItems)
		r.Get("/work-items/{itemType}/{itemId}", workspaceHandler.GetWorkItem)
		r.Post("/work-items/{itemType}/{itemId}/claim", workspaceHandler.ClaimWorkItem)
		r.Post("/work-items/{itemType}/{itemId}/assign", workspaceHandler.AssignWorkItem)
		r.Post("/work-items/{itemType}/{itemId}/unassign", workspaceHandler.UnassignWorkItem)
		r.Post("/work-items/bulk-action", workspaceHandler.BulkAction)
		r.Get("/workload", workspaceHandler.GetWorkload)
		r.Get("/views", workspaceHandler.ListViews)
		r.Post("/views", workspaceHandler.CreateView)
		r.Patch("/views/{viewId}", workspaceHandler.UpdateView)
		r.Delete("/views/{viewId}", workspaceHandler.DeleteView)
		r.Post("/views/{viewId}/set-default", workspaceHandler.SetDefaultView)
		r.Get("/handoffs", workspaceHandler.ListHandoffs)
		r.Post("/handoffs", workspaceHandler.CreateHandoff)
		r.Get("/handoffs/{handoffId}", workspaceHandler.GetHandoff)
	})

	return r
}
