package domain

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

const (
	OutboxEventTypeDriverExceptionReported = "driver.exception_reported"
	OutboxEventTypeDriverShipmentEvent      = "driver.shipment_event_recorded"

	DriverOperationTypeStatusEvent = "status_event"
	DriverOperationTypeException   = "exception"

	DriverExceptionSource = "driver"
)

var allowedDriverOperationalEvents = map[string]string{
	"ARRIVED_AT_PICKUP":    ShipmentStatusInPickup,
	"PICKUP_COMPLETED":     ShipmentStatusLoaded,
	"DEPARTED_PICKUP":      ShipmentStatusInTransit,
	"ARRIVED_AT_DELIVERY":  ShipmentStatusArrivedAtConsignee,
	"UNLOADING_STARTED":    ShipmentStatusUnloading,
	"DELIVERY_COMPLETED":   ShipmentStatusDelivered,
}

// Informational driver events that do not change shipment status.
var driverInformationalEvents = map[string]struct{}{
	"LOADING_STARTED": {},
}

var allowedDriverExceptionCategories = map[string]struct{}{
	"TRAFFIC":               {},
	"VEHICLE_BREAKDOWN":     {},
	"ACCIDENT":              {},
	"LOADING_DELAY":         {},
	"UNLOADING_DELAY":       {},
	"CARGO_ISSUE":           {},
	"DOCUMENT_ISSUE":        {},
	"CUSTOMER_UNAVAILABLE":  {},
	"ROUTE_BLOCKED":         {},
	"OTHER":                 {},
}

type DriverOperationalEventInput struct {
	Type           string
	OccurredAt     *time.Time
	IdempotencyKey string
}

type DriverExceptionInput struct {
	Category       string
	Comment        *string
	OccurredAt     *time.Time
	IdempotencyKey string
}

type DriverReportedException struct {
	ID             uuid.UUID
	TenantID       uuid.UUID
	ShipmentID     uuid.UUID
	DriverID       uuid.UUID
	Category       string
	Comment        *string
	OccurredAt     time.Time
	ReceivedAt     time.Time
	Source         string
	IdempotencyKey string
	CreatedAt      time.Time
}

type DriverMeView struct {
	ID               uuid.UUID
	DisplayName      string
	CompanyID        uuid.UUID
	Status           string
	PreferredLocale  string
	Phone            *string
}

type DriverShipmentSummary struct {
	ID                uuid.UUID
	ShipmentNumber    string
	Status            string
	PlannedPickupAt   *time.Time
	PlannedDeliveryAt *time.Time
	OriginLocationID  uuid.UUID
	DestinationLocationID uuid.UUID
	VehicleID         *uuid.UUID
}

type DriverShipmentDetail struct {
	DriverShipmentSummary
	ActualPickupAt   *time.Time
	ActualDeliveryAt *time.Time
	TransportMode    string
	Version          int
}

type ListDriverShipmentsFilter struct {
	TenantID uuid.UUID
	DriverID uuid.UUID
	Status   *string
	Limit    int
	Offset   int
}

type DriverOperationIdempotencyRecord struct {
	ID                 uuid.UUID
	TenantID           uuid.UUID
	DriverID           uuid.UUID
	OperationType      string
	IdempotencyKey     string
	ResourceType       string
	ResourceID         uuid.UUID
	ResponseStatusCode int
	ResponseBody       []byte
}

func MapDriverEventToTargetStatus(eventType string) (string, bool, bool) {
	if target, ok := allowedDriverOperationalEvents[eventType]; ok {
		return target, true, false
	}
	if _, ok := driverInformationalEvents[eventType]; ok {
		return "", false, true
	}
	return "", false, false
}

func ValidateDriverOperationalEventInput(in DriverOperationalEventInput) error {
	key := strings.TrimSpace(in.IdempotencyKey)
	if key == "" {
		return apperrors.Validation("idempotency_key is required", map[string]any{"field": "idempotencyKey"})
	}
	if len(key) > 128 {
		return apperrors.Validation("idempotency_key must be at most 128 characters", map[string]any{"field": "idempotencyKey"})
	}
	eventType := strings.TrimSpace(in.Type)
	if eventType == "" {
		return apperrors.Validation("type is required", map[string]any{"field": "type"})
	}
	_, statusChange, informational := MapDriverEventToTargetStatus(eventType)
	if !statusChange && !informational {
		return apperrors.Validation("unsupported driver event type", map[string]any{"field": "type", "value": eventType})
	}
	if in.OccurredAt != nil {
		now := time.Now().UTC()
		if in.OccurredAt.After(now.Add(5 * time.Minute)) {
			return apperrors.Validation("occurred_at cannot be more than 5 minutes in the future", map[string]any{"field": "occurredAt"})
		}
	}
	return nil
}

func ValidateDriverExceptionInput(in DriverExceptionInput) error {
	key := strings.TrimSpace(in.IdempotencyKey)
	if key == "" {
		return apperrors.Validation("idempotency_key is required", map[string]any{"field": "idempotencyKey"})
	}
	if len(key) > 128 {
		return apperrors.Validation("idempotency_key must be at most 128 characters", map[string]any{"field": "idempotencyKey"})
	}
	category := strings.TrimSpace(strings.ToUpper(in.Category))
	if category == "" {
		return apperrors.Validation("category is required", map[string]any{"field": "category"})
	}
	if _, ok := allowedDriverExceptionCategories[category]; !ok {
		return apperrors.Validation("unsupported exception category", map[string]any{"field": "category", "value": category})
	}
	if in.Comment != nil && len(strings.TrimSpace(*in.Comment)) > 4000 {
		return apperrors.Validation("comment must be at most 4000 characters", map[string]any{"field": "comment"})
	}
	if in.OccurredAt != nil {
		now := time.Now().UTC()
		if in.OccurredAt.After(now.Add(5 * time.Minute)) {
			return apperrors.Validation("occurred_at cannot be more than 5 minutes in the future", map[string]any{"field": "occurredAt"})
		}
	}
	return nil
}

func ValidateListDriverShipmentsFilter(f ListDriverShipmentsFilter) error {
	if f.TenantID == uuid.Nil {
		return apperrors.Unauthorized("tenant context is required")
	}
	if f.DriverID == uuid.Nil {
		return apperrors.Unauthorized("driver context is required")
	}
	if f.Limit <= 0 {
		f.Limit = 50
	}
	if f.Limit > 200 {
		return apperrors.Validation("limit must be at most 200", map[string]any{"field": "limit"})
	}
	if f.Offset < 0 {
		return apperrors.Validation("offset must be non-negative", map[string]any{"field": "offset"})
	}
	return nil
}

func CanDriverAccessShipment(tenantID, driverID uuid.UUID, shipment *Shipment) bool {
	if shipment == nil || shipment.TenantID != tenantID {
		return false
	}
	if shipment.DriverID == nil || *shipment.DriverID != driverID {
		return false
	}
	return true
}

func ToDriverMeView(driver *Driver) DriverMeView {
	return DriverMeView{
		ID:              driver.ID,
		DisplayName:     driver.FullName,
		CompanyID:       driver.CarrierCompanyID,
		Status:          driver.Status,
		PreferredLocale: driver.PreferredLocale,
		Phone:           driver.Phone,
	}
}

func ToDriverShipmentSummary(s Shipment) DriverShipmentSummary {
	return DriverShipmentSummary{
		ID:                    s.ID,
		ShipmentNumber:        s.ShipmentNumber,
		Status:                s.Status,
		PlannedPickupAt:       s.PlannedPickupAt,
		PlannedDeliveryAt:     s.PlannedDeliveryAt,
		OriginLocationID:      s.OriginLocationID,
		DestinationLocationID: s.DestinationLocationID,
		VehicleID:             s.VehicleID,
	}
}

func ToDriverShipmentDetail(s Shipment) DriverShipmentDetail {
	return DriverShipmentDetail{
		DriverShipmentSummary: ToDriverShipmentSummary(s),
		ActualPickupAt:        s.ActualPickupAt,
		ActualDeliveryAt:      s.ActualDeliveryAt,
		TransportMode:         s.TransportMode,
		Version:               s.Version,
	}
}

func SanitizeDriverExceptionComment(comment *string) *string {
	if comment == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*comment)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func BuildDriverExceptionOutboxPayload(exc DriverReportedException, shipmentVersion int, correlationID *string) ([]byte, error) {
	payload := map[string]any{
		"eventId":        exc.ID.String(),
		"eventType":      OutboxEventTypeDriverExceptionReported,
		"schemaVersion":  OutboxSchemaVersion,
		"occurredAt":     exc.OccurredAt.UTC().Format(time.RFC3339Nano),
		"receivedAt":     exc.ReceivedAt.UTC().Format(time.RFC3339Nano),
		"tenantId":       exc.TenantID.String(),
		"shipmentId":     exc.ShipmentID.String(),
		"driverId":       exc.DriverID.String(),
		"category":       exc.Category,
		"source":         exc.Source,
		"idempotencyKey": exc.IdempotencyKey,
		"aggregate": map[string]any{
			"type":    OutboxAggregateTypeShipment,
			"id":      exc.ShipmentID.String(),
			"version": shipmentVersion,
		},
	}
	if correlationID != nil {
		payload["correlationId"] = *correlationID
	}
	if exc.Comment != nil {
		payload["comment"] = *exc.Comment
	}
	return json.Marshal(payload)
}
