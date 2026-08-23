package provider

import (
	"context"

	"github.com/google/uuid"
)

// TransportOrderAnalyticsDimension holds authoritative transport dimensions for analytics.
type TransportOrderAnalyticsDimension struct {
	TransportOrderID   uuid.UUID
	OrderNumber        *string
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
