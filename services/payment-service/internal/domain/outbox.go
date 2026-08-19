package domain

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

const (
	PaymentOutboxStatusPending   = "PENDING"
	PaymentOutboxStatusPublished = "PUBLISHED"
	PaymentOutboxStatusFailed    = "FAILED"

	PaymentEventObligationPaid = "payment_obligation.paid"
	AggregatePaymentObligation   = "PAYMENT_OBLIGATION"

	PaymentOutboxSchemaVersion = 1
)

var ErrOutboxPublishStateConflict = errors.New("outbox publish state conflict")

type PaymentOutboxStatus string

type PaymentOutboxEvent struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	AggregateType string
	AggregateID   uuid.UUID
	EventType     string
	SchemaVersion int
	Payload       json.RawMessage
	Status        PaymentOutboxStatus
	Attempts      int
	AvailableAt   time.Time
	LockedAt      *time.Time
	LockedBy      *string
	PublishedAt   *time.Time
	LastErrorCode *string
	CreatedAt     time.Time
}

type ObligationPaidOutboxPayload struct {
	TenantID    string `json:"tenant_id"`
	ObligationID string `json:"obligation_id"`
	RegisterID  string `json:"register_id"`
}

func BuildObligationPaidOutboxPayload(tenantID, obligationID, registerID uuid.UUID) (json.RawMessage, error) {
	payload := ObligationPaidOutboxPayload{
		TenantID:     tenantID.String(),
		ObligationID: obligationID.String(),
		RegisterID:   registerID.String(),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func ParseObligationPaidOutboxPayload(raw json.RawMessage) (ObligationPaidOutboxPayload, error) {
	var payload ObligationPaidOutboxPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ObligationPaidOutboxPayload{}, err
	}
	return payload, nil
}
