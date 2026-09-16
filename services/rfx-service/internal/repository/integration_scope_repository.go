package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

type IntegrationScopeRepository struct {
	pool *pgxpool.Pool
	exec dbExecutor
}

func NewIntegrationScopeRepository(pool *pgxpool.Pool) *IntegrationScopeRepository {
	return &IntegrationScopeRepository{pool: pool}
}

func (r *IntegrationScopeRepository) db() dbExecutor {
	if r.exec != nil {
		return r.exec
	}
	return r.pool
}

func (r *IntegrationScopeRepository) Grant(ctx context.Context, tenantID, principalID uuid.UUID, scope string) error {
	if err := domain.ValidateIntegrationScope(scope); err != nil {
		return err
	}
	if tenantID == uuid.Nil || principalID == uuid.Nil {
		return apperrors.Validation("tenant_id and integration_principal_id are required", map[string]any{"field": "owner"})
	}
	_, err := r.db().Exec(ctx, `
		INSERT INTO rfx.rfx_integration_scopes (tenant_id, integration_principal_id, scope)
		VALUES ($1, $2, $3)
		ON CONFLICT (integration_principal_id, scope) DO NOTHING
	`, tenantID, principalID, scope)
	return mapDBError(err)
}

func (r *IntegrationScopeRepository) ListByPrincipal(ctx context.Context, tenantID, principalID uuid.UUID) ([]string, error) {
	rows, err := r.db().Query(ctx, `
		SELECT scope
		FROM rfx.rfx_integration_scopes
		WHERE tenant_id = $1 AND integration_principal_id = $2
		ORDER BY scope ASC
	`, tenantID, principalID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()
	scopes := make([]string, 0)
	for rows.Next() {
		var scope string
		if err := rows.Scan(&scope); err != nil {
			return nil, mapDBError(err)
		}
		scopes = append(scopes, scope)
	}
	return scopes, rows.Err()
}

func (r *IntegrationScopeRepository) HasScope(ctx context.Context, tenantID, principalID uuid.UUID, scope string) (bool, error) {
	if err := domain.ValidateIntegrationScope(scope); err != nil {
		return false, err
	}
	var exists bool
	err := r.db().QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM rfx.rfx_integration_scopes
			WHERE tenant_id = $1 AND integration_principal_id = $2 AND scope = $3
		)
	`, tenantID, principalID, scope).Scan(&exists)
	if err != nil {
		return false, mapDBError(err)
	}
	return exists, nil
}
