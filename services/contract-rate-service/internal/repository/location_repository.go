package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type LocationRepository struct {
	pool *pgxpool.Pool
}

func NewLocationRepository(pool *pgxpool.Pool) *LocationRepository {
	return &LocationRepository{pool: pool}
}

func (r *LocationRepository) ExistsInTenant(ctx context.Context, tenantID, locationID uuid.UUID) (bool, error) {
	const query = `
		SELECT EXISTS (
			SELECT 1 FROM transport.locations
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		)`
	var exists bool
	if err := r.pool.QueryRow(ctx, query, locationID, tenantID).Scan(&exists); err != nil {
		return false, mapDBError(err)
	}
	return exists, nil
}

func (r *LocationRepository) ExistsAllInTenant(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) (bool, error) {
	if len(ids) == 0 {
		return true, nil
	}
	const query = `
		SELECT COUNT(*) FROM transport.locations
		WHERE tenant_id = $1 AND id = ANY($2) AND deleted_at IS NULL`
	var count int
	if err := r.pool.QueryRow(ctx, query, tenantID, ids).Scan(&count); err != nil {
		return false, mapDBError(err)
	}
	return count == len(ids), nil
}
