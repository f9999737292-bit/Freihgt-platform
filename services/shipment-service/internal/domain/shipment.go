package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

const (
	ShipmentStatusCarrierAssigned           = "CARRIER_ASSIGNED"
	ShipmentStatusAcceptedByCarrier         = "ACCEPTED_BY_CARRIER"
	ShipmentStatusVehicleAssigned           = "VEHICLE_ASSIGNED"
	ShipmentStatusDriverAssigned            = "DRIVER_ASSIGNED"
	ShipmentStatusPickupSlotBooked          = "PICKUP_SLOT_BOOKED"
	ShipmentStatusDeliverySlotBooked        = "DELIVERY_SLOT_BOOKED"
	ShipmentStatusInPickup                  = "IN_PICKUP"
	ShipmentStatusLoaded                    = "LOADED"
	ShipmentStatusInTransit                 = "IN_TRANSIT"
	ShipmentStatusArrivedAtConsignee        = "ARRIVED_AT_CONSIGNEE"
	ShipmentStatusUnloading                 = "UNLOADING"
	ShipmentStatusDelivered                 = "DELIVERED"
	ShipmentStatusDeliveryConfirmed         = "DELIVERY_CONFIRMED"
	ShipmentStatusDocumentsCompleted        = "DOCUMENTS_COMPLETED"
	ShipmentStatusReadyForBilling           = "READY_FOR_BILLING"
	ShipmentStatusIncludedInBillingRegister = "INCLUDED_IN_BILLING_REGISTER"
	ShipmentStatusFinanciallyClosed         = "FINANCIALLY_CLOSED"
	ShipmentStatusCancelled                 = "CANCELLED"

	TransportOrderStatusAssigned         = "ASSIGNED"
	TransportOrderStatusReadyForSourcing = "READY_FOR_SOURCING"

	BidStatusAccepted = "ACCEPTED"

	DriverStatusActive  = "ACTIVE"
	VehicleStatusActive = "ACTIVE"
)

var allowedStatusTransitions = map[string][]string{
	ShipmentStatusCarrierAssigned:           {ShipmentStatusAcceptedByCarrier},
	ShipmentStatusAcceptedByCarrier:         {ShipmentStatusVehicleAssigned, ShipmentStatusDriverAssigned},
	ShipmentStatusVehicleAssigned:           {ShipmentStatusDriverAssigned},
	ShipmentStatusDriverAssigned:            {ShipmentStatusPickupSlotBooked},
	ShipmentStatusPickupSlotBooked:          {ShipmentStatusInPickup},
	ShipmentStatusInPickup:                  {ShipmentStatusLoaded},
	ShipmentStatusLoaded:                    {ShipmentStatusInTransit},
	ShipmentStatusInTransit:                 {ShipmentStatusArrivedAtConsignee},
	ShipmentStatusArrivedAtConsignee:        {ShipmentStatusUnloading},
	ShipmentStatusUnloading:                 {ShipmentStatusDelivered},
	ShipmentStatusDelivered:                 {ShipmentStatusDeliveryConfirmed},
	ShipmentStatusDeliveryConfirmed:         {ShipmentStatusDocumentsCompleted},
	ShipmentStatusDocumentsCompleted:        {ShipmentStatusReadyForBilling},
	ShipmentStatusReadyForBilling:           {ShipmentStatusIncludedInBillingRegister},
	ShipmentStatusIncludedInBillingRegister: {ShipmentStatusFinanciallyClosed},
}

var cancelForbiddenStatuses = map[string]struct{}{
	ShipmentStatusDelivered:                 {},
	ShipmentStatusDeliveryConfirmed:         {},
	ShipmentStatusDocumentsCompleted:        {},
	ShipmentStatusReadyForBilling:           {},
	ShipmentStatusIncludedInBillingRegister: {},
	ShipmentStatusFinanciallyClosed:         {},
}

// assignmentOwnedTargetStatuses may only be reached via accept / assign-driver / assign-vehicle.
var assignmentOwnedTargetStatuses = map[string]struct{}{
	ShipmentStatusAcceptedByCarrier: {},
	ShipmentStatusVehicleAssigned:   {},
	ShipmentStatusDriverAssigned:    {},
}

// manualStatusTransitions defines statuses reachable via generic PATCH /status only.
var manualStatusTransitions = map[string][]string{
	ShipmentStatusDriverAssigned:            {ShipmentStatusPickupSlotBooked},
	ShipmentStatusPickupSlotBooked:          {ShipmentStatusInPickup},
	ShipmentStatusInPickup:                  {ShipmentStatusLoaded},
	ShipmentStatusLoaded:                    {ShipmentStatusInTransit},
	ShipmentStatusInTransit:                 {ShipmentStatusArrivedAtConsignee},
	ShipmentStatusArrivedAtConsignee:        {ShipmentStatusUnloading},
	ShipmentStatusUnloading:                 {ShipmentStatusDelivered},
	ShipmentStatusDelivered:                 {ShipmentStatusDeliveryConfirmed},
	ShipmentStatusDeliveryConfirmed:         {ShipmentStatusDocumentsCompleted},
	ShipmentStatusDocumentsCompleted:        {ShipmentStatusReadyForBilling},
	ShipmentStatusReadyForBilling:           {ShipmentStatusIncludedInBillingRegister},
	ShipmentStatusIncludedInBillingRegister: {ShipmentStatusFinanciallyClosed},
}

type Shipment struct {
	ID                    uuid.UUID
	TenantID              uuid.UUID
	ShipmentNumber        string
	TransportOrderID      *uuid.UUID
	ShipperCompanyID      uuid.UUID
	ConsigneeCompanyID    uuid.UUID
	CarrierCompanyID      *uuid.UUID
	ForwarderCompanyID    *uuid.UUID
	DriverID              *uuid.UUID
	VehicleID             *uuid.UUID
	OriginLocationID      uuid.UUID
	DestinationLocationID uuid.UUID
	CargoID               *uuid.UUID
	TransportMode         string
	Status                string
	PlannedPickupAt       *time.Time
	PlannedDeliveryAt     *time.Time
	ActualPickupAt        *time.Time
	ActualDeliveryAt      *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
	Version               int
}

type TransportOrderSnapshot struct {
	ID                    uuid.UUID
	TenantID              uuid.UUID
	Status                string
	ShipperCompanyID      uuid.UUID
	ConsigneeCompanyID    uuid.UUID
	OriginLocationID      uuid.UUID
	DestinationLocationID uuid.UUID
	CargoID               *uuid.UUID
	TransportMode         string
}

type BidSnapshot struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	Status           string
	CarrierCompanyID uuid.UUID
}

type CreateShipmentFromOrderInput struct {
	ShipmentNumber     string
	TransportOrderID   uuid.UUID
	CarrierCompanyID   uuid.UUID
	ForwarderCompanyID *uuid.UUID
	PlannedPickupAt    *time.Time
	PlannedDeliveryAt  *time.Time
}

type CreateShipmentFromBidInput struct {
	ShipmentNumber    string
	BidID             uuid.UUID
	TransportOrderID  uuid.UUID
	PlannedPickupAt   *time.Time
	PlannedDeliveryAt *time.Time
}

type ListShipmentsFilter struct {
	TenantID           uuid.UUID
	ShipperCompanyID   *uuid.UUID
	ConsigneeCompanyID *uuid.UUID
	CarrierCompanyID   *uuid.UUID
	Status             *string
	Limit              int
	Offset             int
}

func ValidateAssignDriverParams(tenantID, shipmentID, driverID uuid.UUID) error {
	if tenantID == uuid.Nil {
		return apperrors.Unauthorized("tenant context is required")
	}
	if shipmentID == uuid.Nil {
		return apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	if driverID == uuid.Nil {
		return apperrors.Validation("driver_id is required", map[string]any{"field": "driver_id"})
	}
	return nil
}

func ValidateAssignVehicleParams(tenantID, shipmentID, vehicleID uuid.UUID) error {
	if tenantID == uuid.Nil {
		return apperrors.Unauthorized("tenant context is required")
	}
	if shipmentID == uuid.Nil {
		return apperrors.Validation("id is required", map[string]any{"field": "id"})
	}
	if vehicleID == uuid.Nil {
		return apperrors.Validation("vehicle_id is required", map[string]any{"field": "vehicle_id"})
	}
	return nil
}

type UpdateShipmentStatusInput struct {
	Status     string
	ActualTime *time.Time
}

type CancelShipmentInput struct {
	Reason string
}

func ValidateVerifiedTenant(tenantID uuid.UUID) error {
	if tenantID == uuid.Nil {
		return apperrors.Unauthorized("tenant context is required")
	}
	return nil
}

func ValidateCreateShipmentFromOrderInput(in CreateShipmentFromOrderInput) error {
	if strings.TrimSpace(in.ShipmentNumber) == "" {
		return apperrors.Validation("shipment_number is required", map[string]any{"field": "shipment_number"})
	}
	if in.TransportOrderID == uuid.Nil {
		return apperrors.Validation("transport_order_id is required", map[string]any{"field": "transport_order_id"})
	}
	if in.CarrierCompanyID == uuid.Nil {
		return apperrors.Validation("carrier_company_id is required", map[string]any{"field": "carrier_company_id"})
	}
	return validatePlannedDates(in.PlannedPickupAt, in.PlannedDeliveryAt)
}

func ValidateCreateShipmentFromBidInput(in CreateShipmentFromBidInput) error {
	if strings.TrimSpace(in.ShipmentNumber) == "" {
		return apperrors.Validation("shipment_number is required", map[string]any{"field": "shipment_number"})
	}
	if in.BidID == uuid.Nil {
		return apperrors.Validation("bid_id is required", map[string]any{"field": "bid_id"})
	}
	if in.TransportOrderID == uuid.Nil {
		return apperrors.Validation("transport_order_id is required", map[string]any{"field": "transport_order_id"})
	}
	return validatePlannedDates(in.PlannedPickupAt, in.PlannedDeliveryAt)
}

func ValidateListShipmentsFilter(f ListShipmentsFilter) error {
	if f.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if f.Limit == 0 {
		return apperrors.Validation("limit must be greater than 0", map[string]any{"field": "limit"})
	}
	return ValidateListPagination(f.Limit, f.Offset)
}

func ValidateAcceptShipmentInput() error {
	return nil
}

func ValidateUpdateShipmentStatusInput(in UpdateShipmentStatusInput) error {
	if strings.TrimSpace(in.Status) == "" {
		return apperrors.Validation("status is required", map[string]any{"field": "status"})
	}
	if in.Status == ShipmentStatusLoaded || in.Status == ShipmentStatusDelivered {
		if in.ActualTime == nil {
			return apperrors.Validation("actual_time is required for this status", map[string]any{"field": "actual_time"})
		}
	}
	return nil
}

func ValidateCancelShipmentInput(in CancelShipmentInput) error {
	return nil
}

func ValidateTransportOrderForShipment(status string) error {
	if status != TransportOrderStatusAssigned && status != TransportOrderStatusReadyForSourcing {
		return apperrors.Validation("transport order must be in ASSIGNED or READY_FOR_SOURCING status", map[string]any{
			"field":  "transport_order_id",
			"status": status,
		})
	}
	return nil
}

func ValidateBidForShipment(status string) error {
	if status != BidStatusAccepted {
		return apperrors.Validation("bid must be in ACCEPTED status", map[string]any{
			"field":  "bid_id",
			"status": status,
		})
	}
	return nil
}

func ValidateAssignDriverStatus(status string) error {
	switch status {
	case ShipmentStatusCarrierAssigned, ShipmentStatusAcceptedByCarrier, ShipmentStatusVehicleAssigned:
		return nil
	default:
		return apperrors.Validation("driver can only be assigned in CARRIER_ASSIGNED, ACCEPTED_BY_CARRIER or VEHICLE_ASSIGNED status", map[string]any{
			"field":  "status",
			"status": status,
		})
	}
}

func ValidateAssignVehicleStatus(status string) error {
	switch status {
	case ShipmentStatusCarrierAssigned, ShipmentStatusAcceptedByCarrier, ShipmentStatusDriverAssigned:
		return nil
	default:
		return apperrors.Validation("vehicle can only be assigned in CARRIER_ASSIGNED, ACCEPTED_BY_CARRIER or DRIVER_ASSIGNED status", map[string]any{
			"field":  "status",
			"status": status,
		})
	}
}

func ValidateAcceptShipmentStatus(status string) error {
	if status != ShipmentStatusCarrierAssigned {
		return apperrors.Validation("shipment can only be accepted from CARRIER_ASSIGNED status", map[string]any{
			"field":  "status",
			"status": status,
		})
	}
	return nil
}

func ValidateStatusTransition(current, next string) error {
	allowed, ok := allowedStatusTransitions[current]
	if !ok {
		return apperrors.Validation("status transition is not allowed", map[string]any{
			"field": "status",
			"from":  current,
			"to":    next,
		})
	}
	for _, candidate := range allowed {
		if candidate == next {
			return nil
		}
	}
	return apperrors.Validation("status transition is not allowed", map[string]any{
		"field": "status",
		"from":  current,
		"to":    next,
	})
}

// ValidateManualStatusTransition restricts generic PATCH /status to operational progression only.
// Assignment-owned statuses must be produced by accept / assign-driver / assign-vehicle endpoints.
func ValidateManualStatusTransition(current, next string) error {
	if _, forbidden := assignmentOwnedTargetStatuses[next]; forbidden {
		return apperrors.Validation("status transition is not allowed via manual update", map[string]any{
			"field": "status",
			"from":  current,
			"to":    next,
		})
	}
	allowed, ok := manualStatusTransitions[current]
	if !ok {
		return apperrors.Validation("status transition is not allowed", map[string]any{
			"field": "status",
			"from":  current,
			"to":    next,
		})
	}
	for _, candidate := range allowed {
		if candidate == next {
			return nil
		}
	}
	return apperrors.Validation("status transition is not allowed", map[string]any{
		"field": "status",
		"from":  current,
		"to":    next,
	})
}

// ValidateExecutionAssignmentReadiness blocks operational progression when assignment IDs are missing.
func ValidateExecutionAssignmentReadiness(shipment *Shipment, next string) error {
	if shipment == nil {
		return apperrors.Validation("shipment is required", map[string]any{"field": "shipment"})
	}
	if shipment.Status != ShipmentStatusDriverAssigned || next != ShipmentStatusPickupSlotBooked {
		return nil
	}
	if shipment.DriverID == nil {
		return apperrors.Validation("driver must be assigned before execution can start", map[string]any{"field": "driver_id"})
	}
	if shipment.VehicleID == nil {
		return apperrors.Validation("vehicle must be assigned before execution can start", map[string]any{"field": "vehicle_id"})
	}
	return nil
}

func ValidateCancelShipmentStatus(status string) error {
	if _, forbidden := cancelForbiddenStatuses[status]; forbidden {
		return apperrors.Validation("shipment cannot be cancelled in current status", map[string]any{
			"field":  "status",
			"status": status,
		})
	}
	return nil
}

func ResolveStatusAfterAssignDriver(current string, hasVehicle bool) string {
	if hasVehicle {
		return ShipmentStatusDriverAssigned
	}
	if current == ShipmentStatusCarrierAssigned {
		return ShipmentStatusAcceptedByCarrier
	}
	return current
}

func ResolveStatusAfterAssignVehicle(hasDriver bool) string {
	if hasDriver {
		return ShipmentStatusDriverAssigned
	}
	return ShipmentStatusVehicleAssigned
}

func validatePlannedDates(pickup, delivery *time.Time) error {
	if pickup != nil && delivery != nil && delivery.Before(*pickup) {
		return apperrors.Validation("planned_delivery_at cannot be earlier than planned_pickup_at", map[string]any{
			"field": "planned_delivery_at",
		})
	}
	return nil
}
