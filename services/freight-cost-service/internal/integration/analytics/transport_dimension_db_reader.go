//go:build integration

package analytics

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/freight-cost-service/internal/provider"
)

// dbTransportDimensionReader hydrates analytics dimensions from shared Postgres in integration tests.
// Production uses transport-order-service internal HTTP batch API (service boundary preserved).
type dbTransportDimensionReader struct {
	pool *pgxpool.Pool
}

func newDBTransportDimensionReader(pool *pgxpool.Pool) provider.TransportDimensionReader {
	return &dbTransportDimensionReader{pool: pool}
}

func (r *dbTransportDimensionReader) BatchGetAnalyticsDimensions(
	ctx context.Context,
	tenantID uuid.UUID,
	transportOrderIDs []uuid.UUID,
) (map[uuid.UUID]provider.TransportOrderAnalyticsDimension, error) {
	result := make(map[uuid.UUID]provider.TransportOrderAnalyticsDimension)
	if len(transportOrderIDs) == 0 {
		return result, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT
			orders.id,
			orig.country_code,
			orig.city,
			dest.country_code,
			dest.city,
			orders.transport_mode,
			orders.equipment_type
		FROM transport.transport_orders orders
		JOIN transport.locations orig ON orig.id = orders.origin_location_id AND orig.deleted_at IS NULL
		JOIN transport.locations dest ON dest.id = orders.destination_location_id AND dest.deleted_at IS NULL
		WHERE orders.tenant_id = $1
		  AND orders.id = ANY($2)
		  AND orders.deleted_at IS NULL`, tenantID, transportOrderIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var item provider.TransportOrderAnalyticsDimension
		if err := rows.Scan(
			&item.TransportOrderID,
			&item.OriginCountry,
			&item.OriginCity,
			&item.DestinationCountry,
			&item.DestinationCity,
			&item.TransportMode,
			&item.EquipmentType,
		); err != nil {
			return nil, err
		}
		item.TransportMode = strings.ToUpper(strings.TrimSpace(item.TransportMode))
		result[item.TransportOrderID] = item
	}
	return result, rows.Err()
}
