package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
	"github.com/freight-platform/shipment-service/internal/platform/respond"
	"github.com/freight-platform/shipment-service/internal/service"
)

type InternalDriverTaskHandler struct {
	service       *service.DriverTaskService
	internalToken string
}

func NewInternalDriverTaskHandler(svc *service.DriverTaskService, internalToken string) *InternalDriverTaskHandler {
	return &InternalDriverTaskHandler{service: svc, internalToken: strings.TrimSpace(internalToken)}
}

type internalCreateTaskRequest struct {
	TenantID       string  `json:"tenantId"`
	DriverID       string  `json:"driverId"`
	ShipmentID     *string `json:"shipmentId"`
	Type           string  `json:"type"`
	Priority       string  `json:"priority"`
	ExpiresAt      *string `json:"expiresAt"`
	Source         string  `json:"source"`
	SourceEventID  *string `json:"sourceEventId"`
	CorrelationID  *string `json:"correlationId"`
	IdempotencyKey string  `json:"idempotencyKey"`
	CreatedByType  string  `json:"createdByType"`
	CreatedByID    *string `json:"createdById"`
}

func (h *InternalDriverTaskHandler) CreateTask(w http.ResponseWriter, r *http.Request) {
	if !h.authorizeInternal(r) {
		respond.Error(w, apperrors.Unauthorized("internal authorization required"))
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req internalCreateTaskRequest
	if err := json.Unmarshal(body, &req); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	tenantID, err := uuid.Parse(strings.TrimSpace(req.TenantID))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid tenantId", nil))
		return
	}
	driverID, err := uuid.Parse(strings.TrimSpace(req.DriverID))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid driverId", nil))
		return
	}
	var shipmentID *uuid.UUID
	if req.ShipmentID != nil && strings.TrimSpace(*req.ShipmentID) != "" {
		id, err := uuid.Parse(strings.TrimSpace(*req.ShipmentID))
		if err != nil {
			respond.Error(w, apperrors.Validation("invalid shipmentId", nil))
			return
		}
		shipmentID = &id
	}
	var expiresAt *time.Time
	if req.ExpiresAt != nil && strings.TrimSpace(*req.ExpiresAt) != "" {
		t, err := time.Parse(time.RFC3339, strings.TrimSpace(*req.ExpiresAt))
		if err != nil {
			respond.Error(w, apperrors.Validation("invalid expiresAt", nil))
			return
		}
		expiresAt = &t
	}
	createdByType := strings.TrimSpace(req.CreatedByType)
	if createdByType == "" {
		createdByType = domain.DriverTaskCreatorControlTower
	}
	var createdByID *uuid.UUID
	if req.CreatedByID != nil && strings.TrimSpace(*req.CreatedByID) != "" {
		id, err := uuid.Parse(strings.TrimSpace(*req.CreatedByID))
		if err != nil {
			respond.Error(w, apperrors.Validation("invalid createdById", nil))
			return
		}
		createdByID = &id
	}
	task, err := h.service.CreateTaskInternal(r.Context(), domain.CreateDriverTaskInput{
		TenantID: tenantID, DriverID: driverID, ShipmentID: shipmentID,
		TaskType: strings.TrimSpace(req.Type), Priority: strings.TrimSpace(req.Priority),
		ExpiresAt: expiresAt, Source: strings.TrimSpace(req.Source),
		SourceEventID: req.SourceEventID, CorrelationID: req.CorrelationID,
		IdempotencyKey: strings.TrimSpace(req.IdempotencyKey),
		CreatedByType: createdByType, CreatedByID: createdByID,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, map[string]any{
		"taskId": task.ID.String(), "status": task.Status, "taskType": task.TaskType,
	})
}

func (h *InternalDriverTaskHandler) CancelTask(w http.ResponseWriter, r *http.Request) {
	if !h.authorizeInternal(r) {
		respond.Error(w, apperrors.Unauthorized("internal authorization required"))
		return
	}
	tenantHeader := strings.TrimSpace(r.Header.Get("X-Tenant-ID"))
	tenantID, err := uuid.Parse(tenantHeader)
	if err != nil {
		respond.Error(w, apperrors.Validation("X-Tenant-ID required", nil))
		return
	}
	taskID, err := uuid.Parse(chi.URLParam(r, "taskId"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid task id", nil))
		return
	}
	task, err := h.service.CancelTaskInternal(r.Context(), tenantID, taskID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{"taskId": task.ID.String(), "status": task.Status})
}

func (h *InternalDriverTaskHandler) authorizeInternal(r *http.Request) bool {
	if h.internalToken == "" {
		return false
	}
	return r.Header.Get("X-Internal-Service-Token") == h.internalToken
}
