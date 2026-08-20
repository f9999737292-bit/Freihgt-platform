//go:build integration

package contractrate

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/contract-rate-service/internal/domain"
	apperrors "github.com/freight-platform/contract-rate-service/internal/platform/errors"
)

func TestCRBAUTH001_CrossCompanyRoleBleedDenied(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	userID := uuid.New()
	companyB := uuid.New()
	seedBuyerCompany(t, ctx, env.Pool, env.TenantID, companyB, "Buyer B")
	seedCompanyRole(t, ctx, env.Pool, env.TenantID, userID, env.BuyerID, "PROCUREMENT_MANAGER")
	seedCompanyMembership(t, ctx, env.Pool, env.TenantID, userID, companyB)

	actor := domain.ActorInput{
		TenantID: env.TenantID, ActorUserID: userID, ActorCompanyID: companyB, ActorKind: domain.ActorKindBuyer,
	}
	_, err := resolveManualSpot(t, env, actor)
	if !isAppErrorCode(err, apperrors.CodeForbidden) {
		t.Fatalf("CR-B-AUTH-001: expected deny, got %v", err)
	}
}

func TestCRBAUTH002_SameCompanyProcurementManagerAllowed(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	userID := uuid.New()
	seedCompanyRole(t, ctx, env.Pool, env.TenantID, userID, env.BuyerID, "PROCUREMENT_MANAGER")
	actor := domain.ActorInput{
		TenantID: env.TenantID, ActorUserID: userID, ActorCompanyID: env.BuyerID, ActorKind: domain.ActorKindBuyer,
	}
	result, err := resolveManualSpot(t, env, actor)
	if err != nil || result.PricingSource != domain.PricingSourceManualSpot {
		t.Fatalf("CR-B-AUTH-002: expected allow, got status=%s err=%v", result.Status, err)
	}
}

func TestCRBAUTH003_ShipperAdminAllowedWhenPermitted(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	userID := uuid.New()
	seedCompanyRole(t, ctx, env.Pool, env.TenantID, userID, env.BuyerID, "SHIPPER_ADMIN")
	actor := domain.ActorInput{
		TenantID: env.TenantID, ActorUserID: userID, ActorCompanyID: env.BuyerID, ActorKind: domain.ActorKindBuyer,
	}
	result, err := resolveManualSpot(t, env, actor)
	if err != nil || result.PricingSource != domain.PricingSourceManualSpot {
		t.Fatalf("CR-B-AUTH-003: expected allow, got status=%s err=%v", result.Status, err)
	}
}

func TestCRBAUTH004_RoleWithoutPermissionDenied(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	userID := uuid.New()
	seedCompanyRole(t, ctx, env.Pool, env.TenantID, userID, env.BuyerID, "SHIPPER_LOGIST")
	actor := domain.ActorInput{
		TenantID: env.TenantID, ActorUserID: userID, ActorCompanyID: env.BuyerID, ActorKind: domain.ActorKindBuyer,
	}
	_, err := resolveManualSpot(t, env, actor)
	if !isAppErrorCode(err, apperrors.CodeForbidden) {
		t.Fatalf("CR-B-AUTH-004: expected deny, got %v", err)
	}
}

func TestCRBAUTH005_CarrierRoleDenied(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	userID := uuid.New()
	seedCompanyRole(t, ctx, env.Pool, env.TenantID, userID, env.CarrierID, "CARRIER_ADMIN")
	actor := domain.ActorInput{
		TenantID: env.TenantID, ActorUserID: userID, ActorCompanyID: env.CarrierID, ActorKind: domain.ActorKindCarrier,
	}
	_, err := resolveManualSpot(t, env, actor)
	if !isAppErrorCode(err, apperrors.CodeForbidden) {
		t.Fatalf("CR-B-AUTH-005: expected deny, got %v", err)
	}
}

func TestCRBAUTH006_OtherCompanyRoleSameTenantDenied(t *testing.T) {
	TestCRBAUTH001_CrossCompanyRoleBleedDenied(t)
}

func TestCRBAUTH007_OtherTenantRoleDenied(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	otherTenant := uuid.New()
	seedTenantOnly(t, ctx, env.Pool, otherTenant)
	userID := uuid.New()
	otherBuyer := uuid.New()
	seedBuyerCompany(t, ctx, env.Pool, otherTenant, otherBuyer, "Other Tenant Buyer")
	seedCompanyRole(t, ctx, env.Pool, otherTenant, userID, otherBuyer, "PROCUREMENT_MANAGER")
	seedCompanyMembership(t, ctx, env.Pool, env.TenantID, userID, env.BuyerID)

	actor := domain.ActorInput{
		TenantID: env.TenantID, ActorUserID: userID, ActorCompanyID: env.BuyerID, ActorKind: domain.ActorKindBuyer,
	}
	_, err := resolveManualSpot(t, env, actor)
	if !isAppErrorCode(err, apperrors.CodeForbidden) {
		t.Fatalf("CR-B-AUTH-007: expected deny, got %v", err)
	}
}

func TestCRBAUTH008_ForgedCompanyMembershipDenied(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	userID := uuid.New()
	forgedCompany := uuid.New()
	seedBuyerCompany(t, ctx, env.Pool, env.TenantID, forgedCompany, "Forged Buyer")
	seedCompanyRole(t, ctx, env.Pool, env.TenantID, userID, env.BuyerID, "PROCUREMENT_MANAGER")

	memberships, err := env.Memberships.ListUserCompanyMemberships(ctx, env.TenantID, userID)
	if err != nil {
		t.Fatalf("memberships: %v", err)
	}
	_, err = domain.ResolveTrustedActor(env.TenantID, userID, forgedCompany, domain.ActorKindBuyer, memberships, false)
	if !isAppErrorCode(err, apperrors.CodeForbidden) {
		t.Fatalf("CR-B-AUTH-008: expected forged company deny, got %v", err)
	}
}

func TestCRBAUTH009_MembershipWithoutMatchingCompanyPermissionDenied(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	userID := uuid.New()
	companyB := uuid.New()
	seedBuyerCompany(t, ctx, env.Pool, env.TenantID, companyB, "Buyer B")
	seedCompanyRole(t, ctx, env.Pool, env.TenantID, userID, env.BuyerID, "PROCUREMENT_MANAGER")
	seedCompanyMembership(t, ctx, env.Pool, env.TenantID, userID, companyB)

	allowed, err := env.Memberships.HasCompanyPermission(ctx, env.TenantID, userID, companyB, domain.PermissionManualSpotUse)
	if err != nil {
		t.Fatalf("permission lookup: %v", err)
	}
	if allowed {
		t.Fatal("CR-B-AUTH-009: permission must not bleed to company B")
	}

	actor := domain.ActorInput{
		TenantID: env.TenantID, ActorUserID: userID, ActorCompanyID: companyB, ActorKind: domain.ActorKindBuyer,
	}
	_, err = resolveManualSpot(t, env, actor)
	if !isAppErrorCode(err, apperrors.CodeForbidden) {
		t.Fatalf("CR-B-AUTH-009: expected deny, got %v", err)
	}
}

func TestCRBAUTH010_TenantGlobalPlatformAdminAllowed(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	userID := uuid.New()
	seedTenantRole(t, ctx, env.Pool, env.TenantID, userID, domain.RolePlatformAdmin)
	seedCompanyMembership(t, ctx, env.Pool, env.TenantID, userID, env.BuyerID)
	actor := domain.ActorInput{
		TenantID: env.TenantID, ActorUserID: userID, ActorCompanyID: env.BuyerID, ActorKind: domain.ActorKindBuyer,
	}
	result, err := resolveManualSpot(t, env, actor)
	if err != nil || result.PricingSource != domain.PricingSourceManualSpot {
		t.Fatalf("CR-B-AUTH-010: expected allow, got status=%s err=%v", result.Status, err)
	}
}

func TestCRBAUTH011_CompanyScopedPlatformAdminNotTenantGlobal(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	userID := uuid.New()
	companyB := uuid.New()
	seedBuyerCompany(t, ctx, env.Pool, env.TenantID, companyB, "Buyer B")
	seedCompanyRole(t, ctx, env.Pool, env.TenantID, userID, env.BuyerID, domain.RolePlatformAdmin)
	seedCompanyMembership(t, ctx, env.Pool, env.TenantID, userID, companyB)

	tenantRoles, err := env.Memberships.ListUserTenantRoleCodes(ctx, env.TenantID, userID)
	if err != nil {
		t.Fatalf("tenant roles: %v", err)
	}
	if domain.HasPlatformAdminRole(tenantRoles) {
		t.Fatal("CR-B-AUTH-011: company-scoped PLATFORM_ADMIN must not become tenant-global admin")
	}

	actor := domain.ActorInput{
		TenantID: env.TenantID, ActorUserID: userID, ActorCompanyID: companyB, ActorKind: domain.ActorKindBuyer,
	}
	_, err = resolveManualSpot(t, env, actor)
	if !isAppErrorCode(err, apperrors.CodeForbidden) {
		t.Fatalf("CR-B-AUTH-011: expected deny without tenant-global admin, got %v", err)
	}
}

func TestCRBAUTH012_InactiveMembershipDenied(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	userID := uuid.New()
	seedCompanyRole(t, ctx, env.Pool, env.TenantID, userID, env.BuyerID, "PROCUREMENT_MANAGER")
	_, err := env.Pool.Exec(ctx, `
		UPDATE core.company_memberships
		SET status='DISABLED'
		WHERE tenant_id=$1 AND user_id=$2 AND company_id=$3`, env.TenantID, userID, env.BuyerID)
	if err != nil {
		t.Fatalf("disable membership: %v", err)
	}

	actor := domain.ActorInput{
		TenantID: env.TenantID, ActorUserID: userID, ActorCompanyID: env.BuyerID, ActorKind: domain.ActorKindBuyer,
	}
	_, err = resolveManualSpot(t, env, actor)
	if !isAppErrorCode(err, apperrors.CodeForbidden) {
		t.Fatalf("CR-B-AUTH-012: expected inactive membership deny, got %v", err)
	}
}

func TestCRBAUTH012_DeletedRoleDenied(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	userID := uuid.New()
	seedCompanyRole(t, ctx, env.Pool, env.TenantID, userID, env.BuyerID, "PROCUREMENT_MANAGER")
	_, err := env.Pool.Exec(ctx, `
		UPDATE core.roles
		SET deleted_at=now()
		WHERE code='PROCUREMENT_MANAGER' AND tenant_id IS NULL`)
	if err != nil {
		t.Fatalf("delete role: %v", err)
	}

	actor := domain.ActorInput{
		TenantID: env.TenantID, ActorUserID: userID, ActorCompanyID: env.BuyerID, ActorKind: domain.ActorKindBuyer,
	}
	_, err = resolveManualSpot(t, env, actor)
	if !isAppErrorCode(err, apperrors.CodeForbidden) {
		t.Fatalf("CR-B-AUTH-012: expected deleted role deny, got %v", err)
	}
}

func TestCRBAUTHPermissionRegistry(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()

	var permCount int
	err := env.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM core.permissions WHERE code=$1`, domain.PermissionManualSpotUse).Scan(&permCount)
	if err != nil {
		t.Fatalf("permission count: %v", err)
	}
	if permCount != 1 {
		t.Fatalf("expected permission registered exactly once, got %d", permCount)
	}

	for _, roleCode := range []string{"PROCUREMENT_MANAGER", "SHIPPER_ADMIN", "FORWARDER_MANAGER"} {
		var rpCount int
		err := env.Pool.QueryRow(ctx, `
			SELECT COUNT(*)
			FROM core.role_permissions rp
			JOIN core.roles r ON r.id = rp.role_id
			JOIN core.permissions p ON p.id = rp.permission_id
			WHERE r.code=$1 AND r.tenant_id IS NULL AND p.code=$2`,
			roleCode, domain.PermissionManualSpotUse).Scan(&rpCount)
		if err != nil {
			t.Fatalf("role permission count for %s: %v", roleCode, err)
		}
		if rpCount != 1 {
			t.Fatalf("expected one role_permission row for %s, got %d", roleCode, rpCount)
		}
	}

	for _, roleCode := range []string{"CARRIER_ADMIN", "CARRIER_DISPATCHER", "SHIPPER_LOGIST"} {
		var rpCount int
		err := env.Pool.QueryRow(ctx, `
			SELECT COUNT(*)
			FROM core.role_permissions rp
			JOIN core.roles r ON r.id = rp.role_id
			JOIN core.permissions p ON p.id = rp.permission_id
			WHERE r.code=$1 AND r.tenant_id IS NULL AND p.code=$2`,
			roleCode, domain.PermissionManualSpotUse).Scan(&rpCount)
		if err != nil {
			t.Fatalf("carrier/logist role permission count for %s: %v", roleCode, err)
		}
		if rpCount != 0 {
			t.Fatalf("carrier/logist role %s must not receive manual spot permission", roleCode)
		}
	}
}

func TestCRBAUTHUnauthorizedManualSpotDoesNotAudit(t *testing.T) {
	env := setupEnv(t)
	ctx := context.Background()
	userID := uuid.New()
	companyB := uuid.New()
	seedBuyerCompany(t, ctx, env.Pool, env.TenantID, companyB, "Buyer B")
	seedCompanyRole(t, ctx, env.Pool, env.TenantID, userID, env.BuyerID, "PROCUREMENT_MANAGER")
	seedCompanyMembership(t, ctx, env.Pool, env.TenantID, userID, companyB)

	actor := domain.ActorInput{
		TenantID: env.TenantID, ActorUserID: userID, ActorCompanyID: companyB, ActorKind: domain.ActorKindBuyer,
	}
	_, err := resolveManualSpot(t, env, actor)
	if !isAppErrorCode(err, apperrors.CodeForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}

	var count int
	err = env.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM contract_rate.audit_event
		WHERE tenant_id=$1 AND action=$2`, env.TenantID, domain.AuditActionManualSpotResolved).Scan(&count)
	if err != nil {
		t.Fatalf("audit count: %v", err)
	}
	if count != 0 {
		t.Fatalf("unauthorized manual spot must not emit MANUAL_SPOT_RESOLVED audit, got %d", count)
	}
}
