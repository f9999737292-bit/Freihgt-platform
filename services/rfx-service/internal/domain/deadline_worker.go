package domain

import (
	"time"

	"github.com/google/uuid"
)

const AuditActorTypeSystem = "SYSTEM"

// ExpiredResponseOpenEvent is a worker scan candidate for automatic response closure.
type ExpiredResponseOpenEvent struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	Status           string
	ResponseDeadline *time.Time
}
