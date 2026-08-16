package domain

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

const (
	DriverEventSchemaVersion = 1
	DriverEventSourceDriver  = "driver"

	DriverEventTypeLocationUpdated     = "driver.location.updated"
	DriverEventTypeArrivedAtPickup     = "driver.arrived_at_pickup"
	DriverEventTypeDepartedPickup      = "driver.departed_pickup"
	DriverEventTypeArrivedAtDelivery   = "driver.arrived_at_delivery"
	DriverEventTypeDeliveryCompleted   = "driver.delivery.completed"
	DriverEventTypeDelayReported       = "driver.delay.reported"
	DriverEventTypeProblemReported     = "driver.problem.reported"
	DriverEventTypeDocumentsUploaded   = "driver.documents.uploaded"
	DriverEventTypeTrackingLost        = "driver.tracking.lost"
	DriverEventTypeTrackingRestored    = "driver.tracking.restored"

	// Legacy outbox type retained for backward-compatible payloads.
	OutboxEventTypeDriverExceptionReported = "driver.exception_reported"
	OutboxEventTypeDriverShipmentEvent     = "driver.shipment_event_recorded"
)

var driverOperationalEventTypeMap = map[string]string{
	"ARRIVED_AT_PICKUP":   DriverEventTypeArrivedAtPickup,
	"DEPARTED_PICKUP":     DriverEventTypeDepartedPickup,
	"ARRIVED_AT_DELIVERY": DriverEventTypeArrivedAtDelivery,
	"DELIVERY_COMPLETED":  DriverEventTypeDeliveryCompleted,
}

var driverKafkaEventTypes = map[string]struct{}{
	DriverEventTypeLocationUpdated:   {},
	DriverEventTypeArrivedAtPickup:   {},
	DriverEventTypeDepartedPickup:    {},
	DriverEventTypeArrivedAtDelivery: {},
	DriverEventTypeDeliveryCompleted: {},
	DriverEventTypeDelayReported:     {},
	DriverEventTypeProblemReported:   {},
	DriverEventTypeDocumentsUploaded: {},
	DriverEventTypeTrackingLost:      {},
	DriverEventTypeTrackingRestored:  {},
	OutboxEventTypeDriverExceptionReported: {},
	OutboxEventTypeDriverShipmentEvent:       {},
}

type DriverEventEnvelope struct {
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
	Aggregate     DriverAggregate `json:"aggregate"`
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

type DriverAggregate struct {
	Type    string `json:"type"`
	ID      string `json:"id"`
	Version int    `json:"version"`
}

func IsDriverKafkaEventType(eventType string) bool {
	_, ok := driverKafkaEventTypes[strings.TrimSpace(eventType)]
	return ok
}

func MapOperationalEventType(raw string) (string, bool) {
	mapped, ok := driverOperationalEventTypeMap[strings.TrimSpace(raw)]
	return mapped, ok
}

func MapExceptionCategoryToProblemSeverity(category string) string {
	switch strings.ToUpper(strings.TrimSpace(category)) {
	case "ACCIDENT", "VEHICLE_BREAKDOWN", "CARGO_ISSUE":
		return "critical"
	case "TRAFFIC", "ROUTE_BLOCKED", "CUSTOMER_UNAVAILABLE", "LOADING_DELAY", "UNLOADING_DELAY":
		return "high"
	case "DOCUMENT_ISSUE":
		return "medium"
	default:
		return "medium"
	}
}

func MapExceptionCategoryToEventType(category string) string {
	switch strings.ToUpper(strings.TrimSpace(category)) {
	case "LOADING_DELAY", "UNLOADING_DELAY", "TRAFFIC":
		return DriverEventTypeDelayReported
	default:
		return DriverEventTypeProblemReported
	}
}

type BuildDriverEventParams struct {
	EventID         uuid.UUID
	EventType       string
	TenantID        uuid.UUID
	ShipmentID      uuid.UUID
	ShipmentVersion int
	DriverID        uuid.UUID
	VehicleID       *uuid.UUID
	ActorID         *uuid.UUID
	SourceEventID   uuid.UUID
	OccurredAt      time.Time
	CorrelationID   *string
	RequestID       *string
	Severity        string
	ReasonCode        string
	ReasonText        *string
	ETA               *time.Time
	Latitude          *float64
	Longitude         *float64
	Accuracy          *float64
	DocumentID        string
	Metadata          map[string]any
}

func BuildDriverEventEnvelope(params BuildDriverEventParams) (DriverEventEnvelope, error) {
	if params.EventID == uuid.Nil || params.SourceEventID == uuid.Nil {
		return DriverEventEnvelope{}, apperrors.Internal("driver event ids are required", nil)
	}
	if params.TenantID == uuid.Nil || params.ShipmentID == uuid.Nil {
		return DriverEventEnvelope{}, apperrors.Internal("driver event tenant and shipment are required", nil)
	}
	eventType := strings.TrimSpace(params.EventType)
	if eventType == "" || !IsDriverKafkaEventType(eventType) {
		return DriverEventEnvelope{}, apperrors.Internal("unsupported driver event type", nil)
	}
	if params.OccurredAt.IsZero() {
		return DriverEventEnvelope{}, apperrors.Internal("occurred_at is required", nil)
	}

	env := DriverEventEnvelope{
		EventID:       params.EventID.String(),
		EventType:     eventType,
		SchemaVersion: DriverEventSchemaVersion,
		OccurredAt:    params.OccurredAt.UTC(),
		TenantID:      params.TenantID.String(),
		ShipmentID:    params.ShipmentID.String(),
		DriverID:      params.DriverID.String(),
		Source:        DriverEventSourceDriver,
		SourceEventID: params.SourceEventID.String(),
		CorrelationID: params.CorrelationID,
		RequestID:     params.RequestID,
		Severity:      strings.TrimSpace(params.Severity),
		ReasonCode:    strings.TrimSpace(params.ReasonCode),
		DocumentID:    strings.TrimSpace(params.DocumentID),
		Metadata:      params.Metadata,
		Aggregate: DriverAggregate{
			Type:    OutboxAggregateTypeShipment,
			ID:      params.ShipmentID.String(),
			Version: params.ShipmentVersion,
		},
	}
	if params.VehicleID != nil && *params.VehicleID != uuid.Nil {
		env.VehicleID = params.VehicleID.String()
	}
	if params.ActorID != nil && *params.ActorID != uuid.Nil {
		env.ActorID = params.ActorID.String()
	}
	if params.ReasonText != nil {
		env.ReasonText = strings.TrimSpace(*params.ReasonText)
	}
	if params.ETA != nil {
		eta := params.ETA.UTC()
		env.ETA = &eta
	}
	if params.Latitude != nil {
		env.Latitude = params.Latitude
	}
	if params.Longitude != nil {
		env.Longitude = params.Longitude
	}
	if params.Accuracy != nil {
		env.Accuracy = params.Accuracy
	}
	return env, nil
}

func BuildDriverEventOutbox(params BuildDriverEventParams) (ShipmentOutboxEvent, error) {
	envelope, err := BuildDriverEventEnvelope(params)
	if err != nil {
		return ShipmentOutboxEvent{}, err
	}
	payload, err := json.Marshal(envelope)
	if err != nil {
		return ShipmentOutboxEvent{}, apperrors.Internal("marshal driver event envelope", err)
	}
	if err := validateOutboxPayload(payload); err != nil {
		return ShipmentOutboxEvent{}, err
	}
	headers, err := json.Marshal(map[string]string{
		"contentType": "application/json",
		"source":      DriverEventSourceDriver,
		"eventType":   envelope.EventType,
	})
	if err != nil {
		return ShipmentOutboxEvent{}, apperrors.Internal("marshal driver event headers", err)
	}
	now := time.Now().UTC()
	return ShipmentOutboxEvent{
		ID:               params.EventID,
		TenantID:         params.TenantID,
		AggregateType:    OutboxAggregateTypeShipment,
		AggregateID:      params.ShipmentID,
		AggregateVersion: params.ShipmentVersion,
		EventType:        envelope.EventType,
		SchemaVersion:    DriverEventSchemaVersion,
		SourceEventID:    params.SourceEventID,
		Payload:          payload,
		Headers:          headers,
		Status:           OutboxStatusPending,
		Attempts:         0,
		AvailableAt:      now,
		CreatedAt:        now,
	}, nil
}

func BuildDriverExceptionOutboxPayload(exc DriverReportedException, shipmentVersion int, correlationID *string) ([]byte, error) {
	eventType := MapExceptionCategoryToEventType(exc.Category)
	if eventType == DriverEventTypeProblemReported {
		eventType = DriverEventTypeProblemReported
	}
	outbox, err := BuildDriverEventOutbox(BuildDriverEventParams{
		EventID:         uuid.New(),
		EventType:       eventType,
		TenantID:        exc.TenantID,
		ShipmentID:      exc.ShipmentID,
		ShipmentVersion: shipmentVersion,
		DriverID:        exc.DriverID,
		SourceEventID:   exc.ID,
		OccurredAt:      exc.OccurredAt,
		CorrelationID:   correlationID,
		Severity:        MapExceptionCategoryToProblemSeverity(exc.Category),
		ReasonCode:      exc.Category,
		ReasonText:      exc.Comment,
		Metadata: map[string]any{
			"legacyEventType": OutboxEventTypeDriverExceptionReported,
			"idempotencyKey":  exc.IdempotencyKey,
		},
	})
	if err != nil {
		return nil, err
	}
	return outbox.Payload, nil
}
