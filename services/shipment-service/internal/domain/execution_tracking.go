package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

type ExecutionSignal struct {
	Code     string     `json:"code"`
	Severity string     `json:"severity"`
	Message  string     `json:"message"`
	At       *time.Time `json:"at,omitempty"`
}

type DriverMilestoneAction struct {
	Type        string `json:"type"`
	LabelKey    string `json:"label_key"`
	RequiresPOD bool   `json:"requires_pod"`
}

type PODDocumentSummary struct {
	ID             uuid.UUID `json:"id"`
	DocumentNumber string    `json:"document_number"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

type ShipmentExecutionView struct {
	Shipment           Shipment
	Milestones         []ShipmentStatusHistory
	AllowedActions     []DriverMilestoneAction
	SLASignals         []ExecutionSignal
	PODDocuments       []PODDocumentSummary
	PODRequired        bool
	PODSignatureModel  string
}

type ListBuyerTransportOrdersFilter struct {
	TenantID       uuid.UUID
	BuyerCompanyID uuid.UUID
	Limit          int
	Offset         int
}

type BuyerTransportOrderListItem struct {
	Link           AwardTransportOrderLink
	OrderNumber    string
	OrderStatus    string
	ShipmentID     *uuid.UUID
	ShipmentStatus *string
}

var driverActionsByStatus = map[string][]DriverMilestoneAction{
	ShipmentStatusInPickup: {
		{Type: "LOADING_STARTED", LabelKey: "driver.milestone.loadingStarted"},
		{Type: "PICKUP_COMPLETED", LabelKey: "driver.milestone.pickupCompleted"},
	},
	ShipmentStatusLoaded: {
		{Type: "DEPARTED_PICKUP", LabelKey: "driver.milestone.departedPickup"},
	},
	ShipmentStatusInTransit: {
		{Type: "ARRIVED_AT_DELIVERY", LabelKey: "driver.milestone.arrivedDelivery"},
	},
	ShipmentStatusArrivedAtConsignee: {
		{Type: "UNLOADING_STARTED", LabelKey: "driver.milestone.unloadingStarted"},
		{Type: "DELIVERY_COMPLETED", LabelKey: "driver.milestone.deliveryCompleted", RequiresPOD: false},
	},
	ShipmentStatusUnloading: {
		{Type: "DELIVERY_COMPLETED", LabelKey: "driver.milestone.deliveryCompleted", RequiresPOD: false},
	},
	ShipmentStatusPickupSlotBooked: {
		{Type: "ARRIVED_AT_PICKUP", LabelKey: "driver.milestone.arrivedPickup"},
	},
	ShipmentStatusDriverAssigned: {
		{Type: "ARRIVED_AT_PICKUP", LabelKey: "driver.milestone.arrivedPickup"},
	},
}

func AllowedDriverMilestoneActions(status string) []DriverMilestoneAction {
	items, ok := driverActionsByStatus[strings.TrimSpace(status)]
	if !ok {
		return []DriverMilestoneAction{}
	}
	out := make([]DriverMilestoneAction, len(items))
	copy(out, items)
	return out
}

func BuildExecutionSLASignals(shipment *Shipment, now time.Time) []ExecutionSignal {
	if shipment == nil {
		return nil
	}
	now = now.UTC()
	signals := make([]ExecutionSignal, 0, 4)

	if shipment.PlannedPickupAt != nil {
		if shipment.ActualPickupAt == nil && now.After(shipment.PlannedPickupAt.UTC()) {
			signals = append(signals, ExecutionSignal{
				Code:     "PICKUP_LATE",
				Severity: "WARNING",
				Message:  "planned pickup time has passed without actual pickup",
				At:       shipment.PlannedPickupAt,
			})
		}
		if shipment.ActualPickupAt != nil && shipment.ActualPickupAt.After(shipment.PlannedPickupAt.UTC()) {
			signals = append(signals, ExecutionSignal{
				Code:     "PICKUP_LATE",
				Severity: "WARNING",
				Message:  "actual pickup occurred after planned pickup",
				At:       shipment.ActualPickupAt,
			})
		}
	}

	if shipment.PlannedDeliveryAt != nil && shipment.Status != ShipmentStatusDelivered &&
		shipment.Status != ShipmentStatusDeliveryConfirmed && shipment.Status != ShipmentStatusCancelled {
		if shipment.ActualDeliveryAt == nil && now.After(shipment.PlannedDeliveryAt.UTC()) {
			signals = append(signals, ExecutionSignal{
				Code:     "DELIVERY_LATE",
				Severity: "WARNING",
				Message:  "planned delivery time has passed without delivery",
				At:       shipment.PlannedDeliveryAt,
			})
		}
	}

	if shipment.ActualDeliveryAt != nil && shipment.PlannedDeliveryAt != nil &&
		shipment.ActualDeliveryAt.After(shipment.PlannedDeliveryAt.UTC()) {
		signals = append(signals, ExecutionSignal{
			Code:     "DELIVERY_LATE",
			Severity: "WARNING",
			Message:  "actual delivery occurred after planned delivery",
			At:       shipment.ActualDeliveryAt,
		})
	}

	return signals
}

func ValidateListBuyerTransportOrdersFilter(filter ListBuyerTransportOrdersFilter) error {
	if err := ValidateVerifiedTenant(filter.TenantID); err != nil {
		return err
	}
	if filter.BuyerCompanyID == uuid.Nil {
		return apperrors.Validation("buyer_company_id is required", map[string]any{"field": "buyer_company_id"})
	}
	if filter.Limit < 0 || filter.Offset < 0 {
		return apperrors.Validation("limit and offset must be non-negative", nil)
	}
	return nil
}
