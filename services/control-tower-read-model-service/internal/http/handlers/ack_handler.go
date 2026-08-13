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

type AckHandler struct {
	repo AckStore
}

func NewAckHandler(repo *repository.AckRepository) *AckHandler {
	return &AckHandler{repo: repo}
}

type acknowledgeRequest struct {
	ShipmentID string `json:"shipmentId"`
	EventType  string `json:"eventType"`
	OccurredAt string `json:"occurredAt"`
	Source     string `json:"source"`
}

type acknowledgementResponse struct {
	EventID        string                     `json:"eventId"`
	ShipmentID     string                     `json:"shipmentId"`
	EventType      string                     `json:"eventType"`
	OccurredAt     string                     `json:"occurredAt"`
	Source         string                     `json:"source"`
	AcknowledgedAt string                     `json:"acknowledgedAt"`
	AcknowledgedBy acknowledgedByResponse     `json:"acknowledgedBy"`
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

	record, err := h.repo.UpsertAcknowledgement(r.Context(), domain.AcknowledgeCriticalEventInput{
		TenantID:   tenantID,
		UserID:     userID,
		EventID:    eventID,
		ShipmentID: shipmentID,
		EventType:  eventType,
		Source:     source,
		OccurredAt: occurredAt,
	})
	if err != nil {
		respond.Error(w, apperrors.Internal("failed to persist acknowledgement", err))
		return
	}

	respond.JSON(w, http.StatusOK, toAcknowledgementResponse(record))
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
	}
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
