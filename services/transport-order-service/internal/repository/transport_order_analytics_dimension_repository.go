package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/transport-order-service/internal/domain"
)

const MaxAnalyticsDimensionBatchSize = 500

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

type TransportOrderAnalyticsDimensionRepository struct {
	pool *pgxpool.Pool
}

func NewTransportOrderAnalyticsDimensionRepository(pool *pgxpool.Pool) *TransportOrderAnalyticsDimensionRepository {
	return &TransportOrderAnalyticsDimensionRepository{pool: pool}
}

func (r *TransportOrderAnalyticsDimensionRepository) BatchGetByTransportOrderIDs(
	ctx context.Context,
	tenantID uuid.UUID,
	transportOrderIDs []uuid.UUID,
) ([]TransportOrderAnalyticsDimension, error) {
	if len(transportOrderIDs) == 0 {
		return nil, nil
	}
	if len(transportOrderIDs) > MaxAnalyticsDimensionBatchSize {
		transportOrderIDs = transportOrderIDs[:MaxAnalyticsDimensionBatchSize]
	}
	query := `
		SELECT
			orders.id,
			orders.order_number,
			orig.country_code,
			orig.city,
			dest.country_code,
			dest.city,
			orders.transport_mode,
			orders.equipment_type
		FROM transport.transport_orders orders
		JOIN transport.locations orig
			ON orig.id = orders.origin_location_id AND orig.deleted_at IS NULL
		JOIN transport.locations dest
			ON dest.id = orders.destination_location_id AND dest.deleted_at IS NULL
		WHERE orders.tenant_id = $1
		  AND orders.id = ANY($2)
		  AND orders.deleted_at IS NULL`
	rows, err := r.pool.Query(ctx, query, tenantID, transportOrderIDs)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()

	var items []TransportOrderAnalyticsDimension
	for rows.Next() {
		var item TransportOrderAnalyticsDimension
		if err := rows.Scan(
			&item.TransportOrderID,
			&item.OrderNumber,
			&item.OriginCountry,
			&item.OriginCity,
			&item.DestinationCountry,
			&item.DestinationCity,
			&item.TransportMode,
			&item.EquipmentType,
		); err != nil {
			return nil, mapDBError(err)
		}
		item.TransportMode = domain.NormalizeTransportMode(item.TransportMode)
		items = append(items, item)
	}
	return items, mapDBError(rows.Err())
}

func ChunkTransportOrderIDs(ids []uuid.UUID, size int) [][]uuid.UUID {
	if size <= 0 {
		size = MaxAnalyticsDimensionBatchSize
	}
	var chunks [][]uuid.UUID
	for start := 0; start < len(ids); start += size {
		end := start + size
		if end > len(ids) {
			end = len(ids)
		}
		chunk := make([]uuid.UUID, end-start)
		copy(chunk, ids[start:end])
		chunks = append(chunks, chunk)
	}
	return chunks
}
