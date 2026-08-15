package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	apperrors "github.com/freight-platform/control-tower-read-model-service/internal/platform/errors"
	"github.com/freight-platform/control-tower-read-model-service/internal/platform/respond"
	"github.com/freight-platform/control-tower-read-model-service/internal/repository"
	"github.com/freight-platform/control-tower-read-model-service/internal/service"
)

type GuardedActionHandler struct {
	actions *repository.GuardedActionRepository
	svc     *service.GuardedActionService
	token   string
}

func NewGuardedActionHandler(actions *repository.GuardedActionRepository, svc *service.GuardedActionService, internalToken string) *GuardedActionHandler {
	return &GuardedActionHandler{actions: actions, svc: svc, token: strings.TrimSpace(internalToken)}
}

func (h *GuardedActionHandler) ApproveAction(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, ok := resolveWorkspaceContext(w, r)
	if !ok {
		return
	}
	ctx := service.WithAutomationPermissions(r.Context(), parseAutomationPermissions(r)...)
	actionID, err := uuid.Parse(chi.URLParam(r, "actionId"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid action id", nil))
		return
	}
	action, err := h.svc.ApproveAction(ctx, tenantID, userID, actionID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toGuardedActionResponse(action))
}

func (h *GuardedActionHandler) RejectAction(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, ok := resolveWorkspaceContext(w, r)
	if !ok {
		return
	}
	ctx := service.WithAutomationPermissions(r.Context(), parseAutomationPermissions(r)...)
	actionID, err := uuid.Parse(chi.URLParam(r, "actionId"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid action id", nil))
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	action, err := h.svc.RejectAction(ctx, tenantID, userID, actionID, body.Reason)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toGuardedActionResponse(action))
}

func (h *GuardedActionHandler) ListExecutionActions(w http.ResponseWriter, r *http.Request) {
	tenantID, _, ok := resolveWorkspaceContext(w, r)
	if !ok {
		return
	}
	execID, err := uuid.Parse(chi.URLParam(r, "executionId"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid execution id", nil))
		return
	}
	items, err := h.actions.ListActionsByExecution(r.Context(), tenantID, execID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, toGuardedActionResponse(item))
	}
	respond.JSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *GuardedActionHandler) IngestDriverTaskEvent(w http.ResponseWriter, r *http.Request) {
	if h.token != "" && r.Header.Get("X-Internal-Service-Token") != h.token {
		respond.Error(w, apperrors.Unauthorized("internal authorization required"))
		return
	}
	var body struct {
		TenantID      string          `json:"tenantId"`
		EventType     string          `json:"eventType"`
		TaskID        string          `json:"taskId"`
		SourceEventID string          `json:"sourceEventId"`
		CorrelationID string          `json:"correlationId"`
		Payload       json.RawMessage `json:"payload"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	tenantID, err := uuid.Parse(strings.TrimSpace(body.TenantID))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid tenantId", nil))
		return
	}
	taskID, err := uuid.Parse(strings.TrimSpace(body.TaskID))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid taskId", nil))
		return
	}
	if err := h.svc.HandleDriverTaskEvent(r.Context(), service.DriverTaskEventInput{
		TenantID: tenantID, EventType: body.EventType, TaskID: taskID,
		SourceEventID: body.SourceEventID, CorrelationID: body.CorrelationID, Payload: body.Payload,
	}); err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusAccepted, map[string]any{"accepted": true})
}

func parseAutomationPermissions(r *http.Request) []string {
	raw := strings.TrimSpace(r.Header.Get("X-User-Permissions"))
	if raw == "" {
		return nil
	}
	return strings.Split(raw, ",")
}

func toGuardedActionResponse(action domain.GuardedAction) map[string]any {
	out := map[string]any{
		"id": action.ID.String(), "executionId": action.ExecutionID.String(),
		"executionStepId": action.ExecutionStepID.String(), "actionType": action.ActionType,
		"safetyClass": action.SafetyClass, "guardDecision": action.GuardDecision,
		"guardReason": action.GuardReason, "status": action.Status,
		"correlationId": action.CorrelationID, "sourceEventId": action.SourceEventID,
	}
	if action.DriverID != nil {
		out["driverId"] = action.DriverID.String()
	}
	if action.ShipmentID != nil {
		out["shipmentId"] = action.ShipmentID.String()
	}
	if action.DriverTaskID != nil {
		out["driverTaskId"] = action.DriverTaskID.String()
	}
	if action.ErrorReason != "" {
		out["errorReason"] = action.ErrorReason
	}
	if len(action.ResponsePayload) > 0 {
		out["response"] = json.RawMessage(action.ResponsePayload)
	}
	return out
}
