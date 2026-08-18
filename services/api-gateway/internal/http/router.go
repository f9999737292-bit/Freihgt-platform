package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/freight-platform/api-gateway/internal/config"
	"github.com/freight-platform/api-gateway/internal/controltower"
	"github.com/freight-platform/api-gateway/internal/driver"
	"github.com/freight-platform/api-gateway/internal/executionrbac"
	"github.com/freight-platform/api-gateway/internal/fleetrbac"
	gwmiddleware "github.com/freight-platform/api-gateway/internal/http/middleware"
	apperrors "github.com/freight-platform/api-gateway/internal/platform/errors"
	"github.com/freight-platform/api-gateway/internal/platform/respond"
	"github.com/freight-platform/api-gateway/internal/rfxrbac"
	"github.com/freight-platform/api-gateway/internal/shipmentevents"
	"github.com/freight-platform/api-gateway/internal/tracking"
	"github.com/freight-platform/api-gateway/internal/shipmentrbac"
	"github.com/freight-platform/shared-go/metrics"
	sharedmiddleware "github.com/freight-platform/shared-go/middleware"
	"github.com/freight-platform/shared-go/observability"
	sharedpprof "github.com/freight-platform/shared-go/pprof"
)

const serviceName = "api-gateway"

func NewRouter(log *slog.Logger, cfg config.Config, proxy *ProxyHandler, controlTower *controltower.Handler, shipmentEvents *shipmentevents.Handler, trackingHandler *tracking.Handler, driverHandler *driver.Handler) http.Handler {
	metricsCollector := metrics.New(serviceName)

	r := chi.NewRouter()
	r.Use(sharedmiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(sharedmiddleware.Recover(log, serviceName))
	r.Use(sharedmiddleware.AccessLog(log, serviceName))
	r.Use(metricsCollector.Middleware)
	r.Use(gwmiddleware.MaxBodySize(cfg.MaxRequestBodyBytes))
	r.Use(gwmiddleware.RateLimit(cfg.RateLimitEnabled, cfg.RateLimitRPS, cfg.RateLimitBurst, serviceName))
	r.Use(gwmiddleware.CORS(cfg.CORSAllowedOrigins))
	r.Use(gwmiddleware.Auth(cfg.AuthEnabled, cfg.JWTSecret))

	sharedpprof.Mount(r)

	r.Get("/health", observability.HealthHandler(serviceName))
	r.Get("/ready", func(w http.ResponseWriter, req *http.Request) {
		status, httpStatus, services := ReadyStatus(req.Context(), cfg)
		respond.JSON(w, httpStatus, map[string]any{
			"status":   status,
			"services": services,
		})
	})
	r.Handle("/metrics", metricsCollector.Handler())

	r.Get("/routes", func(w http.ResponseWriter, _ *http.Request) {
		items := make([]map[string]string, 0, len(proxy.Routes()))
		for _, route := range proxy.Routes() {
			items = append(items, map[string]string{
				"prefix":  route.Prefix,
				"service": route.Service,
				"target":  RouteTarget(route),
			})
		}
		respond.JSON(w, http.StatusOK, map[string]any{"routes": items})
	})

	openAPI := NewOpenAPIHandler(cfg.OpenAPIDir)
	openAPI.RegisterRoutes(r)

	if controlTower != nil {
		r.Get("/api/v1/control-tower/summary", controlTower.Summary)
		r.Post("/api/v1/control-tower/critical-events/{eventId}/acknowledge", controlTower.AcknowledgeCriticalEvent)
		r.Post("/api/v1/control-tower/critical-events/{eventId}/assign", controlTower.AssignCriticalEvent)
		r.Post("/api/v1/control-tower/critical-events/{eventId}/resolve", controlTower.ResolveCriticalEvent)
		r.Post("/api/v1/control-tower/critical-events/{eventId}/reopen", controlTower.ReopenCriticalEvent)
		r.Patch("/api/v1/control-tower/critical-events/{eventId}/exception", controlTower.UpdateCriticalEventException)
		r.Get("/api/v1/control-tower/critical-events/{eventId}/actions", controlTower.GetCriticalEventActions)
		r.Get("/api/v1/control-tower/risks", controlTower.ListRisks)
		r.Get("/api/v1/control-tower/risks/{riskId}", controlTower.GetRisk)
		r.Post("/api/v1/control-tower/risks/{riskId}/acknowledge", controlTower.AcknowledgeRisk)
		r.Post("/api/v1/control-tower/risks/{riskId}/mitigate", controlTower.MitigateRisk)
		r.Get("/api/v1/control-tower/work-items", controlTower.ListWorkItems)
		r.Get("/api/v1/control-tower/work-items/{itemType}/{itemId}", controlTower.GetWorkItem)
		r.Post("/api/v1/control-tower/work-items/{itemType}/{itemId}/claim", controlTower.ClaimWorkItem)
		r.Post("/api/v1/control-tower/work-items/{itemType}/{itemId}/assign", controlTower.AssignWorkItem)
		r.Post("/api/v1/control-tower/work-items/{itemType}/{itemId}/unassign", controlTower.UnassignWorkItem)
		r.Post("/api/v1/control-tower/work-items/bulk-action", controlTower.BulkWorkItemsAction)
		r.Get("/api/v1/control-tower/workload", controlTower.GetWorkload)
		r.Get("/api/v1/control-tower/views", controlTower.ListSavedViews)
		r.Post("/api/v1/control-tower/views", controlTower.CreateSavedView)
		r.Patch("/api/v1/control-tower/views/{viewId}", controlTower.UpdateSavedView)
		r.Delete("/api/v1/control-tower/views/{viewId}", controlTower.DeleteSavedView)
		r.Post("/api/v1/control-tower/views/{viewId}/set-default", controlTower.SetDefaultSavedView)
		r.Get("/api/v1/control-tower/handoffs", controlTower.ListHandoffs)
		r.Post("/api/v1/control-tower/handoffs", controlTower.CreateHandoff)
		r.Get("/api/v1/control-tower/handoffs/{handoffId}", controlTower.GetHandoff)

		r.Get("/api/v1/control-tower/cases/kpi", controlTower.GetCaseKPI)
		r.Get("/api/v1/control-tower/cases/duplicates", controlTower.FindCaseDuplicates)
		r.Get("/api/v1/control-tower/cases", controlTower.ListCases)
		r.Post("/api/v1/control-tower/cases", controlTower.CreateCase)
		r.Get("/api/v1/control-tower/cases/{caseId}", controlTower.GetCase)
		r.Patch("/api/v1/control-tower/cases/{caseId}", controlTower.UpdateCase)
		r.Post("/api/v1/control-tower/cases/{caseId}/claim", controlTower.ClaimCase)
		r.Post("/api/v1/control-tower/cases/{caseId}/assign", controlTower.AssignCase)
		r.Post("/api/v1/control-tower/cases/{caseId}/unassign", controlTower.UnassignCase)
		r.Post("/api/v1/control-tower/cases/{caseId}/links", controlTower.AddCaseLink)
		r.Delete("/api/v1/control-tower/cases/{caseId}/links/{linkId}", controlTower.RemoveCaseLink)
		r.Post("/api/v1/control-tower/cases/{caseId}/notes", controlTower.CreateCaseNote)
		r.Patch("/api/v1/control-tower/cases/{caseId}/notes/{noteId}", controlTower.UpdateCaseNote)
		r.Post("/api/v1/control-tower/cases/{caseId}/actions", controlTower.CreateCaseAction)
		r.Patch("/api/v1/control-tower/cases/{caseId}/actions/{actionId}", controlTower.UpdateCaseAction)
		r.Post("/api/v1/control-tower/cases/{caseId}/actions/{actionId}/complete", controlTower.CompleteCaseAction)
		r.Post("/api/v1/control-tower/cases/{caseId}/decisions", controlTower.CreateCaseDecision)
		r.Post("/api/v1/control-tower/cases/{caseId}/resolve", controlTower.ResolveCase)
		r.Post("/api/v1/control-tower/cases/{caseId}/close", controlTower.CloseCase)
		r.Post("/api/v1/control-tower/cases/{caseId}/reopen", controlTower.ReopenCase)
		r.Get("/api/v1/control-tower/cases/{caseId}/timeline", controlTower.GetCaseTimeline)
		r.Post("/api/v1/control-tower/cases/{caseId}/participants", controlTower.AddCaseParticipant)
		r.Patch("/api/v1/control-tower/cases/{caseId}/participants/{userId}", controlTower.UpdateCaseParticipant)
		r.Delete("/api/v1/control-tower/cases/{caseId}/participants/{userId}", controlTower.RemoveCaseParticipant)

		r.Get("/api/v1/control-tower/automation/kpi", controlTower.GetAutomationKPI)
		r.Get("/api/v1/control-tower/automation/rules", controlTower.ListAutomationRules)
		r.Post("/api/v1/control-tower/automation/rules", controlTower.CreateAutomationRule)
		r.Get("/api/v1/control-tower/automation/rules/{ruleId}", controlTower.GetAutomationRule)
		r.Patch("/api/v1/control-tower/automation/rules/{ruleId}", controlTower.UpdateAutomationRule)
		r.Post("/api/v1/control-tower/automation/rules/{ruleId}/activate", controlTower.ActivateAutomationRule)
		r.Post("/api/v1/control-tower/automation/rules/{ruleId}/disable", controlTower.DisableAutomationRule)
		r.Post("/api/v1/control-tower/automation/rules/{ruleId}/retire", controlTower.RetireAutomationRule)
		r.Post("/api/v1/control-tower/automation/rules/{ruleId}/dry-run", controlTower.DryRunAutomationRule)
		r.Get("/api/v1/control-tower/automation/recommendations", controlTower.ListRecommendations)
		r.Get("/api/v1/control-tower/automation/recommendations/{recommendationId}", controlTower.GetRecommendation)
		r.Post("/api/v1/control-tower/automation/recommendations/{recommendationId}/accept", controlTower.AcceptRecommendation)
		r.Post("/api/v1/control-tower/automation/recommendations/{recommendationId}/dismiss", controlTower.DismissRecommendation)
		r.Get("/api/v1/control-tower/playbooks", controlTower.ListPlaybooks)
		r.Post("/api/v1/control-tower/playbooks", controlTower.CreatePlaybook)
		r.Get("/api/v1/control-tower/playbooks/{playbookId}", controlTower.GetPlaybook)
		r.Patch("/api/v1/control-tower/playbooks/{playbookId}", controlTower.UpdatePlaybook)
		r.Get("/api/v1/control-tower/playbook-executions", controlTower.ListPlaybookExecutions)
		r.Get("/api/v1/control-tower/playbook-executions/{executionId}", controlTower.GetPlaybookExecution)
		r.Post("/api/v1/control-tower/playbook-executions/{executionId}/start", controlTower.StartPlaybookExecution)
		r.Post("/api/v1/control-tower/playbook-executions/{executionId}/complete", controlTower.CompletePlaybookExecution)
		r.Post("/api/v1/control-tower/playbook-executions/{executionId}/cancel", controlTower.CancelPlaybookExecution)
		r.Post("/api/v1/control-tower/playbook-executions/{executionId}/steps/{stepId}/start", controlTower.StartPlaybookExecutionStep)
		r.Post("/api/v1/control-tower/playbook-executions/{executionId}/steps/{stepId}/complete", controlTower.CompletePlaybookExecutionStep)
		r.Post("/api/v1/control-tower/playbook-executions/{executionId}/steps/{stepId}/skip", controlTower.SkipPlaybookExecutionStep)
	}

	if shipmentEvents != nil {
		r.Get("/api/v1/shipments/{shipmentId}/events", shipmentEvents.Events)
	}

	if trackingHandler != nil {
		r.Get("/api/v1/shipments/{shipmentId}/tracking", trackingHandler.GetCurrent)
		r.Get("/api/v1/shipments/{shipmentId}/tracking/locations", trackingHandler.ListLocations)
		r.Get("/api/v1/shipments/{shipmentId}/eta", trackingHandler.GetETA)
		r.Get("/api/v1/shipments/{shipmentId}/eta/history", trackingHandler.ListETAHistory)
		r.Get("/api/v1/shipments/{shipmentId}/slots", trackingHandler.GetSlots)
		r.Get("/api/v1/shipments/{shipmentId}/slots/history", trackingHandler.ListSlotHistory)
	}

	if driverHandler != nil {
		r.Get("/api/v1/driver/me", driverHandler.GetMe)
		r.Get("/api/v1/driver/me/shipments", driverHandler.ListShipments)
		r.Get("/api/v1/driver/me/shipments/{shipmentId}", driverHandler.GetShipment)
		r.Post("/api/v1/driver/me/shipments/{shipmentId}/events", driverHandler.RecordEvent)
		r.Post("/api/v1/driver/me/shipments/{shipmentId}/exceptions", driverHandler.ReportException)
		r.Post("/api/v1/driver/me/shipments/{shipmentId}/delays", driverHandler.ReportDelay)
		r.Post("/api/v1/driver/me/shipments/{shipmentId}/locations", driverHandler.IngestLocation)
		r.Post("/api/v1/driver/me/shipments/{shipmentId}/pod/uploads", driverHandler.InitiatePODUpload)
		r.Put("/api/v1/driver/me/shipments/{shipmentId}/pod/uploads/{uploadId}/content", driverHandler.UploadPODContent)
		r.Post("/api/v1/driver/me/shipments/{shipmentId}/pod/uploads/{uploadId}/complete", driverHandler.CompletePODUpload)
		r.Get("/api/v1/driver/me/tasks", driverHandler.ListTasks)
		r.Get("/api/v1/driver/me/tasks/{taskId}", driverHandler.GetTask)
		r.Post("/api/v1/driver/me/tasks/{taskId}/read", driverHandler.MarkTaskRead)
		r.Post("/api/v1/driver/me/tasks/{taskId}/acknowledge", driverHandler.AcknowledgeTask)
		r.Post("/api/v1/driver/me/tasks/{taskId}/responses", driverHandler.SubmitTaskResponse)
		r.Post("/api/v1/driver/me/devices", driverHandler.RegisterDevice)
		r.Delete("/api/v1/driver/me/devices/{deviceId}", driverHandler.RevokeDevice)
	}

	fleetGuard := fleetrbac.NewGuard(cfg, proxy)
	r.Get("/api/v1/drivers", fleetGuard.WithPolicy(fleetrbac.PolicyView))
	r.Get("/api/v1/drivers/{id}", fleetGuard.WithPolicy(fleetrbac.PolicyView))
	r.Post("/api/v1/drivers", fleetGuard.WithPolicy(fleetrbac.PolicyCreate))
	r.Get("/api/v1/vehicles", fleetGuard.WithPolicy(fleetrbac.PolicyView))
	r.Get("/api/v1/vehicles/{id}", fleetGuard.WithPolicy(fleetrbac.PolicyView))
	r.Post("/api/v1/vehicles", fleetGuard.WithPolicy(fleetrbac.PolicyCreate))
	r.Post("/api/v1/shipments/{id}/assign-driver", fleetGuard.WithPolicy(fleetrbac.PolicyAssign))
	r.Post("/api/v1/shipments/{id}/assign-vehicle", fleetGuard.WithPolicy(fleetrbac.PolicyAssign))

	shipmentGuard := shipmentrbac.NewGuard(cfg, proxy)
	r.Post("/api/v1/shipments/from-transport-order", shipmentGuard.WithPolicy(shipmentrbac.PolicyCreate))
	r.Post("/api/v1/shipments/from-bid", shipmentGuard.WithPolicy(shipmentrbac.PolicyCreate))
	r.Post("/api/v1/shipments/{id}/accept", shipmentGuard.WithPolicy(shipmentrbac.PolicyAccept))
	r.Patch("/api/v1/shipments/{id}/status", shipmentGuard.WithPolicy(shipmentrbac.PolicyUpdateStatus))
	r.Post("/api/v1/shipments/{id}/cancel", shipmentGuard.WithPolicy(shipmentrbac.PolicyCancel))

	executionGuard := executionrbac.NewGuard(cfg, proxy)
	r.Post("/api/v1/order-execution/transport-orders/{id}/execute", executionGuard.WithPolicy(executionrbac.PolicyExecute))
	r.Get("/api/v1/order-execution/transport-orders/{id}", executionGuard.WithPolicy(executionrbac.PolicyRead))
	r.Post("/api/v1/order-execution/transport-orders/{id}/start", executionGuard.WithPolicy(executionrbac.PolicyStart))
	r.Get("/api/v1/order-execution/carrier/transport-orders", executionGuard.WithPolicy(executionrbac.PolicyRead))
	r.Get("/api/v1/carrier/transport-orders", executionGuard.WithPolicy(executionrbac.PolicyRead))

	rfxGuard := rfxrbac.NewGuard(cfg, proxy)
	r.Post("/api/v1/rfx-events", rfxGuard.WithPolicy(rfxrbac.PolicyBuyerManage))
	r.Get("/api/v1/rfx-events", rfxGuard.WithPolicy(rfxrbac.PolicyBuyerRead))
	r.Get("/api/v1/rfx-events/{id}", rfxGuard.WithPolicy(rfxrbac.PolicyBuyerRead))
	r.Patch("/api/v1/rfx-events/{id}", rfxGuard.WithPolicy(rfxrbac.PolicyBuyerManage))
	r.Post("/api/v1/rfx-events/{id}/publish", rfxGuard.WithPolicy(rfxrbac.PolicyBuyerManage))
	r.Post("/api/v1/rfx-events/{id}/cancel", rfxGuard.WithPolicy(rfxrbac.PolicyBuyerManage))
	r.Post("/api/v1/rfx-events/{id}/transitions/{command}", rfxGuard.WithPolicy(rfxrbac.PolicyBuyerManage))
	r.Post("/api/v1/rfx-events/{id}/send-invitations", rfxGuard.WithPolicy(rfxrbac.PolicyBuyerManage))
	r.Post("/api/v1/rfx-events/{id}/open-questions", rfxGuard.WithPolicy(rfxrbac.PolicyBuyerManage))
	r.Post("/api/v1/rfx-events/{id}/open-responses", rfxGuard.WithPolicy(rfxrbac.PolicyBuyerManage))
	r.Post("/api/v1/rfx-events/{id}/close-responses", rfxGuard.WithPolicy(rfxrbac.PolicyBuyerManage))
	r.Post("/api/v1/rfx-events/{id}/start-evaluation", rfxGuard.WithPolicy(rfxrbac.PolicyBuyerManage))
	r.Post("/api/v1/rfx-events/{id}/shortlist", rfxGuard.WithPolicy(rfxrbac.PolicyBuyerManage))
	r.Post("/api/v1/rfx-events/{id}/award", rfxGuard.WithPolicy(rfxrbac.PolicyBuyerManage))
	r.Post("/api/v1/rfx-events/{id}/archive", rfxGuard.WithPolicy(rfxrbac.PolicyBuyerManage))
	r.Post("/api/v1/rfx-events/{id}/extend-deadline", rfxGuard.WithPolicy(rfxrbac.PolicyBuyerManage))
	r.Post("/api/v1/rfx-events/{id}/lots", rfxGuard.WithPolicy(rfxrbac.PolicyBuyerManage))
	r.Get("/api/v1/rfx-events/{id}/lots", rfxGuard.WithPolicy(rfxrbac.PolicyBuyerRead))
	r.Post("/api/v1/rfx-lots/{lot_id}/lanes", rfxGuard.WithPolicy(rfxrbac.PolicyBuyerManage))
	r.Post("/api/v1/rfx-events/{id}/participants", rfxGuard.WithPolicy(rfxrbac.PolicyBuyerManage))
	r.Get("/api/v1/rfx-events/{id}/participants", rfxGuard.WithPolicy(rfxrbac.PolicyBuyerRead))
	r.Post("/api/v1/rfx-events/{id}/responses", rfxGuard.WithPolicy(rfxrbac.PolicyCarrierRespond))
	r.Get("/api/v1/rfx-events/{id}/responses", rfxGuard.WithPolicy(rfxrbac.PolicyBuyerRead))
	r.Post("/api/v1/rfx-events/{id}/evaluation/recalculate", rfxGuard.WithPolicy(rfxrbac.PolicyBuyerManage))
	r.Post("/api/v1/rfx-events/{id}/award-response", rfxGuard.WithPolicy(rfxrbac.PolicyBuyerManage))
	r.Get("/api/v1/rfx-events/{id}/audit-events", rfxGuard.WithPolicy(rfxrbac.PolicyBuyerRead))
	r.Get("/api/v1/rfx-events/{id}/own-award", rfxGuard.WithPolicy(rfxrbac.PolicyCarrierRead))
	r.Post("/api/v1/rfx-events/{id}/transport-orders", rfxGuard.WithPolicy(rfxrbac.PolicyBuyerManage))
	r.Get("/api/v1/rfx-events/{id}/transport-orders", rfxGuard.WithPolicy(rfxrbac.PolicyBuyerRead))
	r.Get("/api/v1/rfx-events/{id}/own-response", rfxGuard.WithPolicy(rfxrbac.PolicyCarrierRead))
	r.Get("/api/v1/rfx-responses/{response_id}", rfxGuard.WithPolicy(rfxrbac.PolicyCombinedRead))
	r.Patch("/api/v1/rfx-responses/{response_id}", rfxGuard.WithPolicy(rfxrbac.PolicyCarrierRespond))
	r.Patch("/api/v1/rfx-responses/{response_id}/evaluation", rfxGuard.WithPolicy(rfxrbac.PolicyBuyerManage))
	r.Post("/api/v1/rfx-responses/{response_id}/shortlist", rfxGuard.WithPolicy(rfxrbac.PolicyBuyerManage))
	r.Delete("/api/v1/rfx-responses/{response_id}/shortlist", rfxGuard.WithPolicy(rfxrbac.PolicyBuyerManage))
	r.Post("/api/v1/rfx-responses/{response_id}/submit", rfxGuard.WithPolicy(rfxrbac.PolicyCarrierRespond))
	r.Post("/api/v1/freight-requests/from-transport-order", rfxGuard.WithPolicy(rfxrbac.PolicyBuyerManage))
	r.Get("/api/v1/freight-requests", rfxGuard.WithPolicy(rfxrbac.PolicyCombinedRead))
	r.Get("/api/v1/freight-requests/{id}", rfxGuard.WithPolicy(rfxrbac.PolicyCombinedRead))
	r.Post("/api/v1/freight-requests/{id}/publish", rfxGuard.WithPolicy(rfxrbac.PolicyBuyerManage))
	r.Post("/api/v1/freight-requests/{id}/bids", rfxGuard.WithPolicy(rfxrbac.PolicyCarrierRespond))
	r.Get("/api/v1/freight-requests/{id}/bids", rfxGuard.WithPolicy(rfxrbac.PolicyCombinedRead))
	r.Get("/api/v1/bids/{id}", rfxGuard.WithPolicy(rfxrbac.PolicyCombinedRead))
	r.Post("/api/v1/bids/{id}/submit", rfxGuard.WithPolicy(rfxrbac.PolicyCarrierRespond))
	r.Post("/api/v1/bids/{id}/accept", rfxGuard.WithPolicy(rfxrbac.PolicyAcceptBid))

	r.Handle("/api/*", proxy)
	r.Handle("/api", proxy)

	r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		respond.Error(w, apperrors.RouteNotFound("no route found for path"))
	})

	return r
}
