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

const (
	CommandArriveStop           = "ARRIVE_STOP"
	CommandStartStopService     = "START_STOP_SERVICE"
	CommandCompleteStop         = "COMPLETE_STOP"
	CommandSkipStop             = "SKIP_STOP"
	CommandCancelRemainingStops = "CANCEL_REMAINING_STOPS"
	CommandCancelInServiceStop  = "CANCEL_IN_SERVICE_STOP"
	CommandConfirmPickup        = "CONFIRM_PICKUP"
	CommandConfirmDelivery      = "CONFIRM_DELIVERY"
	CommandFailAction           = "FAIL_ACTION"
	CommandDepartedPickup       = "DEPARTED_PICKUP"

	ActorKindDriver   = "DRIVER"
	ActorKindOperator = "OPERATOR"
	ActorKindSystem   = "SYSTEM"

	StopStatusReasonPartial = "PARTIAL"

	OutboxAggregateTypeTransportExecution = "TRANSPORT_EXECUTION"

	EventRouteStopCurrent            = "shipment.route_stop.current"
	EventRouteStopArrived            = "shipment.route_stop.arrived"
	EventRouteStopServiceStarted     = "shipment.route_stop.service_started"
	EventRouteStopCompleted          = "shipment.route_stop.completed"
	EventRouteStopSequenceOverridden = "shipment.route_stop.sequence_overridden"

	ReasonStopNotCurrent           = "STOP_NOT_CURRENT"
	ReasonCommandBodyConflict      = "COMMAND_BODY_CONFLICT"
	ReasonStaleRevision            = "STALE_REVISION"
	ReasonVersionConflict          = "VERSION_CONFLICT"
	ReasonStopTransitionDenied     = "STOP_TRANSITION_DENIED"
	ReasonActionTransitionDenied   = "ACTION_TRANSITION_DENIED"
	ReasonActionPending            = "ACTION_PENDING"
	ReasonReasonRequired           = "REASON_REQUIRED"
	ReasonReasonUnknown            = "REASON_UNKNOWN"
	ReasonActorDenied              = "ACTOR_DENIED"
	ReasonUnassignedDriver         = "UNASSIGNED_DRIVER"
	ReasonWrongDriver              = "WRONG_DRIVER"
	ReasonDriverExecutionAmbiguous = "DRIVER_EXECUTION_AMBIGUOUS"
	ReasonNotParticipant           = "NOT_PARTICIPANT"
	ReasonTenantDenied             = "TENANT_DENIED"
	ReasonTerminalStop             = "TERMINAL_STOP"
	ReasonTerminalAction           = "TERMINAL_ACTION"
)

func IsExecutionKafkaEventType(eventType string) bool {
	switch strings.TrimSpace(eventType) {
	case EventRouteStopCurrent, EventRouteStopArrived, EventRouteStopServiceStarted, EventRouteStopCompleted, EventRouteStopSequenceOverridden:
		return true
	default:
		return false
	}
}

// ExecutionCommand is the in-process stop and action command.
// DEPARTED_PICKUP reuses the existing single-leg LOADED to IN_TRANSIT semantic.
// It is not a stop status and it is not a DepartStop API.
type ExecutionCommand struct {
	Name                string
	ExecutionID         uuid.UUID
	RevisionID          uuid.UUID
	StopID              uuid.UUID
	ActionID            uuid.UUID
	ShipmentID          uuid.UUID
	ShipmentTenantID    uuid.UUID
	IdempotencyKey      string
	OccurredAt          time.Time
	ExpectedStopVersion int
	ReasonCode          string
	ActorKind           string
	ActorID             uuid.UUID
	OperatingTenantID   uuid.UUID
}

type ExecutionCommandResult struct {
	CommandID      uuid.UUID  `json:"command_id"`
	ExecutionID    uuid.UUID  `json:"execution_id"`
	RevisionID     uuid.UUID  `json:"revision_id"`
	StopID         *uuid.UUID `json:"stop_id,omitempty"`
	ActionID       *uuid.UUID `json:"action_id,omitempty"`
	StopStatus     string     `json:"stop_status,omitempty"`
	ActionStatus   string     `json:"action_status,omitempty"`
	EvidenceID     *uuid.UUID `json:"evidence_id,omitempty"`
	ShipmentStatus string     `json:"shipment_status,omitempty"`
	Replayed       bool       `json:"replayed"`
}

func (c ExecutionCommand) Validate() error {
	name := strings.TrimSpace(c.Name)
	switch name {
	case CommandArriveStop, CommandStartStopService, CommandCompleteStop, CommandSkipStop,
		CommandCancelRemainingStops, CommandCancelInServiceStop, CommandConfirmPickup,
		CommandConfirmDelivery, CommandFailAction, CommandDepartedPickup:
	default:
		return apperrors.Validation("command name is invalid", map[string]any{"field": "name"})
	}
	if c.ExecutionID == uuid.Nil || c.RevisionID == uuid.Nil || c.OperatingTenantID == uuid.Nil {
		return apperrors.Validation("execution identity is required", map[string]any{"field": "execution_id"})
	}
	key := strings.TrimSpace(c.IdempotencyKey)
	if key == "" || len(key) > 128 {
		return apperrors.Validation("idempotency_key is required", map[string]any{"field": "idempotency_key"})
	}
	if c.OccurredAt.IsZero() {
		return apperrors.Validation("occurred_at is required", map[string]any{"field": "occurred_at"})
	}
	switch c.ActorKind {
	case ActorKindDriver, ActorKindOperator:
		if c.ActorID == uuid.Nil {
			return apperrors.Validation("actor_id is required", map[string]any{"field": "actor_id"})
		}
	case ActorKindSystem:
	default:
		return apperrors.Validation("actor_kind is invalid", map[string]any{"field": "actor_kind"})
	}
	if requiresStop(name) && c.StopID == uuid.Nil {
		return apperrors.Validation("stop_id is required", map[string]any{"field": "stop_id"})
	}
	if requiresAction(name) && c.ActionID == uuid.Nil {
		return apperrors.Validation("action_id is required", map[string]any{"field": "action_id"})
	}
	if name == CommandDepartedPickup && (c.ShipmentID == uuid.Nil || c.ShipmentTenantID == uuid.Nil) {
		return apperrors.Validation("shipment identity is required", map[string]any{"field": "shipment_id"})
	}
	if requiresStop(name) && c.ExpectedStopVersion < 1 {
		return apperrors.Validation("expected_stop_version is required", map[string]any{"field": "expected_stop_version"})
	}
	return nil
}

func requiresStop(name string) bool {
	switch name {
	case CommandArriveStop, CommandStartStopService, CommandCompleteStop, CommandSkipStop,
		CommandCancelInServiceStop, CommandConfirmPickup, CommandConfirmDelivery, CommandFailAction:
		return true
	default:
		return false
	}
}

func requiresAction(name string) bool {
	switch name {
	case CommandConfirmPickup, CommandConfirmDelivery, CommandFailAction:
		return true
	default:
		return false
	}
}

func CommandDigest(cmd ExecutionCommand) (string, error) {
	body := struct {
		Name                string `json:"name"`
		ExecutionID         string `json:"execution_id"`
		RevisionID          string `json:"revision_id"`
		StopID              string `json:"stop_id"`
		ActionID            string `json:"action_id"`
		ShipmentID          string `json:"shipment_id"`
		ShipmentTenantID    string `json:"shipment_tenant_id"`
		OccurredAt          string `json:"occurred_at"`
		ExpectedStopVersion int    `json:"expected_stop_version"`
		ReasonCode          string `json:"reason_code"`
		ActorKind           string `json:"actor_kind"`
		ActorID             string `json:"actor_id"`
		OperatingTenantID   string `json:"operating_tenant_id"`
	}{
		Name:                strings.TrimSpace(cmd.Name),
		ExecutionID:         cmd.ExecutionID.String(),
		RevisionID:          cmd.RevisionID.String(),
		StopID:              commandUUID(cmd.StopID),
		ActionID:            commandUUID(cmd.ActionID),
		ShipmentID:          commandUUID(cmd.ShipmentID),
		ShipmentTenantID:    commandUUID(cmd.ShipmentTenantID),
		OccurredAt:          cmd.OccurredAt.UTC().Format(time.RFC3339Nano),
		ExpectedStopVersion: cmd.ExpectedStopVersion,
		ReasonCode:          strings.TrimSpace(strings.ToUpper(cmd.ReasonCode)),
		ActorKind:           cmd.ActorKind,
		ActorID:             commandUUID(cmd.ActorID),
		OperatingTenantID:   cmd.OperatingTenantID.String(),
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func commandUUID(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}
	return id.String()
}

func ExecutionCommandError(reason string, forbidden bool) *apperrors.AppError {
	if forbidden {
		return &apperrors.AppError{
			Code:    apperrors.CodeForbidden,
			Message: reason,
			Details: map[string]any{"reason": reason},
		}
	}
	return apperrors.Conflict(reason, map[string]any{"reason": reason})
}

func KnownExecutionReason(reason string) bool {
	return IsDriverExceptionCategory(reason)
}
