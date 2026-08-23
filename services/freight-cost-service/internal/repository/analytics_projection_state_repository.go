package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/freight-cost-service/internal/domain"
)

type AnalyticsProjectionStateRepository struct {
	pool *pgxpool.Pool
}

func NewAnalyticsProjectionStateRepository(pool *pgxpool.Pool) *AnalyticsProjectionStateRepository {
	return &AnalyticsProjectionStateRepository{pool: pool}
}

func (r *AnalyticsProjectionStateRepository) Get(
	ctx context.Context,
	tx pgx.Tx,
	projectionName string,
	tenantID uuid.UUID,
) (*domain.AnalyticsProjectionState, error) {
	query := `
		SELECT projection_name, tenant_id, projection_version, source_watermark,
			last_successful_run_at, calculated_at, data_through, status,
			last_error_code, last_error_message, updated_at
		FROM freight_cost.analytics_projection_state
		WHERE projection_name = $1 AND tenant_id = $2`
	var state domain.AnalyticsProjectionState
	var watermark, lastSuccess, calculatedAt, dataThrough *time.Time
	var row pgx.Row
	if tx != nil {
		row = tx.QueryRow(ctx, query, projectionName, tenantID)
	} else {
		row = r.pool.QueryRow(ctx, query, projectionName, tenantID)
	}
	if err := row.Scan(
		&state.ProjectionName, &state.TenantID, &state.ProjectionVersion, &watermark,
		&lastSuccess, &calculatedAt, &dataThrough, &state.Status,
		&state.LastErrorCode, &state.LastErrorMessage, &state.UpdatedAt,
	); err != nil {
		return nil, mapDBError(err)
	}
	state.SourceWatermark = watermark
	state.LastSuccessfulRunAt = lastSuccess
	state.CalculatedAt = calculatedAt
	state.DataThrough = dataThrough
	return &state, nil
}

func (r *AnalyticsProjectionStateRepository) Upsert(ctx context.Context, tx pgx.Tx, state *domain.AnalyticsProjectionState) error {
	query := `
		INSERT INTO freight_cost.analytics_projection_state (
			projection_name, tenant_id, projection_version, source_watermark,
			last_successful_run_at, calculated_at, data_through, status,
			last_error_code, last_error_message, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (projection_name, tenant_id) DO UPDATE SET
			projection_version = EXCLUDED.projection_version,
			source_watermark = EXCLUDED.source_watermark,
			last_successful_run_at = EXCLUDED.last_successful_run_at,
			calculated_at = EXCLUDED.calculated_at,
			data_through = EXCLUDED.data_through,
			status = EXCLUDED.status,
			last_error_code = EXCLUDED.last_error_code,
			last_error_message = EXCLUDED.last_error_message,
			updated_at = EXCLUDED.updated_at`
	_, err := tx.Exec(ctx, query,
		state.ProjectionName, state.TenantID, state.ProjectionVersion, timeArg(state.SourceWatermark),
		timeArg(state.LastSuccessfulRunAt), timeArg(state.CalculatedAt), timeArg(state.DataThrough), state.Status,
		state.LastErrorCode, state.LastErrorMessage, state.UpdatedAt.UTC(),
	)
	return mapDBError(err)
}

func (r *AnalyticsProjectionStateRepository) ListTenantIDs(ctx context.Context) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT tenant_id
		FROM freight_cost.cost_summary_projection
		ORDER BY tenant_id`)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, mapDBError(err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
