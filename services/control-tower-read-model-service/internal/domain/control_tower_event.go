package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	ControlTowerEventSourceDriver = "driver"

	DriverTriggerDelayReported     = "driver_delay_reported"
	DriverTriggerProblemReported   = "driver_problem_reported"
	DriverTriggerTrackingRestored  = "driver_tracking_restored"
)

type ControlTowerEvent struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	Type        string
	Source      string
	SubjectType string
	SubjectID   string
	ShipmentID  uuid.UUID
	OccurredAt  time.Time
	Severity    string
	Actor       string
	Attributes  map[string]any
}

type DriverDomainEventEnvelope struct {
	EventID       string         `json:"eventId"`
	EventType     string         `json:"eventType"`
	SchemaVersion int            `json:"schemaVersion"`
	OccurredAt    time.Time      `json:"occurredAt"`
	TenantID      string         `json:"tenantId"`
	ShipmentID    string         `json:"shipmentId"`
	DriverID      string         `json:"driverId,omitempty"`
	VehicleID     string         `json:"vehicleId,omitempty"`
	ActorID       string         `json:"actorId,omitempty"`
	CorrelationID *string        `json:"correlationId,omitempty"`
	RequestID     *string        `json:"requestId,omitempty"`
	Source        string         `json:"source"`
	SourceEventID string         `json:"sourceEventId"`
	Severity      string         `json:"severity,omitempty"`
	ReasonCode    string         `json:"reasonCode,omitempty"`
	ReasonText    string         `json:"reasonText,omitempty"`
	ETA           *time.Time     `json:"eta,omitempty"`
	Latitude      *float64       `json:"latitude,omitempty"`
	Longitude     *float64       `json:"longitude,omitempty"`
	Accuracy      *float64       `json:"accuracy,omitempty"`
	DocumentID    string         `json:"documentId,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}
