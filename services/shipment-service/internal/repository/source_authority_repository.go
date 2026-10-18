package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/shipment-service/internal/domain"
)

// Shipment source authority reads transport.shipments and the identity rows
// required to prove the shipper-side creator. It does not write, and it does
// not read or write company.carrier_relationship* tables.
const shipmentSourceQuery = `
SELECT tenant_id, id, version, shipper_company_id, carrier_company_id, (deleted_at IS NULL)
FROM transport.shipments
WHERE id = $1
`

const shipmentCreatorQuery = `
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

const shipmentCarrierQuery = `
SELECT EXISTS (
	SELECT 1 FROM core.companies
	WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL AND status = 'ACTIVE'
)
`

// ShipmentSourceAuthorityRepository is a read-only owner lookup.
type ShipmentSourceAuthorityRepository struct {
	pool *pgxpool.Pool
}

func NewShipmentSourceAuthorityRepository(pool *pgxpool.Pool) *ShipmentSourceAuthorityRepository {
	return &ShipmentSourceAuthorityRepository{pool: pool}
}

func (r *ShipmentSourceAuthorityRepository) ReadShipmentSource(ctx context.Context, id uuid.UUID) (domain.ShipmentSourceRecord, error) {
	var row domain.ShipmentSourceRecord
	var carrier *uuid.UUID
	err := r.pool.QueryRow(ctx, shipmentSourceQuery, id).Scan(
		&row.TenantID, &row.ID, &row.Version, &row.ShipperCompanyID, &carrier, &row.Live,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ShipmentSourceRecord{}, domain.ErrShipmentSourceMissing
	}
	if err != nil {
		return domain.ShipmentSourceRecord{}, err
	}
	if carrier != nil && *carrier != uuid.Nil {
		row.CarrierCompanyID = *carrier
		row.CarrierAssigned = true
	}
	return row, nil
}

func (r *ShipmentSourceAuthorityRepository) ReadShipmentCreator(ctx context.Context, tenantID, userID, companyID uuid.UUID) (domain.ShipmentCreatorAuthority, error) {
	var actor domain.ShipmentCreatorAuthority
	err := r.pool.QueryRow(ctx, shipmentCreatorQuery, userID, tenantID, companyID).Scan(
		&actor.ActiveMembership, &actor.CompanyRoles, &actor.TenantGlobalPlatformAdmin,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ShipmentCreatorAuthority{}, nil
	}
	if err != nil {
		return domain.ShipmentCreatorAuthority{}, err
	}
	actor.Active = true
	if actor.CompanyRoles == nil {
		actor.CompanyRoles = []string{}
	}
	return actor, nil
}

func (r *ShipmentSourceAuthorityRepository) ActiveCarrierInTenant(ctx context.Context, tenantID, companyID uuid.UUID) (bool, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx, shipmentCarrierQuery, companyID, tenantID).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func shipmentSourceAuthorityStatements() []string {
	return []string{shipmentSourceQuery, shipmentCreatorQuery, shipmentCarrierQuery}
}

func shipmentSourceAuthorityWritesRelationship(statement string) bool {
	lower := strings.ToLower(statement)
	if strings.Contains(lower, "company.carrier_relationship") {
		return true
	}
	for _, word := range []string{"insert ", "update ", "delete ", "truncate ", "alter ", "drop "} {
		if strings.Contains(lower, word) {
			return true
		}
	}
	return false
}
