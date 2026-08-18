package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

const (
	TransportOrderStatusDraft            = "DRAFT"
	TransportOrderStatusConverted        = "CONVERTED_TO_SHIPMENT"
	TransportOrderSourceSystemRfxAward   = "rfx_award"

	ExecutionActorCarrier = "CARRIER"
	ExecutionActorBuyer   = "BUYER"
)

type AwardTransportOrderLink struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	RfxEventID       uuid.UUID
	RfxAwardID       uuid.UUID
	RfxResponseID    uuid.UUID
	RfxLotID         *uuid.UUID
	TransportOrderID uuid.UUID
	CarrierCompanyID uuid.UUID
	BuyerCompanyID   uuid.UUID
	Amount           float64
	CurrencyCode     string
	ConvertedAt      time.Time
}

type ExecuteTransportOrderInput struct {
	ShipmentNumber    string
	PlannedPickupAt   *time.Time
	PlannedDeliveryAt *time.Time
}

type CarrierTransportOrderListItem struct {
	Link         AwardTransportOrderLink
	OrderNumber  string
	OrderStatus  string
	ShipmentID   *uuid.UUID
	ShipmentStatus *string
}

type OrderExecutionView struct {
	Link           AwardTransportOrderLink
	OrderNumber    string
	OrderStatus    string
	Shipment       *Shipment
	Readiness      ExecutionReadiness
	Provenance     ExecutionProvenance
	Milestones     []ShipmentStatusHistory
	SLASignals     []ExecutionSignal
	PODDocuments   []PODDocumentSummary
	AllowedActions []DriverMilestoneAction
}

type ExecutionReadiness struct {
	CarrierAccepted    bool
	DriverAssigned     bool
	VehicleAssigned    bool
	ReadyToStart       bool
	MissingRequirements []string
}

type ExecutionProvenance struct {
	RfxEventID    uuid.UUID
	RfxAwardID    uuid.UUID
	RfxResponseID uuid.UUID
	RfxLotID      *uuid.UUID
	Amount        float64
	CurrencyCode  string
}

type ListCarrierTransportOrdersFilter struct {
	TenantID         uuid.UUID
	CarrierCompanyID uuid.UUID
	Limit            int
	Offset           int
}

func ValidateExecuteTransportOrderInput(in ExecuteTransportOrderInput) error {
	if strings.TrimSpace(in.ShipmentNumber) == "" {
		return apperrors.Validation("shipment_number is required", map[string]any{"field": "shipment_number"})
	}
	return validatePlannedDates(in.PlannedPickupAt, in.PlannedDeliveryAt)
}

func ValidateListCarrierTransportOrdersFilter(filter ListCarrierTransportOrdersFilter) error {
	if err := ValidateVerifiedTenant(filter.TenantID); err != nil {
		return err
	}
	if filter.CarrierCompanyID == uuid.Nil {
		return apperrors.Validation("carrier_company_id is required", map[string]any{"field": "carrier_company_id"})
	}
	if filter.Limit < 0 || filter.Offset < 0 {
		return apperrors.Validation("limit and offset must be non-negative", nil)
	}
	return nil
}

func BuildExecutionReadiness(shipment *Shipment) ExecutionReadiness {
	readiness := ExecutionReadiness{
		CarrierAccepted: true,
		MissingRequirements: []string{},
	}
	if shipment == nil {
		readiness.CarrierAccepted = false
		readiness.MissingRequirements = append(readiness.MissingRequirements, "shipment")
		return readiness
	}

	switch shipment.Status {
	case ShipmentStatusCarrierAssigned:
		readiness.CarrierAccepted = false
		readiness.MissingRequirements = append(readiness.MissingRequirements, "carrier_acceptance")
	case ShipmentStatusAcceptedByCarrier, ShipmentStatusVehicleAssigned, ShipmentStatusDriverAssigned,
		ShipmentStatusPickupSlotBooked, ShipmentStatusInPickup, ShipmentStatusLoaded, ShipmentStatusInTransit,
		ShipmentStatusArrivedAtConsignee, ShipmentStatusUnloading, ShipmentStatusDelivered:
		readiness.CarrierAccepted = true
	}

	readiness.DriverAssigned = shipment.DriverID != nil
	readiness.VehicleAssigned = shipment.VehicleID != nil
	if !readiness.DriverAssigned {
		readiness.MissingRequirements = append(readiness.MissingRequirements, "driver")
	}
	if !readiness.VehicleAssigned {
		readiness.MissingRequirements = append(readiness.MissingRequirements, "vehicle")
	}
	readiness.ReadyToStart = readiness.CarrierAccepted && readiness.DriverAssigned && readiness.VehicleAssigned &&
		shipment.Status == ShipmentStatusDriverAssigned
	return readiness
}

func ValidateStartExecution(shipment *Shipment, carrierCompanyID uuid.UUID) error {
	if shipment == nil {
		return apperrors.NotFound("shipment not found for transport order")
	}
	if shipment.CarrierCompanyID == nil || *shipment.CarrierCompanyID != carrierCompanyID {
		return apperrors.Forbidden("carrier company does not match shipment")
	}
	readiness := BuildExecutionReadiness(shipment)
	if !readiness.ReadyToStart {
		return apperrors.Validation("execution cannot start until driver and vehicle are assigned", map[string]any{
			"missing": readiness.MissingRequirements,
			"status":  shipment.Status,
		})
	}
	return nil
}
