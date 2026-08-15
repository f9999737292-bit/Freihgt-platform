package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/control-tower-read-model-service/internal/consumer"
	"github.com/freight-platform/control-tower-read-model-service/internal/http/handlers"
	"github.com/freight-platform/control-tower-read-model-service/internal/repository"
	"github.com/freight-platform/control-tower-read-model-service/internal/service"
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
	caseRepo *repository.CaseRepository,
	automationRepo *repository.AutomationRepository,
	automationSvc *service.AutomationService,
	automationIngress *service.AutomationTriggerIngress,
	freshness *consumer.Freshness,
) http.Handler {
	statusHandler := handlers.NewStatusHandler(repo, freshness)
	ackHandler := handlers.NewAckHandler(ackRepo, workflowRepo, automationIngress)
	riskHandler := handlers.NewRiskHandler(riskRepo)
	workspaceHandler := handlers.NewWorkspaceHandler(workItemRepo, viewRepo, handoffRepo, caseRepo)
	caseHandler := handlers.NewCaseHandler(caseRepo, automationIngress)
	automationHandler := handlers.NewAutomationHandler(automationRepo, automationSvc, automationIngress)

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

		r.Get("/cases/kpi", caseHandler.GetKPI)
		r.Get("/cases/duplicates", caseHandler.FindDuplicates)
		r.Get("/cases", caseHandler.ListCases)
		r.Post("/cases", caseHandler.CreateCase)
		r.Get("/cases/{caseId}", caseHandler.GetCase)
		r.Patch("/cases/{caseId}", caseHandler.UpdateCase)
		r.Post("/cases/{caseId}/claim", caseHandler.ClaimCase)
		r.Post("/cases/{caseId}/assign", caseHandler.AssignCase)
		r.Post("/cases/{caseId}/unassign", caseHandler.UnassignCase)
		r.Post("/cases/{caseId}/links", caseHandler.AddLink)
		r.Delete("/cases/{caseId}/links/{linkId}", caseHandler.RemoveLink)
		r.Post("/cases/{caseId}/notes", caseHandler.CreateNote)
		r.Patch("/cases/{caseId}/notes/{noteId}", caseHandler.UpdateNote)
		r.Post("/cases/{caseId}/actions", caseHandler.CreateActionItem)
		r.Patch("/cases/{caseId}/actions/{actionId}", caseHandler.UpdateActionItem)
		r.Post("/cases/{caseId}/actions/{actionId}/complete", caseHandler.CompleteActionItem)
		r.Post("/cases/{caseId}/decisions", caseHandler.CreateDecision)
		r.Post("/cases/{caseId}/resolve", caseHandler.ResolveCase)
		r.Post("/cases/{caseId}/close", caseHandler.CloseCase)
		r.Post("/cases/{caseId}/reopen", caseHandler.ReopenCase)
		r.Get("/cases/{caseId}/timeline", caseHandler.GetTimeline)
		r.Post("/cases/{caseId}/participants", caseHandler.AddParticipant)
		r.Patch("/cases/{caseId}/participants/{userId}", caseHandler.UpdateParticipant)
		r.Delete("/cases/{caseId}/participants/{userId}", caseHandler.RemoveParticipant)

		r.Get("/automation/kpi", automationHandler.GetAutomationKPI)
		r.Get("/automation/rules", automationHandler.ListRules)
		r.Post("/automation/rules", automationHandler.CreateRule)
		r.Get("/automation/rules/{ruleId}", automationHandler.GetRule)
		r.Patch("/automation/rules/{ruleId}", automationHandler.UpdateRule)
		r.Post("/automation/rules/{ruleId}/activate", automationHandler.ActivateRule)
		r.Post("/automation/rules/{ruleId}/disable", automationHandler.DisableRule)
		r.Post("/automation/rules/{ruleId}/retire", automationHandler.RetireRule)
		r.Post("/automation/rules/{ruleId}/dry-run", automationHandler.DryRunRule)
		r.Post("/automation/evaluate", automationHandler.Evaluate)

		r.Get("/playbooks", automationHandler.ListPlaybooks)
		r.Post("/playbooks", automationHandler.CreatePlaybook)
		r.Get("/playbooks/{playbookId}", automationHandler.GetPlaybook)
		r.Patch("/playbooks/{playbookId}", automationHandler.UpdatePlaybook)

		r.Get("/automation/recommendations", automationHandler.ListRecommendations)
		r.Get("/automation/recommendations/{recommendationId}", automationHandler.GetRecommendation)
		r.Post("/automation/recommendations/{recommendationId}/accept", automationHandler.AcceptRecommendation)
		r.Post("/automation/recommendations/{recommendationId}/dismiss", automationHandler.DismissRecommendation)

		r.Get("/playbook-executions", automationHandler.ListExecutions)
		r.Get("/playbook-executions/{executionId}", automationHandler.GetExecution)
		r.Post("/playbook-executions/{executionId}/start", automationHandler.StartExecution)
		r.Post("/playbook-executions/{executionId}/complete", automationHandler.CompleteExecution)
		r.Post("/playbook-executions/{executionId}/cancel", automationHandler.CancelExecution)
		r.Post("/playbook-executions/{executionId}/steps/{stepId}/start", automationHandler.StartExecutionStep)
		r.Post("/playbook-executions/{executionId}/steps/{stepId}/complete", automationHandler.CompleteExecutionStep)
		r.Post("/playbook-executions/{executionId}/steps/{stepId}/skip", automationHandler.SkipExecutionStep)
	})

	return r
}
