package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

type MembershipRepository struct {
	pool *pgxpool.Pool
}

func NewMembershipRepository(pool *pgxpool.Pool) *MembershipRepository {
	return &MembershipRepository{pool: pool}
}

func (r *MembershipRepository) ListUserCompanyTypes(ctx context.Context, tenantID, userID uuid.UUID) ([]string, error) {
	const query = `
		SELECT DISTINCT c.company_type
		FROM core.company_memberships m
		JOIN core.companies c ON c.id = m.company_id AND c.tenant_id = m.tenant_id
		WHERE m.tenant_id = $1 AND m.user_id = $2 AND m.deleted_at IS NULL AND c.deleted_at IS NULL
		  AND m.status = 'ACTIVE' AND c.status = 'ACTIVE'
	`
	rows, err := r.pool.Query(ctx, query, tenantID, userID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()

	types := make([]string, 0)
	for rows.Next() {
		var companyType string
		if err := rows.Scan(&companyType); err != nil {
			return nil, mapDBError(err)
		}
		types = append(types, companyType)
	}
	return types, rows.Err()
}

func (r *MembershipRepository) ListUserRoleCodes(ctx context.Context, tenantID, userID uuid.UUID) ([]string, error) {
	const query = `
		SELECT DISTINCT ro.code
		FROM core.user_roles ur
		JOIN core.roles ro ON ro.id = ur.role_id
		WHERE ur.tenant_id = $1 AND ur.user_id = $2
	`
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

func (r *MembershipRepository) ListUserBuyerCompanyIDs(ctx context.Context, tenantID, userID uuid.UUID) ([]uuid.UUID, error) {
	const query = `
		SELECT DISTINCT c.id
		FROM core.company_memberships m
		JOIN core.companies c ON c.id = m.company_id AND c.tenant_id = m.tenant_id
		WHERE m.tenant_id = $1 AND m.user_id = $2 AND m.deleted_at IS NULL AND c.deleted_at IS NULL
		  AND m.status = 'ACTIVE' AND c.status = 'ACTIVE'
		  AND c.company_type IN ('SHIPPER', 'FORWARDER', 'LSP')
	`
	rows, err := r.pool.Query(ctx, query, tenantID, userID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()

	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, mapDBError(err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *MembershipRepository) ListUserCarrierCompanyIDs(ctx context.Context, tenantID, userID uuid.UUID) ([]uuid.UUID, error) {
	const query = `
		SELECT DISTINCT c.id
		FROM core.company_memberships m
		JOIN core.companies c ON c.id = m.company_id AND c.tenant_id = m.tenant_id
		WHERE m.tenant_id = $1 AND m.user_id = $2 AND m.deleted_at IS NULL AND c.deleted_at IS NULL
		  AND m.status = 'ACTIVE' AND c.status = 'ACTIVE' AND c.company_type = 'CARRIER'
	`
	rows, err := r.pool.Query(ctx, query, tenantID, userID)
	if err != nil {
		return nil, mapDBError(err)
	}
	defer rows.Close()

	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, mapDBError(err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *MembershipRepository) ListBuyerCompanyIDs(ctx context.Context, actor domain.ActorContext) ([]uuid.UUID, error) {
	if actor.UserID == uuid.Nil {
		return nil, apperrors.Forbidden("user context is required")
	}
	return r.ListUserBuyerCompanyIDs(ctx, actor.TenantID, actor.UserID)
}

func (r *MembershipRepository) ResolveActorKind(ctx context.Context, actor domain.ActorContext) (domain.ActorKind, []uuid.UUID, error) {
	if actor.UserID == uuid.Nil {
		return domain.ActorKindBuyer, nil, nil
	}
	companyTypes, err := r.ListUserCompanyTypes(ctx, actor.TenantID, actor.UserID)
	if err != nil {
		return domain.ActorKindUnknown, nil, err
	}
	roleCodes, err := r.ListUserRoleCodes(ctx, actor.TenantID, actor.UserID)
	if err != nil {
		return domain.ActorKindUnknown, nil, err
	}
	kind := domain.ClassifyActorKind(companyTypes, roleCodes)
	carrierIDs, err := r.ListUserCarrierCompanyIDs(ctx, actor.TenantID, actor.UserID)
	if err != nil {
		return domain.ActorKindUnknown, nil, err
	}
	return kind, carrierIDs, nil
}
