package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/identity-service/internal/domain"
	apperrors "github.com/freight-platform/identity-service/internal/platform/errors"
)

type MembershipRepository struct {
	pool *pgxpool.Pool
}

func NewMembershipRepository(pool *pgxpool.Pool) *MembershipRepository {
	return &MembershipRepository{pool: pool}
}

func (r *MembershipRepository) GetUserCompanies(ctx context.Context, filter domain.ListUserCompaniesFilter) ([]domain.UserCompany, error) {
	where := strings.Builder{}
	where.WriteString(`
		FROM core.company_memberships cm
		INNER JOIN core.companies c ON c.id = cm.company_id AND c.deleted_at IS NULL
		WHERE cm.user_id = $1 AND cm.tenant_id = $2 AND cm.deleted_at IS NULL
	`)
	args := []any{filter.UserID, filter.TenantID}
	argIdx := 3

	if filter.Status != nil {
		where.WriteString(fmt.Sprintf(" AND cm.status = $%d", argIdx))
		args = append(args, *filter.Status)
	}

	listQuery := `
		SELECT cm.id, cm.company_id, c.legal_name, c.short_name, c.company_type, cm.position, cm.status
	` + where.String() + " ORDER BY c.legal_name ASC"

	rows, err := r.pool.Query(ctx, listQuery, args...)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()

	companies := make([]domain.UserCompany, 0)
	for rows.Next() {
		var company domain.UserCompany
		if err := rows.Scan(
			&company.MembershipID,
			&company.CompanyID,
			&company.LegalName,
			&company.ShortName,
			&company.CompanyType,
			&company.Position,
			&company.MembershipStatus,
		); err != nil {
			return nil, mapDBError(err)
		}
		roles, err := r.listCompanyRoles(ctx, filter.UserID, company.CompanyID, filter.TenantID)
		if err != nil {
			return nil, err
		}
		company.Roles = roles
		companies = append(companies, company)
	}
	if err := rows.Err(); err != nil {
		return nil, mapDBError(err)
	}

	return companies, nil
}

func (r *MembershipRepository) listCompanyRoles(ctx context.Context, userID, companyID, tenantID uuid.UUID) ([]domain.CompanyRole, error) {
	const query = `
		SELECT r.id, r.code, r.name
		FROM core.user_roles ur
		INNER JOIN core.roles r ON r.id = ur.role_id
		WHERE ur.user_id = $1 AND ur.company_id = $2 AND ur.tenant_id = $3 AND r.deleted_at IS NULL
		ORDER BY r.code ASC
	`
	rows, err := r.pool.Query(ctx, query, userID, companyID, tenantID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()

	roles := make([]domain.CompanyRole, 0)
	for rows.Next() {
		var role domain.CompanyRole
		if err := rows.Scan(&role.RoleID, &role.Code, &role.Name); err != nil {
			return nil, mapDBError(err)
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

func (r *MembershipRepository) ActiveMembershipExists(ctx context.Context, tenantID, userID, companyID uuid.UUID) (bool, error) {
	const query = `
		SELECT EXISTS (
			SELECT 1 FROM core.company_memberships
			WHERE tenant_id = $1 AND user_id = $2 AND company_id = $3
				AND status = $4 AND deleted_at IS NULL
		)
	`
	var exists bool
	if err := r.pool.QueryRow(ctx, query, tenantID, userID, companyID, domain.MembershipStatusActive).Scan(&exists); err != nil {
		return false, mapDBError(err)
	}
	return exists, nil
}

func (r *MembershipRepository) AddCompanyRoleToUser(ctx context.Context, in domain.AssignCompanyRoleInput) error {
	const duplicateQuery = `
		SELECT EXISTS (
			SELECT 1 FROM core.user_roles
			WHERE tenant_id = $1 AND user_id = $2 AND company_id = $3 AND role_id = $4
		)
	`
	var exists bool
	if err := r.pool.QueryRow(ctx, duplicateQuery, in.TenantID, in.UserID, in.CompanyID, in.RoleID).Scan(&exists); err != nil {
		return mapDBError(err)
	}
	if exists {
		return apperrors.Conflict("role already assigned to user in company", map[string]any{
			"user_id":    in.UserID.String(),
			"company_id": in.CompanyID.String(),
			"role_id":    in.RoleID.String(),
		})
	}

	const insertQuery = `
		INSERT INTO core.user_roles (tenant_id, user_id, company_id, role_id)
		VALUES ($1, $2, $3, $4)
	`
	_, err := r.pool.Exec(ctx, insertQuery, in.TenantID, in.UserID, in.CompanyID, in.RoleID)
	return mapDBError(err)
}

func (r *MembershipRepository) RemoveCompanyRoleFromUser(ctx context.Context, tenantID, userID, companyID, roleID uuid.UUID) error {
	const query = `
		DELETE FROM core.user_roles
		WHERE tenant_id = $1 AND user_id = $2 AND company_id = $3 AND role_id = $4
	`
	tag, err := r.pool.Exec(ctx, query, tenantID, userID, companyID, roleID)
	if err != nil {
		return mapDBError(err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NotFound("role assignment not found")
	}
	return nil
}
