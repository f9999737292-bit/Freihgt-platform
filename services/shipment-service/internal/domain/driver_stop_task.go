package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	DriverStopPositionCurrent = "CURRENT"
	DriverStopPositionNext    = "NEXT"
)

// DriverStopActionSummary is built by shipment-service from materialized actions.
// It carries execution facts only.
type DriverStopActionSummary struct {
	Actions []DriverStopActionFact `json:"actions"`
	Counts  map[string]int         `json:"counts"`
}

type DriverStopActionFact struct {
	ActionID   uuid.UUID `json:"actionId"`
	ActionType string    `json:"actionType"`
	ShipmentID uuid.UUID `json:"shipmentId"`
	CargoID    uuid.UUID `json:"cargoId"`
	Ordinal    int       `json:"ordinal"`
}

type DriverStopTaskView struct {
	TaskID          uuid.UUID
	ExecutionID     uuid.UUID
	ExecutionStopID uuid.UUID
	ShipmentID      *uuid.UUID
	Ordinal         int
	LocationID      uuid.UUID
	PlannedArrival  *time.Time
	Status          string
	Version         int
	ActionSummary   DriverStopActionSummary
	Position        string
}

type DriverCurrentNextStops struct {
	Current *DriverStopTaskView
	Next    *DriverStopTaskView
}
