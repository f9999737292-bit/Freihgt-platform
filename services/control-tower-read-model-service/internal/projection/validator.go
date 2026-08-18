package projection

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
)

type envelope struct {
	EventID       string   `json:"eventId"`
	EventType     string   `json:"eventType"`
	SchemaVersion int      `json:"schemaVersion"`
	OccurredAt    string   `json:"occurredAt"`
	TenantID      string   `json:"tenantId"`
	Aggregate     aggJSON  `json:"aggregate"`
	SourceEventID string   `json:"sourceEventId"`
	CorrelationID *string  `json:"correlationId"`
	Data          dataJSON `json:"data"`
}

type aggJSON struct {
	Type    string `json:"type"`
	ID      string `json:"id"`
	Version int    `json:"version"`
}

type dataJSON struct {
	FromStatus        *string `json:"fromStatus"`
	ToStatus          string  `json:"toStatus"`
	ReasonCode        *string `json:"reasonCode"`
	ActorType         string  `json:"actorType"`
	PlannedPickupAt   *string `json:"plannedPickupAt"`
	PlannedDeliveryAt *string `json:"plannedDeliveryAt"`
	ActualPickupAt    *string `json:"actualPickupAt"`
	ActualDeliveryAt  *string `json:"actualDeliveryAt"`
}

func ParseAndValidate(payload []byte, meta domain.KafkaRecordMeta, configuredTopic string) (domain.ShipmentStatusEvent, *domain.PermanentError) {
	var env envelope
	if err := json.Unmarshal(payload, &env); err != nil {
		return domain.ShipmentStatusEvent{}, domain.Permanent(domain.ErrorInvalidJSON, err)
	}
	if env.SchemaVersion != domain.SchemaVersionV1 {
		return domain.ShipmentStatusEvent{}, domain.Permanent(domain.ErrorUnsupportedSchemaVersion, nil)
	}
	if !domain.IsAllowedEventType(env.EventType) {
		return domain.ShipmentStatusEvent{}, domain.Permanent(domain.ErrorInvalidEventType, nil)
	}
	eventID, err := domain.ParseUUID(env.EventID, "eventId")
	if err != nil || eventID == uuid.Nil {
		return domain.ShipmentStatusEvent{}, domain.Permanent(domain.ErrorInvalidEventID, err)
	}
	sourceEventID, err := domain.ParseUUID(env.SourceEventID, "sourceEventId")
	if err != nil || sourceEventID == uuid.Nil {
		return domain.ShipmentStatusEvent{}, domain.Permanent(domain.ErrorInvalidSourceEventID, err)
	}
	tenantID, err := domain.ParseUUID(env.TenantID, "tenantId")
	if err != nil || tenantID == uuid.Nil {
		return domain.ShipmentStatusEvent{}, domain.Permanent(domain.ErrorInvalidTenantID, err)
	}
	if strings.TrimSpace(env.Aggregate.Type) != domain.AggregateTypeShipment {
		return domain.ShipmentStatusEvent{}, domain.Permanent(domain.ErrorInvalidAggregate, nil)
	}
	shipmentID, err := domain.ParseUUID(env.Aggregate.ID, "aggregate.id")
	if err != nil || shipmentID == uuid.Nil {
		return domain.ShipmentStatusEvent{}, domain.Permanent(domain.ErrorInvalidAggregate, err)
	}
	if env.Aggregate.Version <= 0 {
		return domain.ShipmentStatusEvent{}, domain.Permanent(domain.ErrorInvalidAggregate, nil)
	}
	if !domain.IsAllowedShipmentStatus(env.Data.ToStatus) {
		return domain.ShipmentStatusEvent{}, domain.Permanent(domain.ErrorInvalidEventData, nil)
	}
	if strings.TrimSpace(meta.Topic) != "" && meta.Topic != configuredTopic {
		return domain.ShipmentStatusEvent{}, domain.Permanent(domain.ErrorEventSchemaViolation, nil)
	}
	if strings.TrimSpace(meta.Key) != "" && meta.Key != shipmentID.String() {
		return domain.ShipmentStatusEvent{}, domain.Permanent(domain.ErrorEventSchemaViolation, nil)
	}
	if err := validateEventTypeConsistency(env); err != nil {
		return domain.ShipmentStatusEvent{}, err
	}
	occurredAt, err := time.Parse(time.RFC3339Nano, env.OccurredAt)
	if err != nil {
		occurredAt, err = time.Parse(time.RFC3339, env.OccurredAt)
		if err != nil {
			return domain.ShipmentStatusEvent{}, domain.Permanent(domain.ErrorEventSchemaViolation, err)
		}
	}
	return domain.ShipmentStatusEvent{
		EventID:       eventID,
		EventType:     env.EventType,
		SchemaVersion: env.SchemaVersion,
		OccurredAt:    occurredAt.UTC(),
		TenantID:      tenantID,
		Aggregate: domain.ShipmentAggregate{
			Type:    env.Aggregate.Type,
			ID:      shipmentID,
			Version: env.Aggregate.Version,
		},
		SourceEventID: sourceEventID,
		CorrelationID: env.CorrelationID,
		Data: domain.ShipmentStatusEventData{
			FromStatus:        env.Data.FromStatus,
			ToStatus:          env.Data.ToStatus,
			ReasonCode:        env.Data.ReasonCode,
			ActorType:         env.Data.ActorType,
			PlannedPickupAt:   parseOptionalTime(env.Data.PlannedPickupAt),
			PlannedDeliveryAt: parseOptionalTime(env.Data.PlannedDeliveryAt),
			ActualPickupAt:    parseOptionalTime(env.Data.ActualPickupAt),
			ActualDeliveryAt:  parseOptionalTime(env.Data.ActualDeliveryAt),
		},
	}, nil
}

func parseOptionalTime(raw *string) *time.Time {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil
	}
	value := strings.TrimSpace(*raw)
	if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
		utc := t.UTC()
		return &utc
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		utc := t.UTC()
		return &utc
	}
	return nil
}

func validateEventTypeConsistency(env envelope) *domain.PermanentError {
	switch env.EventType {
	case domain.EventTypeCreated:
		if env.Data.FromStatus != nil {
			return domain.Permanent(domain.ErrorEventSchemaViolation, nil)
		}
	case domain.EventTypeCancelled:
		if env.Data.ToStatus != domain.StatusCancelled {
			return domain.Permanent(domain.ErrorEventSchemaViolation, nil)
		}
	case domain.EventTypeReadyForBilling:
		if env.Data.ToStatus != domain.StatusReadyForBilling {
			return domain.Permanent(domain.ErrorEventSchemaViolation, nil)
		}
	case domain.EventTypeDocumentsCompleted:
		if env.Data.ToStatus != domain.StatusDocumentsCompleted {
			return domain.Permanent(domain.ErrorEventSchemaViolation, nil)
		}
	case domain.EventTypeFinanciallyClosed:
		if env.Data.ToStatus != domain.StatusFinanciallyClosed {
			return domain.Permanent(domain.ErrorEventSchemaViolation, nil)
		}
	}
	return nil
}
