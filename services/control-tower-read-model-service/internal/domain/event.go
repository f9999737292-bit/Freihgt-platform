package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	AggregateTypeShipment = "SHIPMENT"
	SchemaVersionV1       = 1

	EventTypeCreated            = "shipment.created"
	EventTypeStatusChanged      = "shipment.status.changed"
	EventTypeCancelled          = "shipment.cancelled"
	EventTypeReadyForBilling    = "shipment.ready_for_billing"
	EventTypeDocumentsCompleted = "shipment.documents_completed"
	EventTypeFinanciallyClosed  = "shipment.financially_closed"

	OutcomeApplied    = "APPLIED"
	OutcomeGapApplied = "GAP_APPLIED"
	OutcomeStale      = "STALE"
	OutcomeDuplicate  = "DUPLICATE"
)

const (
	StatusCarrierAssigned           = "CARRIER_ASSIGNED"
	StatusAcceptedByCarrier         = "ACCEPTED_BY_CARRIER"
	StatusVehicleAssigned           = "VEHICLE_ASSIGNED"
	StatusDriverAssigned            = "DRIVER_ASSIGNED"
	StatusPickupSlotBooked          = "PICKUP_SLOT_BOOKED"
	StatusDeliverySlotBooked        = "DELIVERY_SLOT_BOOKED"
	StatusInPickup                  = "IN_PICKUP"
	StatusLoaded                    = "LOADED"
	StatusInTransit                 = "IN_TRANSIT"
	StatusArrivedAtConsignee        = "ARRIVED_AT_CONSIGNEE"
	StatusUnloading                 = "UNLOADING"
	StatusDelivered                 = "DELIVERED"
	StatusDeliveryConfirmed         = "DELIVERY_CONFIRMED"
	StatusDocumentsCompleted        = "DOCUMENTS_COMPLETED"
	StatusReadyForBilling           = "READY_FOR_BILLING"
	StatusIncludedInBillingRegister = "INCLUDED_IN_BILLING_REGISTER"
	StatusFinanciallyClosed         = "FINANCIALLY_CLOSED"
	StatusCancelled                 = "CANCELLED"
)

var allowedShipmentStatuses = map[string]struct{}{
	StatusCarrierAssigned: {}, StatusAcceptedByCarrier: {}, StatusVehicleAssigned: {},
	StatusDriverAssigned: {}, StatusPickupSlotBooked: {}, StatusDeliverySlotBooked: {},
	StatusInPickup: {}, StatusLoaded: {}, StatusInTransit: {}, StatusArrivedAtConsignee: {},
	StatusUnloading: {}, StatusDelivered: {}, StatusDeliveryConfirmed: {},
	StatusDocumentsCompleted: {}, StatusReadyForBilling: {}, StatusIncludedInBillingRegister: {},
	StatusFinanciallyClosed: {}, StatusCancelled: {},
}

var allowedEventTypes = map[string]struct{}{
	EventTypeCreated: {}, EventTypeStatusChanged: {}, EventTypeCancelled: {},
	EventTypeReadyForBilling: {}, EventTypeDocumentsCompleted: {}, EventTypeFinanciallyClosed: {},
}

type ShipmentStatusEvent struct {
	EventID       uuid.UUID
	EventType     string
	SchemaVersion int
	OccurredAt    time.Time
	TenantID      uuid.UUID
	Aggregate     ShipmentAggregate
	SourceEventID uuid.UUID
	CorrelationID *string
	Data          ShipmentStatusEventData
}

type ShipmentAggregate struct {
	Type    string
	ID      uuid.UUID
	Version int
}

type ShipmentStatusEventData struct {
	FromStatus *string
	ToStatus   string
	ReasonCode *string
	ActorType  string
}

type KafkaRecordMeta struct {
	Topic     string
	Partition int32
	Offset    int64
	Key       string
}

type ShipmentStatusProjection struct {
	TenantID          uuid.UUID
	ShipmentID        uuid.UUID
	ShipmentVersion   int
	CurrentStatus     string
	PreviousStatus    *string
	LastEventID       uuid.UUID
	LastSourceEventID uuid.UUID
	LastEventType     string
	LastOccurredAt    time.Time
	LastConsumedAt    time.Time
	Complete          bool
	GapDetected       bool
	GapFromVersion    *int
	GapToVersion      *int
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func IsAllowedShipmentStatus(status string) bool {
	_, ok := allowedShipmentStatuses[strings.TrimSpace(status)]
	return ok
}

func IsAllowedEventType(eventType string) bool {
	_, ok := allowedEventTypes[strings.TrimSpace(eventType)]
	return ok
}

func ParseUUID(raw, field string) (uuid.UUID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return uuid.Nil, ValidationError(field + " is required")
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, ValidationError("invalid " + field)
	}
	return id, nil
}
