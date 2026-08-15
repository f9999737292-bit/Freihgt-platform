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

type DriverTaskHandler struct {
	service *service.DriverTaskService
}

func NewDriverTaskHandler(svc *service.DriverTaskService) *DriverTaskHandler {
	return &DriverTaskHandler{service: svc}
}

func (h *DriverTaskHandler) ListTasks(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, err := resolveDriverContext(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	filter := domain.ListDriverTasksFilter{
		Limit:  parseIntDefault(r.URL.Query().Get("limit"), 50),
		Offset: parseIntDefault(r.URL.Query().Get("offset"), 0),
		Unread: strings.EqualFold(r.URL.Query().Get("unread"), "true"),
	}
	if status := strings.TrimSpace(r.URL.Query().Get("status")); status != "" {
		filter.Status = &status
	}
	items, total, err := h.service.ListTasks(r.Context(), tenantID, userID, filter)
	if err != nil {
		respond.Error(w, err)
		return
	}
	payload := make([]map[string]any, 0, len(items))
	for _, item := range items {
		payload = append(payload, mapDriverTask(item))
	}
	respond.JSON(w, http.StatusOK, map[string]any{"items": payload, "total": total, "limit": filter.Limit, "offset": filter.Offset})
}

func (h *DriverTaskHandler) GetTask(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, err := resolveDriverContext(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	taskID, err := uuid.Parse(chi.URLParam(r, "taskId"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid task id", nil))
		return
	}
	task, err := h.service.GetTask(r.Context(), tenantID, userID, taskID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, mapDriverTaskDetail(*task))
}

func (h *DriverTaskHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, err := resolveDriverContext(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	taskID, err := uuid.Parse(chi.URLParam(r, "taskId"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid task id", nil))
		return
	}
	task, err := h.service.MarkRead(r.Context(), tenantID, userID, taskID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, mapDriverTaskDetail(*task))
}

func (h *DriverTaskHandler) Acknowledge(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, err := resolveDriverContext(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	taskID, err := uuid.Parse(chi.URLParam(r, "taskId"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid task id", nil))
		return
	}
	task, err := h.service.Acknowledge(r.Context(), tenantID, userID, taskID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, mapDriverTaskDetail(*task))
}

func (h *DriverTaskHandler) SubmitResponse(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, err := resolveDriverContext(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	taskID, err := uuid.Parse(chi.URLParam(r, "taskId"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid task id", nil))
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		respond.Error(w, err)
		return
	}
	idem := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	task, resp, err := h.service.SubmitResponse(r.Context(), service.SubmitTaskResponseInput{
		TenantID: tenantID, UserID: userID, TaskID: taskID, IdempotencyKey: idem, Body: body,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{
		"task":     mapDriverTaskDetail(*task),
		"response": mapDriverTaskResponse(*resp),
	})
}

type registerDeviceRequest struct {
	Platform         string  `json:"platform"`
	DeviceInstanceID string  `json:"deviceInstanceId"`
	PushToken        string  `json:"pushToken"`
	AppVersion       *string `json:"appVersion"`
	Locale           *string `json:"locale"`
}

func (h *DriverTaskHandler) RegisterDevice(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, err := resolveDriverContext(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req registerDeviceRequest
	if err := json.Unmarshal(body, &req); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	dev, err := h.service.RegisterDevice(r.Context(), tenantID, userID, req.Platform, req.DeviceInstanceID, req.PushToken, req.AppVersion, req.Locale)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, mapDriverDevice(*dev))
}

func (h *DriverTaskHandler) RevokeDevice(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, err := resolveDriverContext(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	deviceID, err := uuid.Parse(chi.URLParam(r, "deviceId"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid device id", nil))
		return
	}
	if err := h.service.RevokeDevice(r.Context(), tenantID, userID, deviceID); err != nil {
		respond.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func mapDriverTask(task domain.DriverTask) map[string]any {
	out := map[string]any{
		"id": task.ID.String(), "taskType": task.TaskType, "status": task.Status,
		"priority": task.Priority, "title": task.Title,
		"createdAt": task.CreatedAt.UTC().Format(time.RFC3339),
	}
	if task.ShipmentID != nil {
		out["shipmentId"] = task.ShipmentID.String()
	}
	return out
}

func mapDriverTaskDetail(task domain.DriverTask) map[string]any {
	out := mapDriverTask(task)
	out["payload"] = json.RawMessage(task.Payload)
	if task.ExpiresAt != nil {
		out["expiresAt"] = task.ExpiresAt.UTC().Format(time.RFC3339)
	}
	if task.ReadAt != nil {
		out["readAt"] = task.ReadAt.UTC().Format(time.RFC3339)
	}
	if task.AcknowledgedAt != nil {
		out["acknowledgedAt"] = task.AcknowledgedAt.UTC().Format(time.RFC3339)
	}
	if task.CompletedAt != nil {
		out["completedAt"] = task.CompletedAt.UTC().Format(time.RFC3339)
	}
	return out
}

func mapDriverTaskResponse(resp domain.DriverTaskResponse) map[string]any {
	return map[string]any{
		"id": resp.ID.String(), "taskId": resp.TaskID.String(), "responseType": resp.ResponseType,
		"responseBody": json.RawMessage(resp.ResponseBody),
		"receivedAt":   resp.ReceivedAt.UTC().Format(time.RFC3339),
	}
}

func mapDriverDevice(dev domain.DriverDevice) map[string]any {
	out := map[string]any{
		"id": dev.ID.String(), "platform": dev.Platform, "deviceInstanceId": dev.DeviceInstanceID,
		"createdAt": dev.CreatedAt.UTC().Format(time.RFC3339),
	}
	if dev.AppVersion != nil {
		out["appVersion"] = *dev.AppVersion
	}
	if dev.Locale != nil {
		out["locale"] = *dev.Locale
	}
	return out
}
