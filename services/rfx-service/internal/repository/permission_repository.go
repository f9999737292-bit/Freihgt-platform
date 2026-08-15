package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	PermissionRfxEvaluate     = "rfx.evaluate"
	PermissionRfxApproveAward = "rfx.approve_award"
	PermissionRfxAward        = "rfx.award"
)

type PermissionRepository struct {
	pool *pgxpool.Pool
}

func NewPermissionRepository(pool *pgxpool.Pool) *PermissionRepository {
	return &PermissionRepository{pool: pool}
}

func (r *PermissionRepository) UserHasPermission(ctx context.Context, userID, tenantID uuid.UUID, permissionCode string) (bool, error) {
	var allowed bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM core.user_roles ur
			INNER JOIN core.role_permissions rp ON rp.role_id = ur.role_id
			INNER JOIN core.permissions p ON p.id = rp.permission_id
			WHERE ur.user_id = $1
				AND ur.tenant_id = $2
				AND p.code = $3
		)
	`, userID, tenantID, permissionCode).Scan(&allowed)
	return allowed, mapDBError(err)
}
