package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/freight-cost-service/internal/domain"
)

type AnalyticsDirtyQueueRepository struct {
	pool *pgxpool.Pool
}

func NewAnalyticsDirtyQueueRepository(pool *pgxpool.Pool) *AnalyticsDirtyQueueRepository {
	return &AnalyticsDirtyQueueRepository{pool: pool}
}

func (r *AnalyticsDirtyQueueRepository) MarkDirty(
	ctx context.Context,
	tx pgx.Tx,
	entry domain.AnalyticsDirtyEntry,
) error {
	query := `
		INSERT INTO freight_cost.analytics_projection_dirty (
			tenant_id, transport_order_id, buyer_company_id, currency_code,
			period_start, period_grain, dirty_at, source_event_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (tenant_id, transport_order_id, currency_code) DO UPDATE SET
			buyer_company_id = EXCLUDED.buyer_company_id,
			period_start = EXCLUDED.period_start,
			period_grain = EXCLUDED.period_grain,
			dirty_at = EXCLUDED.dirty_at,
			source_event_id = EXCLUDED.source_event_id`
	_, err := tx.Exec(ctx, query,
		entry.TenantID, entry.TransportOrderID, entry.BuyerCompanyID, entry.CurrencyCode,
		entry.PeriodStart, entry.PeriodGrain, entry.DirtyAt.UTC(), uuidArg(entry.SourceEventID),
	)
	return mapDBError(err)
}

func uuidArg(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return *id
}

func (r *AnalyticsDirtyQueueRepository) ListBatch(ctx context.Context, limit int) ([]domain.AnalyticsDirtyEntry, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, `
		SELECT tenant_id, transport_order_id, buyer_company_id, currency_code,
			period_start, period_grain, dirty_at, source_event_id
		FROM freight_cost.analytics_projection_dirty
		ORDER BY dirty_at ASC
		LIMIT $1`, limit)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()

	var entries []domain.AnalyticsDirtyEntry
	for rows.Next() {
		var entry domain.AnalyticsDirtyEntry
		if err := rows.Scan(
			&entry.TenantID, &entry.TransportOrderID, &entry.BuyerCompanyID, &entry.CurrencyCode,
			&entry.PeriodStart, &entry.PeriodGrain, &entry.DirtyAt, &entry.SourceEventID,
		); err != nil {
			return nil, mapDBError(err)
		}
		entries = append(entries, entry)
	}
	return entries, mapDBError(rows.Err())
}

func (r *AnalyticsDirtyQueueRepository) Delete(ctx context.Context, tx pgx.Tx, entry domain.AnalyticsDirtyEntry) error {
	_, err := tx.Exec(ctx, `
		DELETE FROM freight_cost.analytics_projection_dirty
		WHERE tenant_id = $1 AND transport_order_id = $2 AND currency_code = $3`,
		entry.TenantID, entry.TransportOrderID, entry.CurrencyCode,
	)
	return mapDBError(err)
}

func (r *AnalyticsDirtyQueueRepository) DeleteByTenant(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID) error {
	_, err := tx.Exec(ctx, `DELETE FROM freight_cost.analytics_projection_dirty WHERE tenant_id = $1`, tenantID)
	return mapDBError(err)
}
