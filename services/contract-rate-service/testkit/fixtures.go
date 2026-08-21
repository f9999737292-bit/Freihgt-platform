//go:build integration

package testkit

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func SeedTenantAndCompanies(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID, buyerID, carrierID uuid.UUID) {
	t.Helper()
	_, err := pool.Exec(ctx, `
		INSERT INTO core.tenants (id, code, name, status)
		VALUES ($1, $2, 'Test Tenant', 'ACTIVE')
		ON CONFLICT DO NOTHING`, tenantID, "T-"+tenantID.String()[:8])
	if err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	for _, row := range []struct {
		id, tenant uuid.UUID
		typ, name  string
	}{
		{buyerID, tenantID, "SHIPPER", "Buyer Co"},
		{carrierID, tenantID, "CARRIER", "Carrier Co"},
	} {
		_, err := pool.Exec(ctx, `
			INSERT INTO core.companies (id, tenant_id, company_type, legal_name, status)
			VALUES ($1,$2,$3,$4,'ACTIVE')
			ON CONFLICT DO NOTHING`, row.id, row.tenant, row.typ, row.name)
		if err != nil {
			t.Fatalf("seed company: %v", err)
		}
	}
}

func SeedCompany(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID, companyID uuid.UUID, companyType, name string) {
	t.Helper()
	_, err := pool.Exec(ctx, `
		INSERT INTO core.companies (id, tenant_id, company_type, legal_name, status)
		VALUES ($1,$2,$3,$4,'ACTIVE')
		ON CONFLICT DO NOTHING`, companyID, tenantID, companyType, name)
	if err != nil {
		t.Fatalf("seed company: %v", err)
	}
}

func SeedLocations(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID, companyID, originID, destID uuid.UUID) {
	t.Helper()
	for _, row := range []struct {
		id   uuid.UUID
		name string
	}{
		{originID, "Origin WH"},
		{destID, "Destination WH"},
	} {
		_, err := pool.Exec(ctx, `
			INSERT INTO transport.locations (
				id, tenant_id, company_id, location_type, name, country_code, city, status
			) VALUES ($1,$2,$3,'WAREHOUSE',$4,'RU','Moscow','ACTIVE')`,
			row.id, tenantID, companyID, row.name)
		if err != nil {
			t.Fatalf("seed location: %v", err)
		}
	}
}

func SeedUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID, userID uuid.UUID) {
	t.Helper()
	_, err := pool.Exec(ctx, `
		INSERT INTO core.users (id, tenant_id, email, full_name, status)
		VALUES ($1,$2,$3,'Test User','ACTIVE')
		ON CONFLICT DO NOTHING`, userID, tenantID, userID.String()+"@example.test")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
}

func SeedCompanyMembership(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID, userID, companyID uuid.UUID) {
	t.Helper()
	SeedUser(t, ctx, pool, tenantID, userID)
	_, err := pool.Exec(ctx, `
		INSERT INTO core.company_memberships (tenant_id, company_id, user_id, status)
		VALUES ($1,$2,$3,'ACTIVE')
		ON CONFLICT (company_id, user_id) DO UPDATE SET status='ACTIVE', deleted_at=NULL`,
		tenantID, companyID, userID)
	if err != nil {
		t.Fatalf("seed company membership: %v", err)
	}
}

func resolveRoleID(t *testing.T, ctx context.Context, pool *pgxpool.Pool, roleCode string) uuid.UUID {
	t.Helper()
	var roleID uuid.UUID
	err := pool.QueryRow(ctx, `
		INSERT INTO core.roles (code, name, scope, is_system)
		VALUES ($1,$1,'GLOBAL',true)
		ON CONFLICT DO NOTHING
		RETURNING id`, roleCode).Scan(&roleID)
	if err != nil {
		err = pool.QueryRow(ctx, `SELECT id FROM core.roles WHERE code=$1 AND tenant_id IS NULL LIMIT 1`, roleCode).Scan(&roleID)
		if err != nil {
			t.Fatalf("resolve role %s: %v", roleCode, err)
		}
	}
	return roleID
}

func SeedCompanyRole(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID, userID, companyID uuid.UUID, roleCode string) {
	t.Helper()
	SeedCompanyMembership(t, ctx, pool, tenantID, userID, companyID)
	roleID := resolveRoleID(t, ctx, pool, roleCode)
	_, err := pool.Exec(ctx, `
		INSERT INTO core.user_roles (tenant_id, user_id, company_id, role_id)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT DO NOTHING`, tenantID, userID, companyID, roleID)
	if err != nil {
		t.Fatalf("seed company role: %v", err)
	}
}
