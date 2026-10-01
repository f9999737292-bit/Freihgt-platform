package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

// SuccessorRevisionCommand replaces the remaining route of one TransportExecution.
// Stop and action rows stay on the execution. Completed history is linked, not moved.
// The caller does not choose the current stop.
type SuccessorRevisionCommand struct {
	ExecutionID             uuid.UUID
	ExpectedRevisionID      uuid.UUID
	OperatingTenantID       uuid.UUID
	IdempotencyKey          string
	ReasonCode              string
	OccurredAt              time.Time
	ActorKind               string
	ActorID                 uuid.UUID
	SourceRoutePlanID       uuid.UUID
	SourceRoutePlanVersion  int
	SourceActivationID      uuid.UUID
	SourceActivationVersion int
	PlanningMode            string
	EvaluationFingerprint   string
	Stops                   []SuccessorStopInput
	Actions                 []SuccessorActionInput
}

// SuccessorStopInput is one future stop. Ordinals are assigned by the server.
type SuccessorStopInput struct {
	StopRole               string
	PointKind              string
	LocationID             *uuid.UUID
	Latitude               float64
	Longitude              float64
	PlannedArrival         *time.Time
	PlannedDeparture       *time.Time
	ServiceDurationSeconds *int
}

// SuccessorActionInput attaches a new action to a replacement stop by index.
// Shipment tenant is resolved from the execution participant row.
type SuccessorActionInput struct {
	StopIndex  int
	ActionType string
	ShipmentID uuid.UUID
	CargoID    uuid.UUID
}

// SuccessorRevisionResult is the stored idempotent outcome.
type SuccessorRevisionResult struct {
	CommandID          uuid.UUID `json:"command_id"`
	ExecutionID        uuid.UUID `json:"execution_id"`
	PreviousRevisionID uuid.UUID `json:"previous_revision_id"`
	RevisionID         uuid.UUID `json:"revision_id"`
	CurrentStopID      uuid.UUID `json:"current_stop_id"`
	Replayed           bool      `json:"replayed"`
}

func (c SuccessorRevisionCommand) Validate() error {
	if c.ExecutionID == uuid.Nil || c.ExpectedRevisionID == uuid.Nil || c.OperatingTenantID == uuid.Nil {
		return apperrors.Validation("execution identity is required", map[string]any{"field": "execution_id"})
	}
	key := strings.TrimSpace(c.IdempotencyKey)
	if key == "" || len(key) > 128 {
		return apperrors.Validation("idempotency_key is required", map[string]any{"field": "idempotency_key"})
	}
	reason := strings.TrimSpace(c.ReasonCode)
	if reason == "" || len(reason) > 64 {
		return apperrors.Validation("reason_code is required", map[string]any{"field": "reason_code"})
	}
	if c.OccurredAt.IsZero() {
		return apperrors.Validation("occurred_at is required", map[string]any{"field": "occurred_at"})
	}
	switch c.ActorKind {
	case ActorKindOperator:
		if c.ActorID == uuid.Nil {
			return apperrors.Validation("actor_id is required", map[string]any{"field": "actor_id"})
		}
	case ActorKindSystem:
	default:
		return apperrors.Validation("actor_kind is invalid", map[string]any{"field": "actor_kind"})
	}
	if c.SourceRoutePlanID == uuid.Nil || c.SourceActivationID == uuid.Nil {
		return apperrors.Validation("source identity is required", map[string]any{"field": "source_route_plan_id"})
	}
	if c.SourceRoutePlanVersion < 1 || c.SourceActivationVersion < 1 {
		return apperrors.Validation("source version is required", map[string]any{"field": "source_route_plan_version"})
	}
	fingerprint := strings.TrimSpace(c.EvaluationFingerprint)
	if fingerprint == "" || len(fingerprint) > 512 {
		return apperrors.Validation("evaluation_fingerprint is required", map[string]any{"field": "evaluation_fingerprint"})
	}
	switch c.PlanningMode {
	case PlanningModeDepotStart, PlanningModeCurrentTrip:
	default:
		return apperrors.Validation("planning_mode is invalid", map[string]any{"field": "planning_mode"})
	}
	if len(c.Stops) == 0 {
		return ExecutionCommandError(ReasonNoRemainingRoute, false)
	}
	for i, stop := range c.Stops {
		switch stop.StopRole {
		case StopRoleStart, StopRoleCargo, StopRoleEnd:
		default:
			return apperrors.Validation("stop_role is invalid", map[string]any{"field": "stops", "index": i})
		}
		switch stop.PointKind {
		case PointKindCanonicalLocation:
			if stop.LocationID == nil || *stop.LocationID == uuid.Nil {
				return apperrors.Validation("location_id is required", map[string]any{"field": "location_id", "index": i})
			}
		case PointKindPositionAnchor:
		default:
			return apperrors.Validation("point_kind is invalid", map[string]any{"field": "point_kind", "index": i})
		}
		if stop.ServiceDurationSeconds != nil && *stop.ServiceDurationSeconds < 0 {
			return apperrors.Validation("service_duration_seconds is invalid", map[string]any{"field": "service_duration_seconds", "index": i})
		}
	}
	for i, action := range c.Actions {
		if action.StopIndex < 0 || action.StopIndex >= len(c.Stops) {
			return apperrors.Validation("stop_index is invalid", map[string]any{"field": "stop_index", "index": i})
		}
		if action.ActionType != ActionTypePickup && action.ActionType != ActionTypeDelivery {
			return apperrors.Validation("action_type is invalid", map[string]any{"field": "action_type", "index": i})
		}
		if action.ShipmentID == uuid.Nil || action.CargoID == uuid.Nil {
			return apperrors.Validation("shipment identity is required", map[string]any{"field": "shipment_id", "index": i})
		}
	}
	return nil
}

func SuccessorDigest(cmd SuccessorRevisionCommand) (string, error) {
	stops := make([]successorStopDigest, len(cmd.Stops))
	for i, stop := range cmd.Stops {
		stops[i] = successorStopDigest{
			StopRole:               stop.StopRole,
			PointKind:              stop.PointKind,
			LocationID:             commandUUID(derefUUID(stop.LocationID)),
			Latitude:               stop.Latitude,
			Longitude:              stop.Longitude,
			PlannedArrival:         formatDigestTime(stop.PlannedArrival),
			PlannedDeparture:       formatDigestTime(stop.PlannedDeparture),
			ServiceDurationSeconds: stop.ServiceDurationSeconds,
		}
	}
	actions := make([]successorActionDigest, len(cmd.Actions))
	for i, action := range cmd.Actions {
		actions[i] = successorActionDigest{
			StopIndex:  action.StopIndex,
			ActionType: action.ActionType,
			ShipmentID: action.ShipmentID.String(),
			CargoID:    action.CargoID.String(),
		}
	}
	body := successorDigestBody{
		ExecutionID:             cmd.ExecutionID.String(),
		ExpectedRevisionID:      cmd.ExpectedRevisionID.String(),
		OperatingTenantID:       cmd.OperatingTenantID.String(),
		ReasonCode:              strings.ToUpper(strings.TrimSpace(cmd.ReasonCode)),
		OccurredAt:              cmd.OccurredAt.UTC().Format(time.RFC3339Nano),
		ActorKind:               cmd.ActorKind,
		ActorID:                 commandUUID(cmd.ActorID),
		SourceRoutePlanID:       cmd.SourceRoutePlanID.String(),
		SourceRoutePlanVersion:  cmd.SourceRoutePlanVersion,
		SourceActivationID:      cmd.SourceActivationID.String(),
		SourceActivationVersion: cmd.SourceActivationVersion,
		PlanningMode:            cmd.PlanningMode,
		EvaluationFingerprint:   strings.TrimSpace(cmd.EvaluationFingerprint),
		Stops:                   stops,
		Actions:                 actions,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func derefUUID(id *uuid.UUID) uuid.UUID {
	if id == nil {
		return uuid.Nil
	}
	return *id
}

func formatDigestTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

type successorDigestBody struct {
	ExecutionID             string                  `json:"execution_id"`
	ExpectedRevisionID      string                  `json:"expected_revision_id"`
	OperatingTenantID       string                  `json:"operating_tenant_id"`
	ReasonCode              string                  `json:"reason_code"`
	OccurredAt              string                  `json:"occurred_at"`
	ActorKind               string                  `json:"actor_kind"`
	ActorID                 string                  `json:"actor_id"`
	SourceRoutePlanID       string                  `json:"source_route_plan_id"`
	SourceRoutePlanVersion  int                     `json:"source_route_plan_version"`
	SourceActivationID      string                  `json:"source_activation_id"`
	SourceActivationVersion int                     `json:"source_activation_version"`
	PlanningMode            string                  `json:"planning_mode"`
	EvaluationFingerprint   string                  `json:"evaluation_fingerprint"`
	Stops                   []successorStopDigest   `json:"stops"`
	Actions                 []successorActionDigest `json:"actions"`
}

type successorStopDigest struct {
	StopRole               string  `json:"stop_role"`
	PointKind              string  `json:"point_kind"`
	LocationID             string  `json:"location_id"`
	Latitude               float64 `json:"latitude"`
	Longitude              float64 `json:"longitude"`
	PlannedArrival         string  `json:"planned_arrival"`
	PlannedDeparture       string  `json:"planned_departure"`
	ServiceDurationSeconds *int    `json:"service_duration_seconds"`
}

type successorActionDigest struct {
	StopIndex  int    `json:"stop_index"`
	ActionType string `json:"action_type"`
	ShipmentID string `json:"shipment_id"`
	CargoID    string `json:"cargo_id"`
}
