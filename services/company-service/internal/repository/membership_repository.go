package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/company-service/internal/domain"
	apperrors "github.com/freight-platform/company-service/internal/platform/errors"
)

type MembershipRepository struct {
	pool *pgxpool.Pool
}

func NewMembershipRepository(pool *pgxpool.Pool) *MembershipRepository {
	return &MembershipRepository{pool: pool}
}

func (r *MembershipRepository) UserExistsInTenant(ctx context.Context, userID, tenantID uuid.UUID) (bool, error) {
	const query = `
		SELECT EXISTS (
			SELECT 1 FROM core.users
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		)
	`
	var exists bool
	if err := r.pool.QueryRow(ctx, query, userID, tenantID).Scan(&exists); err != nil {
		return false, mapDBError(err)
	}
	return exists, nil
}

func (r *MembershipRepository) RoleAvailableForTenant(ctx context.Context, roleID, tenantID uuid.UUID) (bool, error) {
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

func (r *MembershipRepository) GetMembershipByCompanyAndUser(ctx context.Context, companyID, userID uuid.UUID) (*domain.Membership, *time.Time, error) {
	const query = `
		SELECT id, tenant_id, company_id, user_id, position, status, created_at, updated_at, deleted_at, version
		FROM core.company_memberships
		WHERE company_id = $1 AND user_id = $2
	`
	var (
		m         domain.Membership
		createdAt time.Time
		updatedAt time.Time
		deletedAt *time.Time
	)
	err := r.pool.QueryRow(ctx, query, companyID, userID).Scan(
		&m.ID, &m.TenantID, &m.CompanyID, &m.UserID, &m.Position, &m.Status,
		&createdAt, &updatedAt, &deletedAt, &m.Version,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, apperrors.NotFound("membership not found")
		}
		return nil, nil, mapDBError(err)
	}
	m.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	m.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
	return &m, deletedAt, nil
}

func (r *MembershipRepository) CreateMembership(ctx context.Context, in domain.CreateMembershipInput) (*domain.Membership, error) {
	var result *domain.Membership
	err := measureDB("company_repository", "add_company_member", func() error {
		const query = `
		INSERT INTO core.company_memberships (tenant_id, company_id, user_id, position, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, tenant_id, company_id, user_id, position, status, created_at, updated_at, version
	`
		row := r.pool.QueryRow(ctx, query,
			in.TenantID,
			in.CompanyID,
			in.UserID,
			domainOptionalString(in.Position),
			domain.MembershipStatusActive,
		)
		membership, err := scanMembership(row)
		if err != nil {
			return err
		}
		result = membership
		return nil
	})
	return result, err
}

func (r *MembershipRepository) ReactivateMembership(ctx context.Context, membershipID uuid.UUID, position *string, version int) (*domain.Membership, error) {
	const query = `
		UPDATE core.company_memberships SET
			position = $2,
			status = $3,
			deleted_at = NULL,
			updated_at = now(),
			version = version + 1
		WHERE id = $1 AND deleted_at IS NOT NULL AND version = $4
		RETURNING id, tenant_id, company_id, user_id, position, status, created_at, updated_at, version
	`
	row := r.pool.QueryRow(ctx, query, membershipID, domainOptionalString(position), domain.MembershipStatusActive, version)
	membership, err := scanMembership(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.Conflict("membership was updated by another request", map[string]any{"field": "version"})
		}
		return nil, mapDBError(err)
	}
	return membership, nil
}

func (r *MembershipRepository) GetMembershipByID(ctx context.Context, membershipID, companyID uuid.UUID) (*domain.Membership, error) {
	const query = `
		SELECT id, tenant_id, company_id, user_id, position, status, created_at, updated_at, version
		FROM core.company_memberships
		WHERE id = $1 AND company_id = $2 AND deleted_at IS NULL
	`
	row := r.pool.QueryRow(ctx, query, membershipID, companyID)
	membership, err := scanMembership(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("membership not found")
		}
		return nil, mapDBError(err)
	}
	return membership, nil
}

func (r *MembershipRepository) GetCompanyMembers(ctx context.Context, filter domain.ListCompanyMembersFilter) ([]domain.CompanyMember, int, error) {
	var items []domain.CompanyMember
	var total int
	err := measureDB("company_repository", "list_company_members", func() error {
		where := strings.Builder{}
		where.WriteString(`
		FROM core.company_memberships cm
		INNER JOIN core.users u ON u.id = cm.user_id AND u.deleted_at IS NULL
		WHERE cm.company_id = $1 AND cm.tenant_id = $2 AND cm.deleted_at IS NULL
	`)
		args := []any{filter.CompanyID, filter.TenantID}
		argIdx := 3

		if filter.Status != nil {
			where.WriteString(fmt.Sprintf(" AND cm.status = $%d", argIdx))
			args = append(args, *filter.Status)
			argIdx++
		}

		countQuery := "SELECT COUNT(*) " + where.String()
		if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
			return mapDBError(err)
		}

		listQuery := `
		SELECT cm.id, cm.user_id, u.email, u.full_name, u.phone, cm.position, cm.status
	` + where.String() + fmt.Sprintf(" ORDER BY cm.created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
		args = append(args, filter.Limit, filter.Offset)

		rows, err := r.pool.Query(ctx, listQuery, args...)
		if err != nil {
			return mapDBError(err)
		}
		defer rows.Close()

		members := make([]domain.CompanyMember, 0)
		for rows.Next() {
			var member domain.CompanyMember
			if err := rows.Scan(
				&member.MembershipID,
				&member.UserID,
				&member.Email,
				&member.FullName,
				&member.Phone,
				&member.Position,
				&member.Status,
			); err != nil {
				return mapDBError(err)
			}
			roles, err := r.listMemberRoles(ctx, member.UserID, filter.CompanyID, filter.TenantID)
			if err != nil {
				return err
			}
			member.Roles = roles
			members = append(members, member)
		}
		if err := rows.Err(); err != nil {
			return mapDBError(err)
		}

		items = members
		return nil
	})
	return items, total, err
}

func (r *MembershipRepository) listMemberRoles(ctx context.Context, userID, companyID, tenantID uuid.UUID) ([]domain.MemberRole, error) {
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

	roles := make([]domain.MemberRole, 0)
	for rows.Next() {
		var role domain.MemberRole
		if err := rows.Scan(&role.RoleID, &role.Code, &role.Name); err != nil {
			return nil, mapDBError(err)
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

func (r *MembershipRepository) UpdateMembership(ctx context.Context, membershipID, companyID uuid.UUID, in domain.UpdateMembershipInput) (*domain.Membership, error) {
	current, err := r.GetMembershipByID(ctx, membershipID, companyID)
	if err != nil {
		return nil, err
	}

	position := current.Position
	if in.Position != nil {
		position = stringPtr(strings.TrimSpace(*in.Position))
	}
	status := current.Status
	if in.Status != nil {
		status = strings.TrimSpace(*in.Status)
	}

	const query = `
		UPDATE core.company_memberships SET
			position = $4,
			status = $5,
			updated_at = now(),
			version = version + 1
		WHERE id = $1 AND company_id = $2 AND tenant_id = $3 AND deleted_at IS NULL AND version = $6
		RETURNING id, tenant_id, company_id, user_id, position, status, created_at, updated_at, version
	`
	row := r.pool.QueryRow(ctx, query, membershipID, companyID, current.TenantID, domainOptionalString(position), status, current.Version)
	membership, err := scanMembership(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.Conflict("membership was updated by another request", map[string]any{"field": "version"})
		}
		return nil, mapDBError(err)
	}
	return membership, nil
}

func (r *MembershipRepository) SoftDeleteMembership(ctx context.Context, membershipID, companyID uuid.UUID) (*domain.Membership, error) {
	current, err := r.GetMembershipByID(ctx, membershipID, companyID)
	if err != nil {
		return nil, err
	}

	const query = `
		UPDATE core.company_memberships SET
			deleted_at = now(),
			status = $4,
			updated_at = now(),
			version = version + 1
		WHERE id = $1 AND company_id = $2 AND tenant_id = $3 AND deleted_at IS NULL AND version = $5
		RETURNING id, tenant_id, company_id, user_id, position, status, created_at, updated_at, version
	`
	row := r.pool.QueryRow(ctx, query, membershipID, companyID, current.TenantID, domain.MembershipStatusDeleted, current.Version)
	membership, err := scanMembership(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.Conflict("membership was updated by another request", map[string]any{"field": "version"})
		}
		return nil, mapDBError(err)
	}
	return membership, nil
}

func (r *MembershipRepository) AddUserRoleForCompany(ctx context.Context, tenantID, userID, companyID, roleID uuid.UUID) error {
	const duplicateQuery = `
		SELECT EXISTS (
			SELECT 1 FROM core.user_roles
			WHERE user_id = $1 AND role_id = $2 AND company_id = $3 AND tenant_id = $4
		)
	`
	var exists bool
	if err := r.pool.QueryRow(ctx, duplicateQuery, userID, roleID, companyID, tenantID).Scan(&exists); err != nil {
		return mapDBError(err)
	}
	if exists {
		return apperrors.Conflict("role already assigned to user in company", map[string]any{
			"user_id":    userID.String(),
			"company_id": companyID.String(),
			"role_id":    roleID.String(),
		})
	}

	const insertQuery = `
		INSERT INTO core.user_roles (tenant_id, user_id, company_id, role_id)
		VALUES ($1, $2, $3, $4)
	`
	_, err := r.pool.Exec(ctx, insertQuery, tenantID, userID, companyID, roleID)
	return mapDBError(err)
}

// RemoveUserRolesForCompany physically deletes user_roles rows for the company/user pair.
// TODO: introduce status/deactivation on core.user_roles instead of hard delete in a future iteration.
func (r *MembershipRepository) RemoveUserRolesForCompany(ctx context.Context, tenantID, userID, companyID uuid.UUID) error {
	const query = `
		DELETE FROM core.user_roles
		WHERE tenant_id = $1 AND user_id = $2 AND company_id = $3
	`
	_, err := r.pool.Exec(ctx, query, tenantID, userID, companyID)
	return mapDBError(err)
}

func scanMembership(row pgx.Row) (*domain.Membership, error) {
	var (
		m         domain.Membership
		createdAt time.Time
		updatedAt time.Time
	)
	if err := row.Scan(
		&m.ID, &m.TenantID, &m.CompanyID, &m.UserID, &m.Position, &m.Status,
		&createdAt, &updatedAt, &m.Version,
	); err != nil {
		return nil, err
	}
	m.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	m.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
	return &m, nil
}

func domainOptionalString(value *string) any {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return trimmed
}
