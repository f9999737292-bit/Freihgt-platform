package domain

import (
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	CargoEvidencePlanned          = "PLANNED"
	CargoEvidencePickedUp         = "PICKED_UP"
	CargoEvidenceConfirmedOnboard = "CONFIRMED_ONBOARD"
	CargoEvidenceUnloaded         = "UNLOADED"
	CargoEvidenceSourceDriver     = "DRIVER_OPERATION"
	OnboardCargoUnproven          = "ONBOARD_CARGO_UNPROVEN"
)

type CargoEvidenceIntent struct {
	State           string
	Source          string
	SourceEventType string
	ActorID         uuid.UUID
	DriverID        uuid.UUID
	OccurredAt      time.Time
}

type ShipmentCargoEvidence struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	ShipmentID      uuid.UUID
	ShipmentVersion int
	CargoID         uuid.UUID
	State           string
	StateVersion    int
	Source          string
	SourceEventType string
	ActorID         *uuid.UUID
	DriverID        *uuid.UUID
	OccurredAt      time.Time
	RecordedAt      time.Time
}

type OnboardCargoItem struct {
	CargoID         uuid.UUID
	State           string
	StateVersion    int
	OccurredAt      time.Time
	Source          string
	SourceEventType string
	DriverID        *uuid.UUID
}

type OnboardCargoView struct {
	ShipmentID      uuid.UUID
	ShipmentVersion int
	Resolution      string
	Items           []OnboardCargoItem
}

type ShipmentExecutionContext struct {
	ShipmentID            uuid.UUID
	TenantID              uuid.UUID
	ShipmentVersion       int
	ShipmentStatus        string
	VehicleID             *uuid.UUID
	DriverID              *uuid.UUID
	CarrierCompanyID      *uuid.UUID
	OriginLocationID      uuid.UUID
	DestinationLocationID uuid.UUID
	CargoID               *uuid.UUID
	PlannedDeliveryAt     *time.Time
	ActualPickupAt        *time.Time
	ActualDeliveryAt      *time.Time
}

func CargoEvidenceIntentForDriverEvent(eventType string, actorID, driverID uuid.UUID, occurredAt time.Time) *CargoEvidenceIntent {
	switch strings.TrimSpace(eventType) {
	case "PICKUP_COMPLETED":
		return &CargoEvidenceIntent{
			State: CargoEvidenceConfirmedOnboard, Source: CargoEvidenceSourceDriver,
			SourceEventType: "PICKUP_COMPLETED", ActorID: actorID, DriverID: driverID, OccurredAt: occurredAt.UTC(),
		}
	case "DELIVERY_COMPLETED":
		return &CargoEvidenceIntent{
			State: CargoEvidenceUnloaded, Source: CargoEvidenceSourceDriver,
			SourceEventType: "DELIVERY_COMPLETED", ActorID: actorID, DriverID: driverID, OccurredAt: occurredAt.UTC(),
		}
	default:
		return nil
	}
}

func ExecutionContextFromShipment(shipment Shipment) ShipmentExecutionContext {
	return ShipmentExecutionContext{
		ShipmentID: shipment.ID, TenantID: shipment.TenantID, ShipmentVersion: shipment.Version,
		ShipmentStatus: shipment.Status, VehicleID: shipment.VehicleID,
		DriverID: shipment.DriverID, CarrierCompanyID: shipment.CarrierCompanyID,
		OriginLocationID: shipment.OriginLocationID, DestinationLocationID: shipment.DestinationLocationID,
		CargoID: shipment.CargoID, PlannedDeliveryAt: shipment.PlannedDeliveryAt,
		ActualPickupAt: shipment.ActualPickupAt, ActualDeliveryAt: shipment.ActualDeliveryAt,
	}
}

func ResolveOnboardCargo(shipment Shipment, rows []ShipmentCargoEvidence) OnboardCargoView {
	view := OnboardCargoView{
		ShipmentID: shipment.ID, ShipmentVersion: shipment.Version,
		Resolution: OnboardCargoUnproven, Items: []OnboardCargoItem{},
	}
	ordered := append([]ShipmentCargoEvidence(nil), rows...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].StateVersion != ordered[j].StateVersion {
			return ordered[i].StateVersion > ordered[j].StateVersion
		}
		if !ordered[i].OccurredAt.Equal(ordered[j].OccurredAt) {
			return ordered[i].OccurredAt.After(ordered[j].OccurredAt)
		}
		return ordered[i].ID.String() > ordered[j].ID.String()
	})
	seen := map[uuid.UUID]struct{}{}
	for _, row := range ordered {
		if row.ShipmentID != shipment.ID {
			continue
		}
		if _, ok := seen[row.CargoID]; ok {
			continue
		}
		seen[row.CargoID] = struct{}{}
		view.Items = append(view.Items, OnboardCargoItem{
			CargoID: row.CargoID, State: row.State, StateVersion: row.StateVersion,
			OccurredAt: row.OccurredAt, Source: row.Source, SourceEventType: row.SourceEventType,
			DriverID: row.DriverID,
		})
	}
	if len(view.Items) == 0 {
		return view
	}
	switch view.Items[0].State {
	case CargoEvidenceConfirmedOnboard:
		view.Resolution = CargoEvidenceConfirmedOnboard
	case CargoEvidenceUnloaded:
		view.Resolution = CargoEvidenceUnloaded
	default:
		view.Resolution = OnboardCargoUnproven
	}
	return view
}
