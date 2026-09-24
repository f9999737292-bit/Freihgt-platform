package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
	"github.com/google/uuid"
)

const predictionInputQuery = `
SELECT s.id, s.tenant_id, s.status, s.version, s.vehicle_id,
       s.destination_location_id, l.lat, l.lon, s.planned_delivery_at
FROM transport.shipments s
JOIN transport.locations l ON l.id = s.destination_location_id AND l.tenant_id = s.tenant_id
WHERE s.id = $1 AND s.tenant_id = $2 AND s.deleted_at IS NULL`

const otherActiveVehicleShipmentsQuery = `
SELECT COUNT(*)
FROM transport.shipments
WHERE tenant_id = $1 AND vehicle_id = $2 AND id <> $3 AND deleted_at IS NULL
  AND status IN (
    'VEHICLE_ASSIGNED', 'DRIVER_ASSIGNED', 'PICKUP_SLOT_BOOKED', 'DELIVERY_SLOT_BOOKED',
    'IN_PICKUP', 'LOADED', 'IN_TRANSIT', 'ARRIVED_AT_CONSIGNEE', 'UNLOADING'
  )`

func (r *ShipmentRepository) PredictionInput(ctx context.Context, id, tenantID uuid.UUID) (*domain.ShipmentPredictionInput, error) {
	var out domain.ShipmentPredictionInput
	err := r.pool.QueryRow(ctx, predictionInputQuery, id, tenantID).Scan(
		&out.ID, &out.TenantID, &out.Status, &out.Version, &out.VehicleID,
		&out.DestinationLocationID, &out.DestinationLatitude, &out.DestinationLongitude, &out.PlannedDeliveryAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperrors.NotFound("shipment not found")
	}
	if err != nil {
		return nil, mapDBError(err)
	}
	if out.VehicleID != nil {
		if err := r.pool.QueryRow(ctx, otherActiveVehicleShipmentsQuery, tenantID, *out.VehicleID, id).Scan(&out.OtherActiveVehicleShipments); err != nil {
			return nil, mapDBError(err)
		}
	}
	return &out, nil
}
