package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/identity-service/internal/domain"
)

type PermissionRepository struct {
	pool *pgxpool.Pool
}

func NewPermissionRepository(pool *pgxpool.Pool) *PermissionRepository {
	return &PermissionRepository{pool: pool}
}

func (r *PermissionRepository) ListByRoleID(ctx context.Context, roleID uuid.UUID) ([]domain.Permission, error) {
	var permissions []domain.Permission
	err := measureDB("permission_repository", "list_role_permissions", func() error {
		const query = `
		SELECT p.id, p.code, p.resource, p.action, p.description, p.created_at
		FROM core.role_permissions rp
		INNER JOIN core.permissions p ON p.id = rp.permission_id
		WHERE rp.role_id = $1
		ORDER BY p.code ASC
	`

		rows, err := r.pool.Query(ctx, query, roleID)
		if err != nil {
			return mapDBError(err)
		}
		defer rows.Close()

		permissions = make([]domain.Permission, 0)
		for rows.Next() {
			var p domain.Permission
			if err := rows.Scan(&p.ID, &p.Code, &p.Resource, &p.Action, &p.Description, &p.CreatedAt); err != nil {
				return mapDBError(err)
			}
			permissions = append(permissions, p)
		}
		return rows.Err()
	})
	return permissions, err
}
