package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	apperrors "github.com/freight-platform/control-tower-read-model-service/internal/platform/errors"
	"github.com/freight-platform/control-tower-read-model-service/internal/platform/respond"
	"github.com/freight-platform/control-tower-read-model-service/internal/repository"
)

type AckStore interface {
	UpsertAcknowledgement(ctx context.Context, input domain.AcknowledgeCriticalEventInput) (domain.CriticalEventAcknowledgement, error)
	LookupAcknowledgements(ctx context.Context, tenantID uuid.UUID, eventIDs []string) ([]domain.CriticalEventAcknowledgement, error)
}

type WorkflowStore interface {
	AcknowledgeWithWorkflow(ctx context.Context, input domain.AcknowledgeCriticalEventInput) (domain.CriticalEventAcknowledgement, domain.CriticalEventWorkflow, error)
	AssignCriticalEvent(ctx context.Context, input domain.AssignCriticalEventInput) (domain.CriticalEventWorkflow, error)
	ResolveCriticalEvent(ctx context.Context, input domain.ResolveCriticalEventInput) (domain.CriticalEventWorkflow, error)
	ReopenCriticalEvent(ctx context.Context, input domain.ReopenCriticalEventInput) (domain.CriticalEventWorkflow, error)
	ListActions(ctx context.Context, tenantID uuid.UUID, eventID string) ([]domain.CriticalEventAction, error)
	LookupWorkflows(ctx context.Context, tenantID uuid.UUID, eventIDs []string) ([]domain.CriticalEventWorkflow, error)
	EnsureExceptionWorkflows(ctx context.Context, tenantID uuid.UUID, seeds []domain.EnsureExceptionSeed) error
	UpdateException(ctx context.Context, input domain.UpdateExceptionInput) (domain.CriticalEventWorkflow, error)
	LookupWorkflowsWithExceptionProcessing(ctx context.Context, tenantID uuid.UUID, eventIDs []string, actorUserID uuid.UUID) ([]domain.CriticalEventWorkflow, error)
}

type AckHandler struct {
	repo         AckStore
	workflowRepo WorkflowStore
}

func NewAckHandler(repo *repository.AckRepository, workflowRepo *repository.WorkflowRepository) *AckHandler {
	return &AckHandler{repo: repo, workflowRepo: workflowRepo}
}

type acknowledgeRequest struct {
	ShipmentID string `json:"shipmentId"`
	EventType  string `json:"eventType"`
	OccurredAt string `json:"occurredAt"`
	Source     string `json:"source"`
}

type acknowledgementResponse struct {
	EventID        string                 `json:"eventId"`
	ShipmentID     string                 `json:"shipmentId"`
	EventType      string                 `json:"eventType"`
	OccurredAt     string                 `json:"occurredAt"`
	Source         string                 `json:"source"`
	AcknowledgedAt string                 `json:"acknowledgedAt"`
	AcknowledgedBy acknowledgedByResponse `json:"acknowledgedBy"`
	Status         string                 `json:"status"`
}

type acknowledgedByResponse struct {
	UserID string `json:"userId"`
}

type lookupRequest struct {
	EventIDs []string `json:"eventIds"`
}

type lookupItemResponse struct {
	EventID              string `json:"eventId"`
	AcknowledgedAt       string `json:"acknowledgedAt"`
	AcknowledgedByUserID string `json:"acknowledgedByUserId"`
}

type lookupResponse struct {
	Items []lookupItemResponse `json:"items"`
}

type workflowLookupRequest struct {
	EventIDs []string `json:"eventIds"`
}

type workflowLookupItemResponse struct {
	EventID string `json:"eventId"`
	Status  string `json:"status"`
	WorkflowSummaryResponse
	Exception exceptionDetailsResponse `json:"exception"`
}

type workflowLookupResponse struct {
	Items []workflowLookupItemResponse `json:"items"`
}

type WorkflowSummaryResponse struct {
	Acknowledgement *acknowledgementSummaryResponse `json:"acknowledgement,omitempty"`
	Assignment      *assignmentSummaryResponse      `json:"assignment,omitempty"`
	Resolution      *resolutionSummaryResponse      `json:"resolution,omitempty"`
}

type acknowledgementSummaryResponse struct {
	AcknowledgedAt string `json:"acknowledgedAt"`
	UserID         string `json:"userId"`
}

type assignmentSummaryResponse struct {
	AssignedToUserID string `json:"assignedToUserId"`
	AssignedByUserID string `json:"assignedByUserId"`
	AssignedAt       string `json:"assignedAt"`
}

type resolutionSummaryResponse struct {
	ResolvedByUserID string  `json:"resolvedByUserId"`
	ResolvedAt       string  `json:"resolvedAt"`
	ResolutionCode   string  `json:"resolutionCode"`
	Comment          *string `json:"comment,omitempty"`
}

type assignRequest struct {
	UserID string `json:"userId"`
}

type resolveRequest struct {
	ResolutionCode string  `json:"resolutionCode"`
	Comment        *string `json:"comment"`
}

type actionItemResponse struct {
	ActionType  string         `json:"actionType"`
	ActorUserID string         `json:"actorUserId"`
	OccurredAt  string         `json:"occurredAt"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type actionsResponse struct {
	Items []actionItemResponse `json:"items"`
}

func (h *AckHandler) AcknowledgeCriticalEvent(w http.ResponseWriter, r *http.Request) {
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	userID, err := resolveVerifiedUser(r)
	if err != nil {
		respond.Error(w, err)
		return
	}

	eventID := strings.ToLower(strings.TrimSpace(chi.URLParam(r, "eventId")))
	if !eventIDPattern.MatchString(eventID) {
		respond.Error(w, apperrors.Validation("invalid eventId", map[string]any{"field": "eventId"}))
		return
	}

	body, err := readOptionalJSONBody(r)
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", map[string]any{"field": "body"}))
		return
	}
	var payload acknowledgeRequest
	if len(body) > 0 {
		if err := json.Unmarshal(body, &payload); err != nil {
			respond.Error(w, apperrors.Validation("invalid request body", map[string]any{"field": "body"}))
			return
		}
	}

	shipmentID, err := uuid.Parse(strings.TrimSpace(payload.ShipmentID))
	if err != nil || shipmentID == uuid.Nil {
		respond.Error(w, apperrors.Validation("invalid shipmentId", map[string]any{"field": "shipmentId"}))
		return
	}
	eventType := strings.TrimSpace(payload.EventType)
	if eventType == "" {
		respond.Error(w, apperrors.Validation("eventType is required", map[string]any{"field": "eventType"}))
		return
	}
	occurredAt, err := time.Parse(time.RFC3339, strings.TrimSpace(payload.OccurredAt))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid occurredAt", map[string]any{"field": "occurredAt"}))
		return
	}

	source := strings.TrimSpace(payload.Source)
	if source == "" {
		source = "control-tower"
	}

	input := domain.AcknowledgeCriticalEventInput{
		TenantID:   tenantID,
		UserID:     userID,
		EventID:    eventID,
		ShipmentID: shipmentID,
		EventType:  eventType,
		Source:     source,
		OccurredAt: occurredAt,
	}

	record, workflow, err := h.workflowRepo.AcknowledgeWithWorkflow(r.Context(), input)
	if err != nil {
		respond.Error(w, err)
		return
	}

	resp := toAcknowledgementResponse(record)
	resp.Status = workflow.Status
	respond.JSON(w, http.StatusOK, resp)
}

func (h *AckHandler) LookupAcknowledgements(w http.ResponseWriter, r *http.Request) {
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}

	var payload lookupRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", map[string]any{"field": "body"}))
		return
	}

	items, err := h.repo.LookupAcknowledgements(r.Context(), tenantID, payload.EventIDs)
	if err != nil {
		respond.Error(w, apperrors.Internal("failed to lookup acknowledgements", err))
		return
	}

	resp := lookupResponse{Items: make([]lookupItemResponse, 0, len(items))}
	for _, item := range items {
		resp.Items = append(resp.Items, lookupItemResponse{
			EventID:              item.EventID,
			AcknowledgedAt:       item.AcknowledgedAt.UTC().Format(time.RFC3339),
			AcknowledgedByUserID: item.AcknowledgedByUserID.String(),
		})
	}
	respond.JSON(w, http.StatusOK, resp)
}

func (h *AckHandler) LookupWorkflows(w http.ResponseWriter, r *http.Request) {
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	userID, _ := resolveVerifiedUser(r)

	var payload workflowLookupRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", map[string]any{"field": "body"}))
		return
	}

	items, err := h.workflowRepo.LookupWorkflowsWithExceptionProcessing(r.Context(), tenantID, payload.EventIDs, userID)
	if err != nil {
		respond.Error(w, apperrors.Internal("failed to lookup workflows", err))
		return
	}

	now := time.Now().UTC()
	resp := workflowLookupResponse{Items: make([]workflowLookupItemResponse, 0, len(items))}
	for _, item := range items {
		resp.Items = append(resp.Items, workflowLookupItemResponse{
			EventID:                 item.EventID,
			Status:                  item.Status,
			WorkflowSummaryResponse: toWorkflowSummaryResponse(item),
			Exception:               toExceptionDetailsResponse(item, now),
		})
	}
	respond.JSON(w, http.StatusOK, resp)
}

func (h *AckHandler) AssignCriticalEvent(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, eventID, ok := h.resolveWorkflowMutation(w, r)
	if !ok {
		return
	}

	var payload assignRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", map[string]any{"field": "body"}))
		return
	}
	assignee, err := uuid.Parse(strings.TrimSpace(payload.UserID))
	if err != nil || assignee == uuid.Nil {
		respond.Error(w, apperrors.Validation("invalid userId", map[string]any{"field": "userId"}))
		return
	}

	workflow, err := h.workflowRepo.AssignCriticalEvent(r.Context(), domain.AssignCriticalEventInput{
		TenantID:       tenantID,
		ActorUserID:    userID,
		EventID:        eventID,
		AssignedToUser: assignee,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, toWorkflowResponse(workflow))
}

func (h *AckHandler) ResolveCriticalEvent(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, eventID, ok := h.resolveWorkflowMutation(w, r)
	if !ok {
		return
	}

	var payload resolveRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", map[string]any{"field": "body"}))
		return
	}

	workflow, err := h.workflowRepo.ResolveCriticalEvent(r.Context(), domain.ResolveCriticalEventInput{
		TenantID:          tenantID,
		ActorUserID:       userID,
		EventID:           eventID,
		ResolutionCode:    strings.TrimSpace(payload.ResolutionCode),
		ResolutionComment: payload.Comment,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, toWorkflowResponse(workflow))
}

func (h *AckHandler) ReopenCriticalEvent(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, eventID, ok := h.resolveWorkflowMutation(w, r)
	if !ok {
		return
	}

	if err := ensureEmptyBody(r); err != nil {
		respond.Error(w, err)
		return
	}

	workflow, err := h.workflowRepo.ReopenCriticalEvent(r.Context(), domain.ReopenCriticalEventInput{
		TenantID:    tenantID,
		ActorUserID: userID,
		EventID:     eventID,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, toWorkflowResponse(workflow))
}

func (h *AckHandler) ListCriticalEventActions(w http.ResponseWriter, r *http.Request) {
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}

	eventID := strings.ToLower(strings.TrimSpace(chi.URLParam(r, "eventId")))
	if !eventIDPattern.MatchString(eventID) {
		respond.Error(w, apperrors.Validation("invalid eventId", map[string]any{"field": "eventId"}))
		return
	}

	items, err := h.workflowRepo.ListActions(r.Context(), tenantID, eventID)
	if err != nil {
		respond.Error(w, apperrors.Internal("failed to list actions", err))
		return
	}

	resp := actionsResponse{Items: make([]actionItemResponse, 0, len(items))}
	for _, item := range items {
		resp.Items = append(resp.Items, actionItemResponse{
			ActionType:  item.ActionType,
			ActorUserID: item.ActorUserID.String(),
			OccurredAt:  item.OccurredAt.UTC().Format(time.RFC3339),
			Metadata:    item.Metadata,
		})
	}
	respond.JSON(w, http.StatusOK, resp)
}

func (h *AckHandler) resolveWorkflowMutation(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, string, bool) {
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return uuid.Nil, uuid.Nil, "", false
	}
	userID, err := resolveVerifiedUser(r)
	if err != nil {
		respond.Error(w, err)
		return uuid.Nil, uuid.Nil, "", false
	}

	eventID := strings.ToLower(strings.TrimSpace(chi.URLParam(r, "eventId")))
	if !eventIDPattern.MatchString(eventID) {
		respond.Error(w, apperrors.Validation("invalid eventId", map[string]any{"field": "eventId"}))
		return uuid.Nil, uuid.Nil, "", false
	}
	return tenantID, userID, eventID, true
}

func toAcknowledgementResponse(record domain.CriticalEventAcknowledgement) acknowledgementResponse {
	source := record.Source
	if source == "" {
		source = "control-tower"
	}
	return acknowledgementResponse{
		EventID:        record.EventID,
		ShipmentID:     record.ShipmentID.String(),
		EventType:      record.EventType,
		OccurredAt:     record.OccurredAt.UTC().Format(time.RFC3339),
		Source:         source,
		AcknowledgedAt: record.AcknowledgedAt.UTC().Format(time.RFC3339),
		AcknowledgedBy: acknowledgedByResponse{UserID: record.AcknowledgedByUserID.String()},
		Status:         domain.WorkflowStatusAcknowledged,
	}
}

type workflowResponse struct {
	EventID string `json:"eventId"`
	Status  string `json:"status"`
	WorkflowSummaryResponse
}

func toWorkflowResponse(workflow domain.CriticalEventWorkflow) workflowResponse {
	return workflowResponse{
		EventID:                 workflow.EventID,
		Status:                  workflow.Status,
		WorkflowSummaryResponse: toWorkflowSummaryResponse(workflow),
	}
}

func toWorkflowSummaryResponse(workflow domain.CriticalEventWorkflow) WorkflowSummaryResponse {
	resp := WorkflowSummaryResponse{}
	if workflow.AcknowledgedAt != nil && workflow.AcknowledgedByUserID != nil {
		resp.Acknowledgement = &acknowledgementSummaryResponse{
			AcknowledgedAt: workflow.AcknowledgedAt.UTC().Format(time.RFC3339),
			UserID:         workflow.AcknowledgedByUserID.String(),
		}
	}
	if workflow.AssignedAt != nil && workflow.AssignedToUserID != nil && workflow.AssignedByUserID != nil {
		resp.Assignment = &assignmentSummaryResponse{
			AssignedToUserID: workflow.AssignedToUserID.String(),
			AssignedByUserID: workflow.AssignedByUserID.String(),
			AssignedAt:       workflow.AssignedAt.UTC().Format(time.RFC3339),
		}
	}
	if workflow.ResolvedAt != nil && workflow.ResolvedByUserID != nil && workflow.ResolutionCode != nil {
		resp.Resolution = &resolutionSummaryResponse{
			ResolvedByUserID: workflow.ResolvedByUserID.String(),
			ResolvedAt:       workflow.ResolvedAt.UTC().Format(time.RFC3339),
			ResolutionCode:   *workflow.ResolutionCode,
			Comment:          workflow.ResolutionComment,
		}
	}
	return resp
}

func readOptionalJSONBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, 4096))
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, nil
	}
	return body, nil
}

func ensureEmptyBody(r *http.Request) error {
	body, err := readOptionalJSONBody(r)
	if err != nil {
		return apperrors.Validation("invalid request body", map[string]any{"field": "body"})
	}
	if len(body) == 0 {
		return nil
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return apperrors.Validation("request body must be empty", map[string]any{"field": "body"})
	}
	if len(payload) > 0 {
		return apperrors.Validation("request body must be empty", map[string]any{"field": "body"})
	}
	return nil
}
