package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	apperrors "github.com/freight-platform/control-tower-read-model-service/internal/platform/errors"
	"github.com/freight-platform/control-tower-read-model-service/internal/platform/respond"
	"github.com/freight-platform/control-tower-read-model-service/internal/service"
)

type ensureExceptionRequest struct {
	Events []ensureExceptionSeedRequest `json:"events"`
}

type ensureExceptionSeedRequest struct {
	EventID    string `json:"eventId"`
	ShipmentID string `json:"shipmentId"`
	EventType  string `json:"eventType"`
	Source     string `json:"source"`
	OccurredAt string `json:"occurredAt"`
	Severity   string `json:"severity"`
}

type updateExceptionRequest struct {
	Priority       *string `json:"priority"`
	Category       *string `json:"category"`
	BusinessImpact *string `json:"businessImpact"`
}

type exceptionSLAResponse struct {
	Phase            string `json:"phase"`
	Status           string `json:"status"`
	AcknowledgeDueAt string `json:"acknowledgeDueAt"`
	AssignmentDueAt  string `json:"assignmentDueAt"`
	ResolutionDueAt  string `json:"resolutionDueAt"`
	RemainingSeconds *int64 `json:"remainingSeconds,omitempty"`
}

type exceptionEscalationResponse struct {
	Level string `json:"level"`
}

type exceptionDetailsResponse struct {
	Priority          string                      `json:"priority"`
	ExceptionCategory string                      `json:"exceptionCategory"`
	BusinessImpact    string                      `json:"businessImpact"`
	SLA               exceptionSLAResponse        `json:"sla"`
	Escalation        exceptionEscalationResponse `json:"escalation"`
}

func toExceptionDetailsResponse(workflow domain.CriticalEventWorkflow, now time.Time) exceptionDetailsResponse {
	slaEval := domain.EvaluateSLA(workflow, now)
	return exceptionDetailsResponse{
		Priority:          workflow.Priority,
		ExceptionCategory: workflow.ExceptionCategory,
		BusinessImpact:    workflow.BusinessImpact,
		SLA: exceptionSLAResponse{
			Phase:            slaEval.Phase,
			Status:           slaEval.Status,
			AcknowledgeDueAt: workflow.AcknowledgeDueAt.UTC().Format(time.RFC3339),
			AssignmentDueAt:  workflow.AssignmentDueAt.UTC().Format(time.RFC3339),
			ResolutionDueAt:  workflow.ResolutionDueAt.UTC().Format(time.RFC3339),
			RemainingSeconds: slaEval.RemainingSeconds,
		},
		Escalation: exceptionEscalationResponse{Level: workflow.EscalationLevel},
	}
}

func (h *AckHandler) EnsureExceptionWorkflows(w http.ResponseWriter, r *http.Request) {
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}

	var payload ensureExceptionRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", map[string]any{"field": "body"}))
		return
	}

	seeds := make([]domain.EnsureExceptionSeed, 0, len(payload.Events))
	for _, item := range payload.Events {
		eventID := strings.ToLower(strings.TrimSpace(item.EventID))
		if !eventIDPattern.MatchString(eventID) {
			continue
		}
		occurredAt, err := time.Parse(time.RFC3339, strings.TrimSpace(item.OccurredAt))
		if err != nil {
			continue
		}
		seeds = append(seeds, domain.EnsureExceptionSeed{
			EventID:    eventID,
			ShipmentID: strings.TrimSpace(item.ShipmentID),
			EventType:  strings.TrimSpace(item.EventType),
			Source:     strings.TrimSpace(item.Source),
			OccurredAt: occurredAt,
			Severity:   strings.TrimSpace(item.Severity),
		})
	}

	created, err := h.workflowRepo.EnsureExceptionWorkflows(r.Context(), tenantID, seeds)
	if err != nil {
		respond.Error(w, apperrors.Internal("failed to ensure exception workflows", err))
		return
	}

	if h.automation != nil {
		for _, id := range created {
			for _, seed := range seeds {
				if seed.EventID == id {
					trigger := service.BuildExceptionCreatedTrigger(tenantID, seed)
					_, _ = h.automation.HandleTrigger(r.Context(), tenantID, trigger, true)
				}
			}
		}
	}

	respond.JSON(w, http.StatusOK, map[string]any{
		"ensured":         len(seeds),
		"createdEventIds": created,
	})
}

func (h *AckHandler) UpdateException(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, eventID, ok := h.resolveWorkflowMutation(w, r)
	if !ok {
		return
	}

	var payload updateExceptionRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", map[string]any{"field": "body"}))
		return
	}

	workflow, err := h.workflowRepo.UpdateException(r.Context(), domain.UpdateExceptionInput{
		TenantID:       tenantID,
		ActorUserID:    userID,
		EventID:        eventID,
		Priority:       payload.Priority,
		Category:       payload.Category,
		BusinessImpact: payload.BusinessImpact,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, toExceptionWorkflowResponse(workflow))
}

func toExceptionWorkflowResponse(workflow domain.CriticalEventWorkflow) map[string]any {
	now := time.Now().UTC()
	return map[string]any{
		"eventId":         workflow.EventID,
		"status":          workflow.Status,
		"exception":       toExceptionDetailsResponse(workflow, now),
		"acknowledgement": workflowSummaryAck(workflow),
		"assignment":      workflowSummaryAssignment(workflow),
		"resolution":      workflowSummaryResolution(workflow),
	}
}

func workflowSummaryAck(workflow domain.CriticalEventWorkflow) any {
	if workflow.AcknowledgedAt == nil || workflow.AcknowledgedByUserID == nil {
		return nil
	}
	return map[string]any{
		"acknowledgedAt": workflow.AcknowledgedAt.UTC().Format(time.RFC3339),
		"userId":         workflow.AcknowledgedByUserID.String(),
	}
}

func workflowSummaryAssignment(workflow domain.CriticalEventWorkflow) any {
	if workflow.AssignedAt == nil || workflow.AssignedToUserID == nil || workflow.AssignedByUserID == nil {
		return nil
	}
	return map[string]any{
		"assignedToUserId": workflow.AssignedToUserID.String(),
		"assignedByUserId": workflow.AssignedByUserID.String(),
		"assignedAt":       workflow.AssignedAt.UTC().Format(time.RFC3339),
	}
}

func workflowSummaryResolution(workflow domain.CriticalEventWorkflow) any {
	if workflow.ResolvedAt == nil || workflow.ResolvedByUserID == nil || workflow.ResolutionCode == nil {
		return nil
	}
	return map[string]any{
		"resolvedByUserId": workflow.ResolvedByUserID.String(),
		"resolvedAt":       workflow.ResolvedAt.UTC().Format(time.RFC3339),
		"resolutionCode":   *workflow.ResolutionCode,
		"comment":          workflow.ResolutionComment,
	}
}
