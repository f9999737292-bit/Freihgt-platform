package provider

import (
	"context"

	"github.com/google/uuid"
)

// TransportOrderAnalyticsDimension holds authoritative transport dimensions for v2.2C analytics.
// Display fields (order_number, carrier name, lane_label) are intentionally omitted (v2.2D).
type TransportOrderAnalyticsDimension struct {
	TransportOrderID   uuid.UUID
	OriginCountry      string
	OriginCity         *string
	DestinationCountry string
	DestinationCity    *string
	TransportMode      string
	EquipmentType      *string
}

type TransportDimensionReader interface {
	BatchGetAnalyticsDimensions(
		ctx context.Context,
		tenantID uuid.UUID,
		transportOrderIDs []uuid.UUID,
	) (map[uuid.UUID]TransportOrderAnalyticsDimension, error)
}
