package handlers

import (
	"context"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	apperrors "github.com/freight-platform/control-tower-read-model-service/internal/platform/errors"
	"github.com/freight-platform/control-tower-read-model-service/internal/platform/respond"
	"github.com/freight-platform/control-tower-read-model-service/internal/repository"
)

type ShipmentOwner interface {
	Owns(ctx context.Context, tenantID, shipmentID uuid.UUID) (bool, error)
}

type ExecutionHandler struct {
	repo  *repository.ExecutionProjectionRepository
	owner ShipmentOwner
}

func NewExecutionHandler(repo *repository.ExecutionProjectionRepository, owner ShipmentOwner) *ExecutionHandler {
	return &ExecutionHandler{repo: repo, owner: owner}
}

func (h *ExecutionHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, parseErr := strconv.Atoi(raw)
		if parseErr != nil || parsed < 1 || parsed > 200 {
			respond.Error(w, apperrors.Validation("invalid limit", nil))
			return
		}
		limit = parsed
	}
	offset := 0
	if raw := r.URL.Query().Get("offset"); raw != "" {
		parsed, parseErr := strconv.Atoi(raw)
		if parseErr != nil || parsed < 0 {
			respond.Error(w, apperrors.Validation("invalid offset", nil))
			return
		}
		offset = parsed
	}
	driverID, err := optionalQueryUUID(r, "driverId")
	if err != nil {
		respond.Error(w, err)
		return
	}
	vehicleID, err := optionalQueryUUID(r, "vehicleId")
	if err != nil {
		respond.Error(w, err)
		return
	}
	rows, err := h.repo.List(r.Context(), tenantID, driverID, vehicleID, limit, offset)
	if err != nil {
		respond.Error(w, err)
		return
	}
	out := make([]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, executionResponse(row, nil))
	}
	respond.JSON(w, http.StatusOK, map[string]any{"executions": out})
}

func (h *ExecutionHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	executionID, err := domain.ParseUUID(chi.URLParam(r, "executionId"), "executionId")
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid executionId", nil))
		return
	}
	view, err := h.repo.Get(r.Context(), executionID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if view == nil {
		respond.Error(w, apperrors.NotFound("execution not found"))
		return
	}
	if view.OperatingTenantID == tenantID {
		respond.JSON(w, http.StatusOK, executionResponse(*view, nil))
		return
	}
	if h.owner == nil {
		respond.Error(w, apperrors.NotFound("execution not found"))
		return
	}
	allowed := map[uuid.UUID]struct{}{}
	for _, action := range view.Actions {
		owns, ownErr := h.owner.Owns(r.Context(), tenantID, action.ShipmentID)
		if ownErr != nil {
			respond.Error(w, ownErr)
			return
		}
		if owns {
			allowed[action.ShipmentID] = struct{}{}
		}
	}
	if len(allowed) == 0 {
		respond.Error(w, apperrors.NotFound("execution not found"))
		return
	}
	respond.JSON(w, http.StatusOK, executionResponse(*view, allowed))
}

func optionalQueryUUID(r *http.Request, name string) (*uuid.UUID, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return nil, nil
	}
	id, err := domain.ParseUUID(raw, name)
	if err != nil {
		return nil, apperrors.Validation("invalid "+name, nil)
	}
	return &id, nil
}

func executionResponse(view repository.ExecutionView, allowedShipments map[uuid.UUID]struct{}) map[string]any {
	stops := make([]any, 0)
	stopVisible := map[uuid.UUID]struct{}{}
	actions := make([]any, 0)
	for _, action := range view.Actions {
		if allowedShipments != nil {
			if _, ok := allowedShipments[action.ShipmentID]; !ok {
				continue
			}
		}
		stopVisible[action.ExecutionStopID] = struct{}{}
		actions = append(actions, map[string]any{
			"actionId": action.ActionID, "executionStopId": action.ExecutionStopID,
			"actionType": action.ActionType, "shipmentId": action.ShipmentID,
			"cargoId": action.CargoID, "status": action.Status,
		})
	}
	for _, stop := range view.Stops {
		if allowedShipments != nil {
			if _, ok := stopVisible[stop.ExecutionStopID]; !ok {
				continue
			}
		}
		stops = append(stops, map[string]any{
			"executionStopId": stop.ExecutionStopID, "ordinal": stop.Ordinal,
			"stopRole": stop.StopRole, "pointKind": stop.PointKind, "locationId": stop.LocationID,
			"status": stop.Status, "arrivedAt": stop.ArrivedAt, "serviceStartedAt": stop.ServiceStartedAt,
			"completedAt": stop.CompletedAt, "approachingAt": stop.ApproachingAt,
			"approachDistanceMeters": stop.ApproachDistanceMeters,
			"plannedArrival":         stop.PlannedArrival, "plannedDeparture": stop.PlannedDeparture,
		})
	}
	progress := make([]any, 0, len(view.Progress))
	for _, row := range view.Progress {
		if allowedShipments != nil {
			if row.ShipmentID != nil {
				if _, ok := allowedShipments[*row.ShipmentID]; !ok {
					continue
				}
			} else if row.ExecutionStopID != nil {
				if _, ok := stopVisible[*row.ExecutionStopID]; !ok {
					continue
				}
			} else {
				continue
			}
		}
		progress = append(progress, map[string]any{
			"eventId": row.EventID, "eventType": row.EventType, "executionStopId": row.ExecutionStopID,
			"actionId": row.ActionID, "shipmentId": row.ShipmentID, "reasonCode": row.ReasonCode,
			"severity": row.Severity, "occurredAt": row.OccurredAt,
		})
	}
	return map[string]any{
		"executionId": view.ExecutionID, "activeRevisionId": view.ActiveRevisionID,
		"carrierCompanyId": view.CarrierCompanyID, "driverId": view.DriverID, "vehicleId": view.VehicleID,
		"currentStopId": view.CurrentStopID, "lastEventSequence": view.LastEventSequence,
		"lastEventType": view.LastEventType, "gapDetected": view.GapDetected,
		"stops": stops, "actions": actions, "progress": progress,
	}
}
