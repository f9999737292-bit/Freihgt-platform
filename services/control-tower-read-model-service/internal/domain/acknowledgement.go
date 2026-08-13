package domain

import (
	"time"

	"github.com/google/uuid"
)

type CriticalEventAcknowledgement struct {
	TenantID              uuid.UUID
	EventID               string
	ShipmentID            uuid.UUID
	EventType             string
	Source                string
	OccurredAt            time.Time
	AcknowledgedAt        time.Time
	AcknowledgedByUserID  uuid.UUID
}

type AcknowledgeCriticalEventInput struct {
	TenantID             uuid.UUID
	UserID               uuid.UUID
	EventID              string
	ShipmentID           uuid.UUID
	EventType            string
	Source               string
	OccurredAt           time.Time
}
