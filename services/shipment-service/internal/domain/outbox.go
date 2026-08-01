package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

var ErrOutboxPublishStateConflict = errors.New("outbox publish state conflict")

const (
	OutboxAggregateTypeShipment = "SHIPMENT"

	OutboxEventTypeCreated            = "shipment.created"
	OutboxEventTypeStatusChanged      = "shipment.status.changed"
	OutboxEventTypeCancelled          = "shipment.cancelled"
	OutboxEventTypeReadyForBilling    = "shipment.ready_for_billing"
	OutboxEventTypeDocumentsCompleted = "shipment.documents_completed"
	OutboxEventTypeFinanciallyClosed  = "shipment.financially_closed"

	OutboxSchemaVersion = 1
)

type OutboxStatus string

const (
	OutboxStatusPending   OutboxStatus = "PENDING"
	OutboxStatusPublished OutboxStatus = "PUBLISHED"
	OutboxStatusFailed    OutboxStatus = "FAILED"
)

type ShipmentOutboxEvent struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	AggregateType    string
	AggregateID      uuid.UUID
	AggregateVersion int

	EventType     string
	SchemaVersion int
	SourceEventID uuid.UUID

	Payload json.RawMessage
	Headers json.RawMessage

	Status        OutboxStatus
	Attempts      int
	AvailableAt   time.Time
	LockedAt      *time.Time
	LockedBy      *string
	PublishedAt   *time.Time
	LastErrorCode *string
	CreatedAt     time.Time
}

type ShipmentStatusEventEnvelope struct {
	EventID       string                  `json:"eventId"`
	EventType     string                  `json:"eventType"`
	SchemaVersion int                     `json:"schemaVersion"`
	OccurredAt    time.Time               `json:"occurredAt"`
	TenantID      string                  `json:"tenantId"`
	Aggregate     ShipmentStatusAggregate `json:"aggregate"`
	SourceEventID string                  `json:"sourceEventId"`
	CorrelationID *string                 `json:"correlationId,omitempty"`
	Data          ShipmentStatusEventData `json:"data"`
}

type ShipmentStatusAggregate struct {
	Type    string `json:"type"`
	ID      string `json:"id"`
	Version int    `json:"version"`
}

type ShipmentStatusEventData struct {
	FromStatus *string `json:"fromStatus"`
	ToStatus   string  `json:"toStatus"`
	ReasonCode *string `json:"reasonCode"`
	ActorType  string  `json:"actorType"`
}

func MapOutboxEventType(history ShipmentStatusHistory) string {
	if history.FromStatus == nil {
		return OutboxEventTypeCreated
	}
	switch history.ToStatus {
	case ShipmentStatusCancelled:
		return OutboxEventTypeCancelled
	case ShipmentStatusReadyForBilling:
		return OutboxEventTypeReadyForBilling
	case ShipmentStatusDocumentsCompleted:
		return OutboxEventTypeDocumentsCompleted
	case ShipmentStatusFinanciallyClosed:
		return OutboxEventTypeFinanciallyClosed
	default:
		return OutboxEventTypeStatusChanged
	}
}

func BuildOutboxEventFromStatusHistory(history ShipmentStatusHistory) (ShipmentOutboxEvent, error) {
	if history.ID == uuid.Nil {
		return ShipmentOutboxEvent{}, apperrors.Internal("status history id is required for outbox", nil)
	}
	if history.TenantID == uuid.Nil || history.ShipmentID == uuid.Nil {
		return ShipmentOutboxEvent{}, apperrors.Internal("status history tenant and shipment are required", nil)
	}
	if history.ShipmentVersion <= 0 {
		return ShipmentOutboxEvent{}, apperrors.Internal("status history shipment version must be positive", nil)
	}
	if strings.TrimSpace(history.ToStatus) == "" {
		return ShipmentOutboxEvent{}, apperrors.Internal("status history to_status is required", nil)
	}
	if history.OccurredAt.IsZero() {
		return ShipmentOutboxEvent{}, apperrors.Internal("status history occurred_at is required", nil)
	}

	eventType := MapOutboxEventType(history)
	outboxID := uuid.New()

	envelope := ShipmentStatusEventEnvelope{
		EventID:       outboxID.String(),
		EventType:     eventType,
		SchemaVersion: OutboxSchemaVersion,
		OccurredAt:    history.OccurredAt.UTC(),
		TenantID:      history.TenantID.String(),
		Aggregate: ShipmentStatusAggregate{
			Type:    OutboxAggregateTypeShipment,
			ID:      history.ShipmentID.String(),
			Version: history.ShipmentVersion,
		},
		SourceEventID: history.ID.String(),
		CorrelationID: history.CorrelationID,
		Data: ShipmentStatusEventData{
			FromStatus: history.FromStatus,
			ToStatus:   history.ToStatus,
			ReasonCode: history.ReasonCode,
			ActorType:  string(history.ActorType),
		},
	}

	payload, err := json.Marshal(envelope)
	if err != nil {
		return ShipmentOutboxEvent{}, apperrors.Internal("marshal outbox envelope", err)
	}
	if err := validateOutboxPayload(payload); err != nil {
		return ShipmentOutboxEvent{}, err
	}

	headers, err := json.Marshal(map[string]string{
		"contentType": "application/json",
		"actorType":   string(history.ActorType),
	})
	if err != nil {
		return ShipmentOutboxEvent{}, apperrors.Internal("marshal outbox headers", err)
	}

	now := time.Now().UTC()
	return ShipmentOutboxEvent{
		ID:               outboxID,
		TenantID:         history.TenantID,
		AggregateType:    OutboxAggregateTypeShipment,
		AggregateID:      history.ShipmentID,
		AggregateVersion: history.ShipmentVersion,
		EventType:        eventType,
		SchemaVersion:    OutboxSchemaVersion,
		SourceEventID:    history.ID,
		Payload:          payload,
		Headers:          headers,
		Status:           OutboxStatusPending,
		Attempts:         0,
		AvailableAt:      now,
		CreatedAt:        now,
	}, nil
}

func ValidateOutboxAgainstHistory(outbox ShipmentOutboxEvent, history ShipmentStatusHistory) error {
	if outbox.TenantID != history.TenantID {
		return apperrors.Internal("outbox tenant_id must match history tenant_id", nil)
	}
	if outbox.AggregateID != history.ShipmentID {
		return apperrors.Internal("outbox aggregate_id must match history shipment_id", nil)
	}
	if outbox.AggregateVersion != history.ShipmentVersion {
		return apperrors.Internal("outbox aggregate_version must match history shipment_version", nil)
	}
	if outbox.SourceEventID != history.ID {
		return apperrors.Internal("outbox source_event_id must match history id", nil)
	}
	if outbox.SchemaVersion <= 0 {
		return apperrors.Internal("outbox schema_version must be positive", nil)
	}
	if strings.TrimSpace(outbox.EventType) == "" {
		return apperrors.Internal("outbox event_type is required", nil)
	}
	if !isKnownOutboxEventType(outbox.EventType) {
		return apperrors.Internal("outbox event_type is unknown", nil)
	}
	if len(outbox.Payload) == 0 {
		return apperrors.Internal("outbox payload is required", nil)
	}
	if err := validateOutboxPayload(outbox.Payload); err != nil {
		return err
	}
	return nil
}

func isKnownOutboxEventType(eventType string) bool {
	switch eventType {
	case OutboxEventTypeCreated,
		OutboxEventTypeStatusChanged,
		OutboxEventTypeCancelled,
		OutboxEventTypeReadyForBilling,
		OutboxEventTypeDocumentsCompleted,
		OutboxEventTypeFinanciallyClosed:
		return true
	default:
		return false
	}
}

func validateOutboxPayload(payload []byte) error {
	lower := strings.ToLower(string(payload))
	for _, forbidden := range []string{"jwt", "email", "phone", "password", "authorization", "bearer"} {
		if strings.Contains(lower, forbidden) {
			return apperrors.Internal(fmt.Sprintf("outbox payload must not contain %s", forbidden), nil)
		}
	}
	return nil
}
