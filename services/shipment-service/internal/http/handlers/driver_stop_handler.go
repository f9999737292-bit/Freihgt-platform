package handlers

import (
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

type DriverStopHandler struct {
	service *service.DriverStopService
}

func NewDriverStopHandler(svc *service.DriverStopService) *DriverStopHandler {
	return &DriverStopHandler{service: svc}
}

type driverStopCommandRequest struct {
	OccurredAt      *string `json:"occurredAt"`
	ExpectedVersion int     `json:"expectedVersion"`
	ReasonCode      string  `json:"reasonCode"`
	Comment         *string `json:"comment"`
}

func (h *DriverStopHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, err := resolveDriverContext(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	view, err := h.service.List(r.Context(), tenantID, userID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{
		"current": mapDriverStopTask(view.Current),
		"next":    mapDriverStopTask(view.Next),
	})
}

func (h *DriverStopHandler) Arrive(w http.ResponseWriter, r *http.Request) {
	h.command(w, r, false, func(tenantID, userID uuid.UUID, in service.DriverStopCommandInput) (service.DriverStopCommandResult, error) {
		return h.service.Arrive(r.Context(), tenantID, userID, in)
	})
}

func (h *DriverStopHandler) StartService(w http.ResponseWriter, r *http.Request) {
	h.command(w, r, false, func(tenantID, userID uuid.UUID, in service.DriverStopCommandInput) (service.DriverStopCommandResult, error) {
		return h.service.StartService(r.Context(), tenantID, userID, in)
	})
}

func (h *DriverStopHandler) Complete(w http.ResponseWriter, r *http.Request) {
	h.command(w, r, false, func(tenantID, userID uuid.UUID, in service.DriverStopCommandInput) (service.DriverStopCommandResult, error) {
		return h.service.Complete(r.Context(), tenantID, userID, in)
	})
}

func (h *DriverStopHandler) Confirm(w http.ResponseWriter, r *http.Request) {
	h.command(w, r, true, func(tenantID, userID uuid.UUID, in service.DriverStopCommandInput) (service.DriverStopCommandResult, error) {
		return h.service.Confirm(r.Context(), tenantID, userID, in)
	})
}

func (h *DriverStopHandler) Fail(w http.ResponseWriter, r *http.Request) {
	h.command(w, r, true, func(tenantID, userID uuid.UUID, in service.DriverStopCommandInput) (service.DriverStopCommandResult, error) {
		return h.service.Fail(r.Context(), tenantID, userID, in)
	})
}

func (h *DriverStopHandler) command(
	w http.ResponseWriter,
	r *http.Request,
	withAction bool,
	call func(uuid.UUID, uuid.UUID, service.DriverStopCommandInput) (service.DriverStopCommandResult, error),
) {
	tenantID, userID, err := resolveDriverContext(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	stopID, err := domain.ParseUUID(chi.URLParam(r, "stopId"), "stopId")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var actionID uuid.UUID
	if withAction {
		actionID, err = domain.ParseUUID(chi.URLParam(r, "actionId"), "actionId")
		if err != nil {
			respond.Error(w, err)
			return
		}
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" {
		respond.Error(w, apperrors.Validation("Idempotency-Key is required", map[string]any{"field": "Idempotency-Key"}))
		return
	}
	var req driverStopCommandRequest
	if err := decodeStrictJSON(r, &req); err != nil {
		respond.Error(w, err)
		return
	}
	_ = req.Comment
	occurred := time.Now().UTC()
	if req.OccurredAt != nil && strings.TrimSpace(*req.OccurredAt) != "" {
		parsed, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(*req.OccurredAt))
		if parseErr != nil {
			respond.Error(w, apperrors.Validation("occurredAt must be RFC3339", map[string]any{"field": "occurredAt"}))
			return
		}
		occurred = parsed
	}
	result, err := call(tenantID, userID, service.DriverStopCommandInput{
		StopID: stopID, ActionID: actionID, IdempotencyKey: key,
		OccurredAt: occurred, ExpectedVersion: req.ExpectedVersion, ReasonCode: req.ReasonCode,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}
	body := map[string]any{
		"taskId": result.TaskID.String(), "executionStopId": result.ExecutionStopID.String(),
		"status": result.Status, "version": result.Version, "replayed": result.Replayed,
	}
	if result.ActionStatus != "" {
		body["actionStatus"] = result.ActionStatus
	}
	if result.EvidenceID != nil {
		body["evidenceId"] = result.EvidenceID.String()
	}
	if result.ShipmentStatus != "" {
		body["shipmentStatus"] = result.ShipmentStatus
	}
	respond.JSON(w, http.StatusOK, body)
}

func mapDriverStopTask(task *domain.DriverStopTaskView) any {
	if task == nil {
		return nil
	}
	body := map[string]any{
		"taskId": task.TaskID.String(), "executionId": task.ExecutionID.String(),
		"executionStopId": task.ExecutionStopID.String(), "ordinal": task.Ordinal,
		"locationId": task.LocationID.String(), "status": task.Status, "version": task.Version,
		"actionSummary": task.ActionSummary, "position": task.Position,
	}
	if task.ShipmentID != nil {
		body["shipmentId"] = task.ShipmentID.String()
	} else {
		body["shipmentId"] = nil
	}
	if task.PlannedArrival != nil {
		body["plannedArrival"] = task.PlannedArrival.UTC().Format(time.RFC3339)
	} else {
		body["plannedArrival"] = nil
	}
	return body
}
