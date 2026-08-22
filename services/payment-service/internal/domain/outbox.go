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

	PaymentEventObligationPaid         = "payment_obligation.paid"
	PaymentEventObligationPaidSnapshot = "payment_obligation.paid_snapshot.v1"
	AggregatePaymentObligation         = "PAYMENT_OBLIGATION"

	PaymentOutboxSchemaVersion        = 1
	PaymentPaidSnapshotSchemaVersion  = 1
	EntryKindPaidAmountSnapshot       = "PAID_AMOUNT_SNAPSHOT"
	TaxBasisWithVAT                   = "WITH_VAT"
)

var ErrOutboxPublishStateConflict = errors.New("outbox publish state conflict")

type PaymentOutboxStatus string

type PaymentOutboxEvent struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	AggregateType    string
	AggregateID      uuid.UUID
	AggregateVersion int64
	EventType        string
	SchemaVersion    int
	Payload          json.RawMessage
	Status           PaymentOutboxStatus
	Attempts         int
	AvailableAt      time.Time
	LockedAt         *time.Time
	LockedBy         *string
	PublishedAt      *time.Time
	LastErrorCode    *string
	CreatedAt        time.Time
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

type ObligationPaidSnapshotOutboxPayload struct {
	EventID          string  `json:"event_id"`
	TenantID         string  `json:"tenant_id"`
	ObligationID     string  `json:"obligation_id"`
	RegisterID       string  `json:"register_id"`
	SourceRevision   int64   `json:"source_revision"`
	PaidAmount       string  `json:"paid_amount"`
	ObligationAmount string  `json:"obligation_amount"`
	CurrencyCode     string  `json:"currency_code"`
	TaxBasis         string  `json:"tax_basis"`
	EntryKind        string  `json:"entry_kind"`
	OccurredAt       string  `json:"occurred_at"`
	SourceService    string  `json:"source_service"`
	SourceType       string  `json:"source_type"`
	SourceID         string  `json:"source_id"`
	AmountAvailability string `json:"amount_availability"`
}

func BuildObligationPaidSnapshotOutboxPayload(eventID uuid.UUID, obligation *PaymentObligation, occurredAt time.Time) (json.RawMessage, error) {
	registerID := obligation.SourceID.String()
	payload := ObligationPaidSnapshotOutboxPayload{
		EventID:            eventID.String(),
		TenantID:           obligation.TenantID.String(),
		ObligationID:       obligation.ID.String(),
		RegisterID:         registerID,
		SourceRevision:     int64(obligation.Version),
		PaidAmount:         obligation.PaidAmount.StringFixed(MoneyScale),
		ObligationAmount:   obligation.OriginalAmount.StringFixed(MoneyScale),
		CurrencyCode:       obligation.CurrencyCode,
		TaxBasis:           TaxBasisWithVAT,
		EntryKind:          EntryKindPaidAmountSnapshot,
		OccurredAt:         occurredAt.UTC().Format(time.RFC3339Nano),
		SourceService:      "payment-service",
		SourceType:         AggregatePaymentObligation,
		SourceID:           obligation.ID.String(),
		AmountAvailability: "AVAILABLE",
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func ParseObligationPaidSnapshotOutboxPayload(raw json.RawMessage) (ObligationPaidSnapshotOutboxPayload, error) {
	var payload ObligationPaidSnapshotOutboxPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ObligationPaidSnapshotOutboxPayload{}, err
	}
	return payload, nil
}
