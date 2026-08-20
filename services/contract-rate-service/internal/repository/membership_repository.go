package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/contract-rate-service/internal/domain"
)

type MembershipRepository struct {
	pool *pgxpool.Pool
}

func NewMembershipRepository(pool *pgxpool.Pool) *MembershipRepository {
	return &MembershipRepository{pool: pool}
}

func (r *MembershipRepository) ListUserCompanyMemberships(ctx context.Context, tenantID, userID uuid.UUID) ([]domain.UserCompanyMembership, error) {
	const query = `
		SELECT c.id, c.company_type, COALESCE(array_agg(DISTINCT ro.code) FILTER (WHERE ro.code IS NOT NULL), '{}')
		FROM core.company_memberships m
		JOIN core.companies c ON c.id = m.company_id AND c.tenant_id = m.tenant_id
		LEFT JOIN core.user_roles ur ON ur.user_id = m.user_id AND ur.company_id = m.company_id AND ur.tenant_id = m.tenant_id
		LEFT JOIN core.roles ro ON ro.id = ur.role_id AND ro.deleted_at IS NULL
		WHERE m.tenant_id = $1 AND m.user_id = $2
		  AND m.deleted_at IS NULL AND m.status = 'ACTIVE'
		  AND c.deleted_at IS NULL AND c.status = 'ACTIVE'
		GROUP BY c.id, c.company_type
		ORDER BY c.legal_name ASC`
	rows, err := r.pool.Query(ctx, query, tenantID, userID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()

	result := make([]domain.UserCompanyMembership, 0)
	for rows.Next() {
		var item domain.UserCompanyMembership
		var roles []string
		if err := rows.Scan(&item.CompanyID, &item.CompanyType, &roles); err != nil {
			return nil, mapDBError(err)
		}
		item.RoleCodes = roles
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *MembershipRepository) ListUserTenantRoleCodes(ctx context.Context, tenantID, userID uuid.UUID) ([]string, error) {
	const query = `
		SELECT DISTINCT ro.code
		FROM core.user_roles ur
		JOIN core.roles ro ON ro.id = ur.role_id AND ro.deleted_at IS NULL
		WHERE ur.tenant_id = $1 AND ur.user_id = $2 AND ur.company_id IS NULL
		ORDER BY ro.code ASC`
	return r.queryRoleCodes(ctx, query, tenantID, userID)
}

func (r *MembershipRepository) ListUserCompanyRoleCodes(ctx context.Context, tenantID, userID, companyID uuid.UUID) ([]string, error) {
	const query = `
		SELECT DISTINCT ro.code
		FROM core.user_roles ur
		JOIN core.roles ro ON ro.id = ur.role_id AND ro.deleted_at IS NULL
		WHERE ur.tenant_id = $1 AND ur.user_id = $2 AND ur.company_id = $3
		ORDER BY ro.code ASC`
	rows, err := r.pool.Query(ctx, query, tenantID, userID, companyID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()

	codes := make([]string, 0)
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, mapDBError(err)
		}
		codes = append(codes, code)
	}
	return codes, rows.Err()
}

func (r *MembershipRepository) HasCompanyPermission(ctx context.Context, tenantID, userID, companyID uuid.UUID, permissionCode string) (bool, error) {
	const query = `
		SELECT EXISTS (
			SELECT 1
			FROM core.company_memberships m
			JOIN core.user_roles ur
			  ON ur.tenant_id = m.tenant_id
			 AND ur.user_id = m.user_id
			 AND ur.company_id = m.company_id
			JOIN core.roles ro ON ro.id = ur.role_id AND ro.deleted_at IS NULL
			JOIN core.role_permissions rp ON rp.role_id = ro.id
			JOIN core.permissions p ON p.id = rp.permission_id
			WHERE m.tenant_id = $1
			  AND m.user_id = $2
			  AND m.company_id = $3
			  AND m.deleted_at IS NULL
			  AND m.status = 'ACTIVE'
			  AND p.code = $4
		)`
	var allowed bool
	if err := r.pool.QueryRow(ctx, query, tenantID, userID, companyID, permissionCode).Scan(&allowed); err != nil {
		return false, mapDBError(err)
	}
	return allowed, nil
}

func (r *MembershipRepository) queryRoleCodes(ctx context.Context, query string, tenantID, userID uuid.UUID) ([]string, error) {
	rows, err := r.pool.Query(ctx, query, tenantID, userID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()

	codes := make([]string, 0)
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, mapDBError(err)
		}
		codes = append(codes, code)
	}
	return codes, rows.Err()
}

func (r *MembershipRepository) CompanyExistsInTenant(ctx context.Context, tenantID, companyID uuid.UUID) (bool, error) {
	const query = `
		SELECT EXISTS (
			SELECT 1 FROM core.companies
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL AND status = 'ACTIVE'
		)`
	var exists bool
	if err := r.pool.QueryRow(ctx, query, companyID, tenantID).Scan(&exists); err != nil {
		return false, mapDBError(err)
	}
	return exists, nil
}
