package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/freight-cost-service/internal/domain"
)

type SourceCursorRepository struct {
	pool *pgxpool.Pool
}

func NewSourceCursorRepository(pool *pgxpool.Pool) *SourceCursorRepository {
	return &SourceCursorRepository{pool: pool}
}

func (r *SourceCursorRepository) Get(ctx context.Context, tx pgx.Tx, key domain.SourceCursorKey) (*domain.SourceCursor, error) {
	query := `
		SELECT tenant_id, transport_order_id, source_service, source_type, source_id, entry_kind,
		       last_source_revision, last_source_event_id, last_cost_entry_id
		FROM freight_cost.source_cursor
		WHERE tenant_id = $1 AND transport_order_id = $2
		  AND source_service = $3 AND source_type = $4 AND source_id = $5 AND entry_kind = $6`
	var cursor domain.SourceCursor
	var row pgx.Row
	if tx != nil {
		row = tx.QueryRow(ctx, query,
			key.TenantID, key.TransportOrderID, key.SourceService, key.SourceType, key.SourceID, key.EntryKind,
		)
	} else {
		row = r.pool.QueryRow(ctx, query,
			key.TenantID, key.TransportOrderID, key.SourceService, key.SourceType, key.SourceID, key.EntryKind,
		)
	}
	err := row.Scan(
		&cursor.TenantID, &cursor.TransportOrderID, &cursor.SourceService, &cursor.SourceType,
		&cursor.SourceID, &cursor.EntryKind,
		&cursor.LastSourceRevision, &cursor.LastSourceEventID, &cursor.LastCostEntryID,
	)
	if err != nil {
		return nil, mapDBError(err)
	}
	cursor.SourceCursorKey = key
	return &cursor, nil
}

func (r *SourceCursorRepository) Upsert(ctx context.Context, tx pgx.Tx, cursor *domain.SourceCursor) error {
	query := `
		INSERT INTO freight_cost.source_cursor (
			tenant_id, transport_order_id, source_service, source_type, source_id, entry_kind,
			last_source_revision, last_source_event_id, last_cost_entry_id, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (tenant_id, transport_order_id, source_service, source_type, source_id, entry_kind)
		DO UPDATE SET
			last_source_revision = EXCLUDED.last_source_revision,
			last_source_event_id = EXCLUDED.last_source_event_id,
			last_cost_entry_id = EXCLUDED.last_cost_entry_id,
			updated_at = EXCLUDED.updated_at`
	now := time.Now().UTC()
	args := []any{
		cursor.TenantID, cursor.TransportOrderID, cursor.SourceService, cursor.SourceType,
		cursor.SourceID, cursor.EntryKind,
		cursor.LastSourceRevision, cursor.LastSourceEventID, cursor.LastCostEntryID, now,
	}
	var err error
	if tx != nil {
		_, err = tx.Exec(ctx, query, args...)
	} else {
		_, err = r.pool.Exec(ctx, query, args...)
	}
	return mapDBError(err)
}
