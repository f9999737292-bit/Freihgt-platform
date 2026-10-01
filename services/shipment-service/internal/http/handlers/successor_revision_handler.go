package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
	"github.com/freight-platform/shipment-service/internal/platform/respond"
)

// SuccessorRevisionWriter creates the next revision of one transport execution.
// Operating tenant comes from the verified gateway header, never from the body.
type SuccessorRevisionWriter interface {
	CreateSuccessorRevision(ctx context.Context, cmd domain.SuccessorRevisionCommand) (domain.SuccessorRevisionResult, error)
}

type SuccessorRevisionHandler struct {
	writer SuccessorRevisionWriter
}

func NewSuccessorRevisionHandler(writer SuccessorRevisionWriter) *SuccessorRevisionHandler {
	return &SuccessorRevisionHandler{writer: writer}
}

type successorRevisionBody struct {
	ExpectedCurrentRevisionID string                `json:"expectedCurrentRevisionId"`
	IdempotencyKey            string                `json:"idempotencyKey"`
	ReasonCode                string                `json:"reasonCode"`
	OccurredAt                string                `json:"occurredAt"`
	ActorKind                 string                `json:"actorKind"`
	ActorID                   string                `json:"actorId"`
	SourceRoutePlanID         string                `json:"sourceRoutePlanId"`
	SourceRoutePlanVersion    int                   `json:"sourceRoutePlanVersion"`
	SourceActivationID        string                `json:"sourceActivationId"`
	SourceActivationVersion   int                   `json:"sourceActivationVersion"`
	PlanningMode              string                `json:"planningMode"`
	EvaluationFingerprint     string                `json:"evaluationFingerprint"`
	Stops                     []successorStopBody   `json:"stops"`
	Actions                   []successorActionBody `json:"actions"`
}

type successorStopBody struct {
	StopRole               string  `json:"stopRole"`
	PointKind              string  `json:"pointKind"`
	LocationID             string  `json:"locationId"`
	Latitude               float64 `json:"latitude"`
	Longitude              float64 `json:"longitude"`
	PlannedArrival         string  `json:"plannedArrival"`
	PlannedDeparture       string  `json:"plannedDeparture"`
	ServiceDurationSeconds *int    `json:"serviceDurationSeconds"`
}

type successorActionBody struct {
	StopIndex  int    `json:"stopIndex"`
	ActionType string `json:"actionType"`
	ShipmentID string `json:"shipmentId"`
	CargoID    string `json:"cargoId"`
}

func (h *SuccessorRevisionHandler) Create(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.writer == nil {
		respond.Error(w, apperrors.NotFound("transport execution not found"))
		return
	}
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	executionID, err := domain.ParseUUID(chi.URLParam(r, "executionId"), "execution_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var body successorRevisionBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, apperrors.Validation("request body is invalid", map[string]any{"field": "body"}))
		return
	}
	cmd, err := body.command(tenantID, executionID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	result, err := h.writer.CreateSuccessorRevision(r.Context(), cmd)
	if err != nil {
		respond.Error(w, err)
		return
	}
	status := http.StatusCreated
	if result.Replayed {
		status = http.StatusOK
	}
	respond.JSON(w, status, map[string]any{
		"commandId":          result.CommandID,
		"executionId":        result.ExecutionID,
		"previousRevisionId": result.PreviousRevisionID,
		"revisionId":         result.RevisionID,
		"currentStopId":      result.CurrentStopID,
		"replayed":           result.Replayed,
	})
}

func (body successorRevisionBody) command(tenantID, executionID uuid.UUID) (domain.SuccessorRevisionCommand, error) {
	expected, err := domain.ParseUUID(body.ExpectedCurrentRevisionID, "expected_current_revision_id")
	if err != nil {
		return domain.SuccessorRevisionCommand{}, err
	}
	occurred, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(body.OccurredAt))
	if err != nil {
		occurred, err = time.Parse(time.RFC3339, strings.TrimSpace(body.OccurredAt))
		if err != nil {
			return domain.SuccessorRevisionCommand{}, apperrors.Validation("occurred_at is invalid", map[string]any{"field": "occurred_at"})
		}
	}
	planID, err := domain.ParseUUID(body.SourceRoutePlanID, "source_route_plan_id")
	if err != nil {
		return domain.SuccessorRevisionCommand{}, err
	}
	activationID, err := domain.ParseUUID(body.SourceActivationID, "source_activation_id")
	if err != nil {
		return domain.SuccessorRevisionCommand{}, err
	}
	var actorID uuid.UUID
	if strings.TrimSpace(body.ActorID) != "" {
		actorID, err = domain.ParseUUID(body.ActorID, "actor_id")
		if err != nil {
			return domain.SuccessorRevisionCommand{}, err
		}
	}
	stops := make([]domain.SuccessorStopInput, len(body.Stops))
	for i, stop := range body.Stops {
		var locationID *uuid.UUID
		if strings.TrimSpace(stop.LocationID) != "" {
			parsed, parseErr := domain.ParseUUID(stop.LocationID, "location_id")
			if parseErr != nil {
				return domain.SuccessorRevisionCommand{}, parseErr
			}
			locationID = &parsed
		}
		arrival, err := parseOptionalTime(stop.PlannedArrival, "planned_arrival")
		if err != nil {
			return domain.SuccessorRevisionCommand{}, err
		}
		departure, err := parseOptionalTime(stop.PlannedDeparture, "planned_departure")
		if err != nil {
			return domain.SuccessorRevisionCommand{}, err
		}
		stops[i] = domain.SuccessorStopInput{
			StopRole: stop.StopRole, PointKind: stop.PointKind, LocationID: locationID,
			Latitude: stop.Latitude, Longitude: stop.Longitude,
			PlannedArrival: arrival, PlannedDeparture: departure,
			ServiceDurationSeconds: stop.ServiceDurationSeconds,
		}
	}
	actions := make([]domain.SuccessorActionInput, len(body.Actions))
	for i, action := range body.Actions {
		shipmentID, err := domain.ParseUUID(action.ShipmentID, "shipment_id")
		if err != nil {
			return domain.SuccessorRevisionCommand{}, err
		}
		cargoID, err := domain.ParseUUID(action.CargoID, "cargo_id")
		if err != nil {
			return domain.SuccessorRevisionCommand{}, err
		}
		actions[i] = domain.SuccessorActionInput{
			StopIndex: action.StopIndex, ActionType: action.ActionType,
			ShipmentID: shipmentID, CargoID: cargoID,
		}
	}
	return domain.SuccessorRevisionCommand{
		ExecutionID: executionID, ExpectedRevisionID: expected, OperatingTenantID: tenantID,
		IdempotencyKey: body.IdempotencyKey, ReasonCode: body.ReasonCode, OccurredAt: occurred,
		ActorKind: strings.TrimSpace(body.ActorKind), ActorID: actorID,
		SourceRoutePlanID: planID, SourceRoutePlanVersion: body.SourceRoutePlanVersion,
		SourceActivationID: activationID, SourceActivationVersion: body.SourceActivationVersion,
		PlanningMode: body.PlanningMode, EvaluationFingerprint: body.EvaluationFingerprint,
		Stops: stops, Actions: actions,
	}, nil
}

func parseOptionalTime(value, field string) (*time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, trimmed)
	if err != nil {
		parsed, err = time.Parse(time.RFC3339, trimmed)
		if err != nil {
			return nil, apperrors.Validation(field+" is invalid", map[string]any{"field": field})
		}
	}
	return &parsed, nil
}
