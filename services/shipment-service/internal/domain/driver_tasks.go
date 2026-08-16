package domain

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

const (
	OutboxEventTypeDriverTaskCreated   = "driver.task_created"
	OutboxEventTypeDriverTaskCompleted = "driver.task_completed"
	OutboxEventTypeDriverTaskExpired   = "driver.task_expired"
	OutboxEventTypeDriverTaskCancelled = "driver.task_cancelled"

	DriverTaskTypeRequestDelayReason          = "REQUEST_DELAY_REASON"
	DriverTaskTypeRequestStatusConfirmation   = "REQUEST_STATUS_CONFIRMATION"
	DriverTaskTypeRequestArrivalConfirmation  = "REQUEST_ARRIVAL_CONFIRMATION"
	DriverTaskTypeRequestDocumentAction       = "REQUEST_DOCUMENT_ACTION"
	DriverTaskTypeGeneralOperationalNotice    = "GENERAL_OPERATIONAL_NOTICE"

	DriverTaskStatusPending      = "PENDING"
	DriverTaskStatusDelivered    = "DELIVERED"
	DriverTaskStatusRead         = "READ"
	DriverTaskStatusAcknowledged = "ACKNOWLEDGED"
	DriverTaskStatusCompleted    = "COMPLETED"
	DriverTaskStatusExpired      = "EXPIRED"
	DriverTaskStatusCancelled    = "CANCELLED"

	DriverTaskPriorityNormal   = "NORMAL"
	DriverTaskPriorityHigh     = "HIGH"
	DriverTaskPriorityCritical = "CRITICAL"

	DriverTaskSourceSystem        = "SYSTEM"
	DriverTaskSourceControlTower  = "CONTROL_TOWER"
	DriverTaskSourceOperator      = "OPERATOR"

	DriverTaskCreatorSystem       = "SYSTEM"
	DriverTaskCreatorControlTower = "CONTROL_TOWER"
	DriverTaskCreatorOperator     = "OPERATOR"
)

var allowedDriverTaskTypes = map[string]struct{}{
	DriverTaskTypeRequestDelayReason:         {},
	DriverTaskTypeRequestStatusConfirmation:  {},
	DriverTaskTypeRequestArrivalConfirmation: {},
	DriverTaskTypeRequestDocumentAction:      {},
	DriverTaskTypeGeneralOperationalNotice:   {},
}

var allowedDelayReasons = map[string]struct{}{
	"TRAFFIC":              {},
	"VEHICLE_BREAKDOWN":    {},
	"LOADING_DELAY":        {},
	"UNLOADING_DELAY":      {},
	"ROUTE_BLOCKED":        {},
	"CUSTOMER_UNAVAILABLE": {},
	"OTHER":                {},
}

var driverTaskTransitions = map[string]map[string]struct{}{
	DriverTaskStatusPending: {
		DriverTaskStatusDelivered: {}, DriverTaskStatusRead: {}, DriverTaskStatusExpired: {}, DriverTaskStatusCancelled: {},
	},
	DriverTaskStatusDelivered: {
		DriverTaskStatusRead: {}, DriverTaskStatusAcknowledged: {}, DriverTaskStatusCompleted: {},
		DriverTaskStatusExpired: {}, DriverTaskStatusCancelled: {},
	},
	DriverTaskStatusRead: {
		DriverTaskStatusAcknowledged: {}, DriverTaskStatusCompleted: {},
		DriverTaskStatusExpired: {}, DriverTaskStatusCancelled: {},
	},
	DriverTaskStatusAcknowledged: {
		DriverTaskStatusCompleted: {}, DriverTaskStatusExpired: {}, DriverTaskStatusCancelled: {},
	},
}

type DriverTask struct {
	ID             uuid.UUID
	TenantID       uuid.UUID
	DriverID       uuid.UUID
	ShipmentID     *uuid.UUID
	TaskType       string
	Status         string
	Priority       string
	Title          string
	Payload        json.RawMessage
	CreatedAt      time.Time
	AvailableAt    time.Time
	ExpiresAt      *time.Time
	DeliveredAt    *time.Time
	ReadAt         *time.Time
	AcknowledgedAt *time.Time
	CompletedAt    *time.Time
	CancelledAt    *time.Time
	CreatedByType  string
	CreatedByID    *uuid.UUID
	Source         string
	CorrelationID  *string
	SourceEventID  *string
	IdempotencyKey *string
	Version        int
}

type DriverTaskResponse struct {
	ID             uuid.UUID
	TenantID       uuid.UUID
	TaskID         uuid.UUID
	DriverID       uuid.UUID
	ResponseType   string
	ResponseBody   json.RawMessage
	OccurredAt     *time.Time
	ReceivedAt     time.Time
	IdempotencyKey string
	CreatedAt      time.Time
}

type DriverDevice struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	DriverID         uuid.UUID
	Platform         string
	PushProvider     string
	PushTokenHash    string
	DeviceInstanceID string
	AppVersion       *string
	Locale           *string
	LastSeenAt       *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
	RevokedAt        *time.Time
}

type CreateDriverTaskInput struct {
	TenantID       uuid.UUID
	DriverID       uuid.UUID
	ShipmentID     *uuid.UUID
	TaskType       string
	Priority       string
	ExpiresAt      *time.Time
	Source         string
	SourceEventID  *string
	CorrelationID  *string
	IdempotencyKey string
	CreatedByType  string
	CreatedByID    *uuid.UUID
}

type DelayReasonResponse struct {
	Reason     string     `json:"reason"`
	Comment    *string    `json:"comment,omitempty"`
	OccurredAt *time.Time `json:"occurredAt,omitempty"`
}

type ListDriverTasksFilter struct {
	TenantID uuid.UUID
	DriverID uuid.UUID
	Status   *string
	Unread   bool
	Limit    int
	Offset   int
}

func TaskTitle(taskType string) string {
	switch taskType {
	case DriverTaskTypeRequestDelayReason:
		return "Please provide the reason for delay"
	case DriverTaskTypeRequestStatusConfirmation:
		return "Please confirm shipment status"
	case DriverTaskTypeRequestArrivalConfirmation:
		return "Please confirm arrival"
	case DriverTaskTypeRequestDocumentAction:
		return "Document action required"
	default:
		return "Operational notice"
	}
}

func ValidateCreateDriverTaskInput(in CreateDriverTaskInput) error {
	if in.TenantID == uuid.Nil || in.DriverID == uuid.Nil {
		return apperrors.Validation("tenant_id and driver_id are required", nil)
	}
	taskType := strings.TrimSpace(in.TaskType)
	if _, ok := allowedDriverTaskTypes[taskType]; !ok {
		return apperrors.Validation("unsupported task type", map[string]any{"field": "type", "value": taskType})
	}
	if in.ShipmentID == nil && taskType != DriverTaskTypeGeneralOperationalNotice {
		return apperrors.Validation("shipment_id is required for this task type", map[string]any{"field": "shipmentId"})
	}
	priority := strings.TrimSpace(in.Priority)
	if priority == "" {
		priority = DriverTaskPriorityNormal
	}
	switch priority {
	case DriverTaskPriorityNormal, DriverTaskPriorityHigh, DriverTaskPriorityCritical:
	default:
		return apperrors.Validation("unsupported priority", map[string]any{"field": "priority"})
	}
	source := strings.TrimSpace(strings.ToUpper(in.Source))
	switch source {
	case DriverTaskSourceSystem, DriverTaskSourceControlTower, DriverTaskSourceOperator:
	default:
		return apperrors.Validation("unsupported source", map[string]any{"field": "source"})
	}
	key := strings.TrimSpace(in.IdempotencyKey)
	if key == "" && (in.SourceEventID == nil || strings.TrimSpace(*in.SourceEventID) == "") {
		return apperrors.Validation("idempotency_key or source_event_id is required", nil)
	}
	if len(key) > 128 {
		return apperrors.Validation("idempotency_key must be at most 128 characters", map[string]any{"field": "idempotencyKey"})
	}
	if in.ExpiresAt != nil && !in.ExpiresAt.After(time.Now().UTC()) {
		return apperrors.Validation("expires_at must be in the future", map[string]any{"field": "expiresAt"})
	}
	return nil
}

func CanTransitionDriverTask(from, to string) bool {
	if from == to {
		return true
	}
	targets, ok := driverTaskTransitions[from]
	if !ok {
		return false
	}
	_, ok = targets[to]
	return ok
}

func IsDriverTaskTerminal(status string) bool {
	switch status {
	case DriverTaskStatusCompleted, DriverTaskStatusExpired, DriverTaskStatusCancelled:
		return true
	default:
		return false
	}
}

func CanDriverRespondToTask(task *DriverTask) bool {
	if task == nil || IsDriverTaskTerminal(task.Status) {
		return false
	}
	if task.ExpiresAt != nil && time.Now().UTC().After(*task.ExpiresAt) {
		return false
	}
	switch task.Status {
	case DriverTaskStatusPending, DriverTaskStatusDelivered, DriverTaskStatusRead, DriverTaskStatusAcknowledged:
		return true
	default:
		return false
	}
}

func ValidateDelayReasonResponse(body DelayReasonResponse) error {
	reason := strings.TrimSpace(strings.ToUpper(body.Reason))
	if reason == "" {
		return apperrors.Validation("reason is required", map[string]any{"field": "reason"})
	}
	if _, ok := allowedDelayReasons[reason]; !ok {
		return apperrors.Validation("unsupported delay reason", map[string]any{"field": "reason", "value": reason})
	}
	if body.Comment != nil && len(strings.TrimSpace(*body.Comment)) > 4000 {
		return apperrors.Validation("comment must be at most 4000 characters", map[string]any{"field": "comment"})
	}
	if body.OccurredAt != nil && body.OccurredAt.After(time.Now().UTC().Add(5*time.Minute)) {
		return apperrors.Validation("occurred_at cannot be more than 5 minutes in the future", map[string]any{"field": "occurredAt"})
	}
	return nil
}

func SanitizeTaskComment(comment *string) *string {
	if comment == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*comment)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func BuildDriverTaskOutboxPayload(task DriverTask, eventType string, shipmentVersion int, response *DriverTaskResponse) ([]byte, error) {
	payload := map[string]any{
		"eventId":       task.ID.String(),
		"eventType":     eventType,
		"schemaVersion": OutboxSchemaVersion,
		"occurredAt":    time.Now().UTC().Format(time.RFC3339Nano),
		"tenantId":      task.TenantID.String(),
		"driverId":      task.DriverID.String(),
		"taskId":        task.ID.String(),
		"taskType":      task.TaskType,
		"status":        task.Status,
		"source":        task.Source,
		"aggregate": map[string]any{
			"type":    OutboxAggregateTypeShipment,
			"id":      coalesceShipmentID(task.ShipmentID),
			"version": shipmentVersion,
		},
	}
	if task.ShipmentID != nil {
		payload["shipmentId"] = task.ShipmentID.String()
	}
	if task.CorrelationID != nil {
		payload["correlationId"] = *task.CorrelationID
	}
	if task.SourceEventID != nil {
		payload["sourceEventId"] = *task.SourceEventID
	}
	if task.IdempotencyKey != nil {
		payload["idempotencyKey"] = *task.IdempotencyKey
	}
	if response != nil {
		payload["response"] = json.RawMessage(response.ResponseBody)
		payload["responseId"] = response.ID.String()
	}
	return json.Marshal(payload)
}

func coalesceShipmentID(id *uuid.UUID) string {
	if id == nil {
		return uuid.Nil.String()
	}
	return id.String()
}
