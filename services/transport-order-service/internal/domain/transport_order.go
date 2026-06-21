package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/transport-order-service/internal/platform/errors"
)

const (
	TransportOrderStatusDraft              = "DRAFT"
	TransportOrderStatusReadyForSourcing   = "READY_FOR_SOURCING"
	TransportOrderStatusSourcingInProgress = "SOURCING_IN_PROGRESS"
	TransportOrderStatusAssigned           = "ASSIGNED"
	TransportOrderStatusCancelled          = "CANCELLED"
	TransportOrderStatusConverted          = "CONVERTED_TO_SHIPMENT"

	TransportModeRoad = "ROAD"
)

type TransportOrder struct {
	ID                    uuid.UUID
	TenantID              uuid.UUID
	OrderNumber           string
	ShipperCompanyID      uuid.UUID
	ConsigneeCompanyID    uuid.UUID
	OriginLocationID      uuid.UUID
	DestinationLocationID uuid.UUID
	CargoID               *uuid.UUID
	RequestedPickupDate   *time.Time
	RequestedDeliveryDate *time.Time
	TransportMode         string
	EquipmentType         *string
	Status                string
	SourceSystem          *string
	ExternalReference     *string
	CreatedAt             time.Time
	UpdatedAt             time.Time
	Version               int
}

type CreateTransportOrderInput struct {
	TenantID              uuid.UUID
	OrderNumber           string
	ShipperCompanyID      uuid.UUID
	ConsigneeCompanyID    uuid.UUID
	OriginLocationID      uuid.UUID
	DestinationLocationID uuid.UUID
	CargoID               uuid.UUID
	RequestedPickupDate   *time.Time
	RequestedDeliveryDate *time.Time
	TransportMode         string
	EquipmentType         *string
	SourceSystem          *string
	ExternalReference     *string
}

type UpdateTransportOrderInput struct {
	RequestedPickupDate   *time.Time
	RequestedDeliveryDate *time.Time
	EquipmentType         *string
	TransportMode         *string
}

type ListTransportOrdersFilter struct {
	TenantID           uuid.UUID
	ShipperCompanyID   *uuid.UUID
	ConsigneeCompanyID *uuid.UUID
	Status             *string
	Limit              int
	Offset             int
}

func ValidateCreateTransportOrderInput(in CreateTransportOrderInput) error {
	if in.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if strings.TrimSpace(in.OrderNumber) == "" {
		return apperrors.Validation("order_number is required", map[string]any{"field": "order_number"})
	}
	if in.ShipperCompanyID == uuid.Nil {
		return apperrors.Validation("shipper_company_id is required", map[string]any{"field": "shipper_company_id"})
	}
	if in.ConsigneeCompanyID == uuid.Nil {
		return apperrors.Validation("consignee_company_id is required", map[string]any{"field": "consignee_company_id"})
	}
	if in.OriginLocationID == uuid.Nil {
		return apperrors.Validation("origin_location_id is required", map[string]any{"field": "origin_location_id"})
	}
	if in.DestinationLocationID == uuid.Nil {
		return apperrors.Validation("destination_location_id is required", map[string]any{"field": "destination_location_id"})
	}
	if in.CargoID == uuid.Nil {
		return apperrors.Validation("cargo_id is required", map[string]any{"field": "cargo_id"})
	}
	if err := validateTransportMode(in.TransportMode); err != nil {
		return err
	}
	return validatePickupDeliveryDates(in.RequestedPickupDate, in.RequestedDeliveryDate)
}

func ValidateUpdateTransportOrderInput(current *TransportOrder, in UpdateTransportOrderInput) error {
	pickup := current.RequestedPickupDate
	if in.RequestedPickupDate != nil {
		pickup = in.RequestedPickupDate
	}
	delivery := current.RequestedDeliveryDate
	if in.RequestedDeliveryDate != nil {
		delivery = in.RequestedDeliveryDate
	}
	if in.TransportMode != nil {
		if err := validateTransportMode(*in.TransportMode); err != nil {
			return err
		}
	}
	return validatePickupDeliveryDates(pickup, delivery)
}

func ValidateListTransportOrdersFilter(f ListTransportOrdersFilter) error {
	if f.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if f.Limit <= 0 {
		return apperrors.Validation("limit must be greater than 0", map[string]any{"field": "limit"})
	}
	if f.Limit > 100 {
		return apperrors.Validation("limit must be less than or equal to 100", map[string]any{"field": "limit"})
	}
	if f.Offset < 0 {
		return apperrors.Validation("offset must be greater than or equal to 0", map[string]any{"field": "offset"})
	}
	return nil
}

func ValidateSubmitTransportOrder(status string) error {
	if status != TransportOrderStatusDraft {
		return apperrors.Validation("transport order can only be submitted from DRAFT status", map[string]any{
			"field":  "status",
			"status": status,
		})
	}
	return nil
}

func ValidateCancelTransportOrder(status string) error {
	if status != TransportOrderStatusDraft && status != TransportOrderStatusReadyForSourcing {
		return apperrors.Validation("transport order can only be cancelled from DRAFT or READY_FOR_SOURCING status", map[string]any{
			"field":  "status",
			"status": status,
		})
	}
	return nil
}

func ValidateUpdateTransportOrderStatus(status string) error {
	if status != TransportOrderStatusDraft {
		return apperrors.Validation("transport order can only be updated in DRAFT status", map[string]any{
			"field":  "status",
			"status": status,
		})
	}
	return nil
}

func ParseDate(value, field string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, apperrors.Validation(field+" is required", map[string]any{"field": field})
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil, apperrors.Validation("invalid "+field, map[string]any{"field": field, "format": "YYYY-MM-DD"})
	}
	return &parsed, nil
}

func ParseOptionalDate(value, field string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil, apperrors.Validation("invalid "+field, map[string]any{"field": field, "format": "YYYY-MM-DD"})
	}
	return &parsed, nil
}

func FormatDate(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.Format("2006-01-02")
	return &formatted
}

func validateTransportMode(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		value = TransportModeRoad
	}
	if value != TransportModeRoad {
		return apperrors.Validation("transport_mode must be ROAD", map[string]any{"field": "transport_mode", "value": value})
	}
	return nil
}

func validatePickupDeliveryDates(pickup, delivery *time.Time) error {
	if pickup != nil && delivery != nil && delivery.Before(*pickup) {
		return apperrors.Validation("requested_delivery_date cannot be earlier than requested_pickup_date", map[string]any{
			"field": "requested_delivery_date",
		})
	}
	return nil
}

func NormalizeTransportMode(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return TransportModeRoad
	}
	return value
}
