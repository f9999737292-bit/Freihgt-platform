package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const statusSummaryAggregateQuery = `
SELECT
    status,
    COUNT(*)::BIGINT AS shipment_count,
    SUM(COUNT(*)) OVER ()::BIGINT AS total_count
FROM transport.shipments
WHERE tenant_id = $1
  AND deleted_at IS NULL
GROUP BY status
ORDER BY status`

type StatusSummaryRow struct {
	Status         string
	ShipmentCount  int64
	TotalShipments int64
}

type ShipmentStatusSummaryRepository struct {
	pool *pgxpool.Pool
}

func NewShipmentStatusSummaryRepository(pool *pgxpool.Pool) *ShipmentStatusSummaryRepository {
	return &ShipmentStatusSummaryRepository{pool: pool}
}

func (r *ShipmentStatusSummaryRepository) GetStatusSummary(ctx context.Context, tenantID uuid.UUID) ([]StatusSummaryRow, error) {
	if tenantID == uuid.Nil {
		return nil, fmt.Errorf("tenant_id is required")
	}

	var rows []StatusSummaryRow
	err := measureDB("shipment_status_summary_repository", "get_status_summary", func() error {
		dbRows, qErr := r.pool.Query(ctx, statusSummaryAggregateQuery, tenantID)
		if qErr != nil {
			return qErr
		}
		defer dbRows.Close()

		for dbRows.Next() {
			var row StatusSummaryRow
			if scanErr := dbRows.Scan(&row.Status, &row.ShipmentCount, &row.TotalShipments); scanErr != nil {
				return scanErr
			}
			rows = append(rows, row)
		}
		return dbRows.Err()
	})
	if err != nil {
		return nil, err
	}
	return rows, nil
}
