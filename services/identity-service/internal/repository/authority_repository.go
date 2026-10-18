package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/identity-service/internal/http/authority"
)

type AuthorityRepository struct {
	pool *pgxpool.Pool
}

func NewAuthorityRepository(pool *pgxpool.Pool) *AuthorityRepository {
	return &AuthorityRepository{pool: pool}
}

func (r *AuthorityRepository) LookupPrincipal(ctx context.Context, userID, tenantID uuid.UUID) (bool, []string, error) {
	const query = `
		SELECT r.code
		FROM core.user_roles ur
		INNER JOIN core.roles r ON r.id = ur.role_id
		WHERE ur.user_id = $1
			AND ur.tenant_id = $2
			AND ur.company_id IS NULL
			AND r.deleted_at IS NULL
		ORDER BY r.code ASC
	`
	rows, err := r.pool.Query(ctx, query, userID, tenantID)
	if err != nil {
		return false, nil, mapDBError(err)
	}
	defer rows.Close()

	codes := make([]string, 0)
	platformAdmin := false
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return false, nil, mapDBError(err)
		}
		codes = append(codes, code)
		if code == authority.PlatformAdminRole {
			platformAdmin = true
		}
	}
	if err := rows.Err(); err != nil {
		return false, nil, mapDBError(err)
	}
	return platformAdmin, codes, nil
}

func (r *AuthorityRepository) TenantExists(ctx context.Context, tenantID uuid.UUID) (bool, error) {
	const query = `
		SELECT EXISTS (
			SELECT 1 FROM core.tenants
			WHERE id = $1 AND deleted_at IS NULL
		)
	`
	var exists bool
	if err := r.pool.QueryRow(ctx, query, tenantID).Scan(&exists); err != nil {
		return false, mapDBError(err)
	}
	return exists, nil
}
