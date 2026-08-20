package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestRequireManualSpotPermissionCompanyScopedAllow(t *testing.T) {
	actor := ActorInput{ActorKind: ActorKindBuyer}
	if err := actor.RequireManualSpotPermission(true, false); err != nil {
		t.Fatalf("expected allow, got %v", err)
	}
}

func TestRequireManualSpotPermissionOtherCompanyRoleDenied(t *testing.T) {
	actor := ActorInput{ActorKind: ActorKindBuyer}
	if err := actor.RequireManualSpotPermission(false, false); err == nil {
		t.Fatal("expected deny when permission belongs to another company scope")
	}
}

func TestRequireManualSpotPermissionTenantPlatformAdminAllow(t *testing.T) {
	actor := ActorInput{ActorKind: ActorKindBuyer}
	if err := actor.RequireManualSpotPermission(false, true); err != nil {
		t.Fatalf("expected tenant platform admin allow, got %v", err)
	}
}

func TestRequireManualSpotPermissionCarrierDenied(t *testing.T) {
	actor := ActorInput{ActorKind: ActorKindCarrier}
	if err := actor.RequireManualSpotPermission(true, true); err == nil {
		t.Fatal("expected carrier deny even with permission flags")
	}
}

func TestRequireManualSpotPermissionIgnoresClientPlatformAdminFlag(t *testing.T) {
	actor := ActorInput{ActorKind: ActorKindBuyer, IsPlatformAdmin: true}
	if err := actor.RequireManualSpotPermission(false, false); err == nil {
		t.Fatal("must not trust client-provided IsPlatformAdmin without tenant-global verification")
	}
}

func TestHasPlatformAdminRoleTenantGlobalOnly(t *testing.T) {
	if !HasPlatformAdminRole([]string{"PLATFORM_ADMIN"}) {
		t.Fatal("expected tenant-global platform admin detection")
	}
	if HasPlatformAdminRole([]string{"SHIPPER_ADMIN"}) {
		t.Fatal("shipper admin must not imply platform admin")
	}
}

func TestResolveTrustedActorRejectsForgedCompanyMembership(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	companyA := uuid.New()
	forgedCompany := uuid.New()
	memberships := []UserCompanyMembership{
		{CompanyID: companyA, CompanyType: "SHIPPER", RoleCodes: []string{"SHIPPER_LOGIST"}},
	}
	_, err := ResolveTrustedActor(tenantID, userID, forgedCompany, ActorKindBuyer, memberships, false)
	if err == nil {
		t.Fatal("expected forged company_id to be rejected")
	}
}

func TestResolveTrustedActorCompanyScopedPlatformAdminNotTenantGlobal(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	companyA := uuid.New()
	memberships := []UserCompanyMembership{
		{CompanyID: companyA, CompanyType: "SHIPPER", RoleCodes: []string{RolePlatformAdmin}},
	}
	actor, err := ResolveTrustedActor(tenantID, userID, companyA, ActorKindBuyer, memberships, false)
	if err != nil {
		t.Fatalf("resolve actor: %v", err)
	}
	if actor.IsPlatformAdmin {
		t.Fatal("company-scoped PLATFORM_ADMIN role must not grant tenant-global platform admin")
	}
}
