package domain

import (
	"time"

	"github.com/google/uuid"
)

type ShipmentPredictionInput struct {
	ID                          uuid.UUID  `json:"id"`
	TenantID                    uuid.UUID  `json:"tenant_id"`
	Status                      string     `json:"status"`
	Version                     int        `json:"version"`
	VehicleID                   *uuid.UUID `json:"vehicle_id"`
	DestinationLocationID       uuid.UUID  `json:"destination_location_id"`
	DestinationLatitude         *float64   `json:"destination_latitude"`
	DestinationLongitude        *float64   `json:"destination_longitude"`
	PlannedDeliveryAt           *time.Time `json:"planned_delivery_at"`
	OtherActiveVehicleShipments int        `json:"other_active_vehicle_shipments"`
}
