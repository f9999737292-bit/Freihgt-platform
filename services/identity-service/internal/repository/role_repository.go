package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/identity-service/internal/domain"
	apperrors "github.com/freight-platform/identity-service/internal/platform/errors"
)

type RoleRepository struct {
	pool *pgxpool.Pool
}

func NewRoleRepository(pool *pgxpool.Pool) *RoleRepository {
	return &RoleRepository{pool: pool}
}

func (r *RoleRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]domain.Role, error) {
	var roles []domain.Role
	err := measureDB("role_repository", "list_roles", func() error {
		const query = `
		SELECT id, tenant_id, code, name, description, scope, is_system, created_at, updated_at, version
		FROM core.roles
		WHERE deleted_at IS NULL AND (tenant_id IS NULL OR tenant_id = $1)
		ORDER BY is_system DESC, code ASC
	`

		rows, err := r.pool.Query(ctx, query, tenantID)
		if err != nil {
			return mapDBError(err)
		}
		defer rows.Close()

		roles = make([]domain.Role, 0)
		for rows.Next() {
			role, err := scanRole(rows)
			if err != nil {
				return mapDBError(err)
			}
			roles = append(roles, *role)
		}
		return rows.Err()
	})
	return roles, err
}

func (r *RoleRepository) GetByID(ctx context.Context, roleID uuid.UUID) (*domain.Role, error) {
	const query = `
		SELECT id, tenant_id, code, name, description, scope, is_system, created_at, updated_at, version
		FROM core.roles
		WHERE id = $1 AND deleted_at IS NULL
	`

	row := r.pool.QueryRow(ctx, query, roleID)
	role, err := scanRole(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("role not found")
		}
		return nil, mapDBError(err)
	}
	return role, nil
}

func (r *RoleRepository) RoleAvailableForTenant(ctx context.Context, roleID, tenantID uuid.UUID) (bool, error) {
	const query = `
		SELECT EXISTS (
			SELECT 1 FROM core.roles
			WHERE id = $1 AND deleted_at IS NULL AND (tenant_id IS NULL OR tenant_id = $2)
		)
	`
	var exists bool
	if err := r.pool.QueryRow(ctx, query, roleID, tenantID).Scan(&exists); err != nil {
		return false, mapDBError(err)
	}
	return exists, nil
}

func (r *RoleRepository) CompanyExists(ctx context.Context, companyID, tenantID uuid.UUID) (bool, error) {
	const query = `
		SELECT EXISTS (
			SELECT 1 FROM core.companies
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		)
	`
	var exists bool
	if err := r.pool.QueryRow(ctx, query, companyID, tenantID).Scan(&exists); err != nil {
		return false, mapDBError(err)
	}
	return exists, nil
}

func (r *RoleRepository) AssignRole(ctx context.Context, in domain.AssignRoleInput) error {
	const duplicateQuery = `
		SELECT EXISTS (
			SELECT 1 FROM core.user_roles
			WHERE user_id = $1 AND role_id = $2 AND company_id IS NOT DISTINCT FROM $3
		)
	`
	var exists bool
	if err := r.pool.QueryRow(ctx, duplicateQuery, in.UserID, in.RoleID, in.CompanyID).Scan(&exists); err != nil {
		return mapDBError(err)
	}
	if exists {
		return apperrors.Conflict("role already assigned to user", map[string]any{
			"user_id":    in.UserID.String(),
			"role_id":    in.RoleID.String(),
			"company_id": nullableUUID(in.CompanyID),
		})
	}

	const insertQuery = `
		INSERT INTO core.user_roles (tenant_id, user_id, company_id, role_id)
		VALUES ($1, $2, $3, $4)
	`
	_, err := r.pool.Exec(ctx, insertQuery, in.TenantID, in.UserID, in.CompanyID, in.RoleID)
	return mapDBError(err)
}

func (r *RoleRepository) ListUserRoles(ctx context.Context, userID, tenantID uuid.UUID) ([]domain.UserRoleAssignment, error) {
	var items []domain.UserRoleAssignment
	err := measureDB("role_repository", "list_user_roles", func() error {
		const query = `
		SELECT r.id, r.code, r.name, ur.company_id
		FROM core.user_roles ur
		INNER JOIN core.roles r ON r.id = ur.role_id
		WHERE ur.user_id = $1 AND ur.tenant_id = $2 AND r.deleted_at IS NULL
		ORDER BY r.code ASC
	`

		rows, err := r.pool.Query(ctx, query, userID, tenantID)
		if err != nil {
			return mapDBError(err)
		}
		defer rows.Close()

		items = make([]domain.UserRoleAssignment, 0)
		for rows.Next() {
			var item domain.UserRoleAssignment
			if err := rows.Scan(&item.RoleID, &item.Code, &item.Name, &item.CompanyID); err != nil {
				return mapDBError(err)
			}
			items = append(items, item)
		}
		return rows.Err()
	})
	return items, err
}

func scanRole(row scannable) (*domain.Role, error) {
	var role domain.Role
	err := row.Scan(
		&role.ID,
		&role.TenantID,
		&role.Code,
		&role.Name,
		&role.Description,
		&role.Scope,
		&role.IsSystem,
		&role.CreatedAt,
		&role.UpdatedAt,
		&role.Version,
	)
	if err != nil {
		return nil, err
	}
	return &role, nil
}

func nullableUUID(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return id.String()
}
