package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/rfx-service/internal/domain"
)

// RFx source authority reads rfx.rfx_awards, rfx.rfx_award_transport_orders,
// rfx.rfx_events, and the identity rows required to prove the buyer-side creator.
// It does not write. It does not read transport orders or company.carrier_relationship*.
const rfxAwardSourceQuery = `
SELECT
	a.tenant_id,
	a.id,
	a.version,
	a.rfx_event_id,
	a.carrier_company_id,
	e.tenant_id,
	e.id,
	e.owner_company_id,
	e.status,
	(e.deleted_at IS NULL)
FROM rfx.rfx_awards a
LEFT JOIN rfx.rfx_events e ON e.id = a.rfx_event_id
WHERE a.id = $1
`

const rfxAwardLinkSourceQuery = `
SELECT
	l.tenant_id,
	l.id,
	l.version,
	l.rfx_award_id,
	l.rfx_event_id,
	l.transport_order_id,
	l.carrier_company_id,
	l.buyer_company_id,
	a.tenant_id,
	a.rfx_event_id,
	a.carrier_company_id,
	e.tenant_id,
	e.id,
	e.owner_company_id,
	e.status,
	(e.deleted_at IS NULL)
FROM rfx.rfx_award_transport_orders l
LEFT JOIN rfx.rfx_awards a ON a.id = l.rfx_award_id
LEFT JOIN rfx.rfx_events e ON e.id = a.rfx_event_id
WHERE l.id = $1
`

const rfxCreatorQuery = `
SELECT
	EXISTS (
		SELECT 1
		FROM core.company_memberships m
		JOIN core.companies c ON c.id = m.company_id
		WHERE m.tenant_id = u.tenant_id
		  AND m.user_id = u.id
		  AND m.company_id = $3
		  AND m.status = 'ACTIVE'
		  AND m.deleted_at IS NULL
		  AND c.tenant_id = u.tenant_id
		  AND c.deleted_at IS NULL
		  AND c.status = 'ACTIVE'
	),
	COALESCE((
		SELECT array_agg(r.code ORDER BY r.code)
		FROM core.user_roles ur
		JOIN core.roles r ON r.id = ur.role_id AND r.deleted_at IS NULL
		WHERE ur.tenant_id = u.tenant_id
		  AND ur.user_id = u.id
		  AND ur.company_id = $3
	), ARRAY[]::varchar[]),
	EXISTS (
		SELECT 1
		FROM core.user_roles ur
		JOIN core.roles r ON r.id = ur.role_id AND r.deleted_at IS NULL
		WHERE ur.tenant_id = u.tenant_id
		  AND ur.user_id = u.id
		  AND ur.company_id IS NULL
		  AND r.code = 'PLATFORM_ADMIN'
	)
FROM core.users u
WHERE u.id = $1
  AND u.tenant_id = $2
  AND u.deleted_at IS NULL
  AND u.status = 'ACTIVE'
`

const rfxCarrierQuery = `
SELECT EXISTS (
	SELECT 1 FROM core.companies
	WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL AND status = 'ACTIVE'
)
`

// RfxSourceAuthorityRepository is a read-only owner lookup.
type RfxSourceAuthorityRepository struct {
	pool *pgxpool.Pool
}

func NewRfxSourceAuthorityRepository(pool *pgxpool.Pool) *RfxSourceAuthorityRepository {
	return &RfxSourceAuthorityRepository{pool: pool}
}

func (r *RfxSourceAuthorityRepository) ReadAwardSource(ctx context.Context, id uuid.UUID) (domain.RfxAwardSourceRecord, error) {
	var row domain.RfxAwardSourceRecord
	var eventTenant, eventID, owner *uuid.UUID
	var status *string
	var live *bool
	err := r.pool.QueryRow(ctx, rfxAwardSourceQuery, id).Scan(
		&row.TenantID, &row.ID, &row.Version, &row.RfxEventID, &row.CarrierCompanyID,
		&eventTenant, &eventID, &owner, &status, &live,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.RfxAwardSourceRecord{}, domain.ErrRfxSourceMissing
	}
	if err != nil {
		return domain.RfxAwardSourceRecord{}, err
	}
	fillAwardEvent(&row, eventTenant, eventID, owner, status, live)
	return row, nil
}

func (r *RfxSourceAuthorityRepository) ReadAwardLinkSource(ctx context.Context, id uuid.UUID) (domain.RfxAwardLinkSourceRecord, error) {
	var row domain.RfxAwardLinkSourceRecord
	var awardTenant, awardEvent, awardCarrier *uuid.UUID
	var eventTenant, eventID, owner *uuid.UUID
	var status *string
	var live *bool
	err := r.pool.QueryRow(ctx, rfxAwardLinkSourceQuery, id).Scan(
		&row.TenantID, &row.ID, &row.Version, &row.RfxAwardID, &row.RfxEventID, &row.TransportOrderID,
		&row.CarrierCompanyID, &row.BuyerCompanyID,
		&awardTenant, &awardEvent, &awardCarrier,
		&eventTenant, &eventID, &owner, &status, &live,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.RfxAwardLinkSourceRecord{}, domain.ErrRfxSourceMissing
	}
	if err != nil {
		return domain.RfxAwardLinkSourceRecord{}, err
	}
	if awardTenant != nil && awardEvent != nil && awardCarrier != nil {
		row.AwardPresent = true
		row.AwardTenantID = *awardTenant
		row.AwardEventID = *awardEvent
		row.AwardCarrierID = *awardCarrier
	}
	fillLinkEvent(&row, eventTenant, eventID, owner, status, live)
	return row, nil
}

func (r *RfxSourceAuthorityRepository) ReadRfxCreator(ctx context.Context, tenantID, userID, companyID uuid.UUID) (domain.RfxCreatorAuthority, error) {
	var actor domain.RfxCreatorAuthority
	err := r.pool.QueryRow(ctx, rfxCreatorQuery, userID, tenantID, companyID).Scan(
		&actor.ActiveMembership, &actor.CompanyRoles, &actor.TenantGlobalPlatformAdmin,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.RfxCreatorAuthority{}, nil
	}
	if err != nil {
		return domain.RfxCreatorAuthority{}, err
	}
	actor.Active = true
	if actor.CompanyRoles == nil {
		actor.CompanyRoles = []string{}
	}
	return actor, nil
}

func (r *RfxSourceAuthorityRepository) ActiveCarrierInTenant(ctx context.Context, tenantID, companyID uuid.UUID) (bool, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx, rfxCarrierQuery, companyID, tenantID).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func fillAwardEvent(row *domain.RfxAwardSourceRecord, eventTenant, eventID, owner *uuid.UUID, status *string, live *bool) {
	if eventTenant == nil || eventID == nil || owner == nil || status == nil || live == nil {
		return
	}
	row.EventPresent = true
	row.EventTenantID = *eventTenant
	row.EventID = *eventID
	row.OwnerCompanyID = *owner
	row.EventStatus = *status
	row.EventLive = *live
}

func fillLinkEvent(row *domain.RfxAwardLinkSourceRecord, eventTenant, eventID, owner *uuid.UUID, status *string, live *bool) {
	if eventTenant == nil || eventID == nil || owner == nil || status == nil || live == nil {
		return
	}
	row.EventPresent = true
	row.EventTenantID = *eventTenant
	row.EventID = *eventID
	row.OwnerCompanyID = *owner
	row.EventStatus = *status
	row.EventLive = *live
}

func rfxSourceAuthorityStatements() []string {
	return []string{rfxAwardSourceQuery, rfxAwardLinkSourceQuery, rfxCreatorQuery, rfxCarrierQuery}
}

func rfxSourceAuthorityWritesRelationship(statement string) bool {
	lower := strings.ToLower(statement)
	if strings.Contains(lower, "company.carrier_relationship") || strings.Contains(lower, "transport.transport_orders") {
		return true
	}
	for _, word := range []string{"insert ", "update ", "delete ", "truncate ", "alter ", "drop "} {
		if strings.Contains(lower, word) {
			return true
		}
	}
	return false
}
