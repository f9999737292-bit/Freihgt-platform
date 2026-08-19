package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/billing-register-service/internal/domain"
)

type stubMembershipLookup struct {
	memberships []domain.UserCompanyMembership
	globalRoles []string
}

func (s stubMembershipLookup) ListUserCompanyMemberships(_ context.Context, _, _ uuid.UUID) ([]domain.UserCompanyMembership, error) {
	return s.memberships, nil
}

func (s stubMembershipLookup) ListUserGlobalRoleCodes(_ context.Context, _, _ uuid.UUID) ([]string, error) {
	return s.globalRoles, nil
}

func TestLegacySettlementActorInputAcceptsClientControlledCompanyAndActor(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	userID := uuid.New()
	carrierCompany := uuid.New()
	buyerCompany := uuid.New()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/billing-registers", nil)
	req.Header.Set("X-Tenant-ID", tenantID.String())
	req.Header.Set("X-User-ID", userID.String())
	q := req.URL.Query()
	q.Set("company_id", buyerCompany.String())
	q.Set("actor", domain.SettlementActorBuyer)
	req.URL.RawQuery = q.Encode()

	actor, err := legacySettlementActorInput(req)
	if err != nil {
		t.Fatalf("legacy path unexpectedly failed: %v", err)
	}
	if actor.ActorCompanyID != buyerCompany {
		t.Fatalf("legacy trusted spoofed company_id: got %s want %s", actor.ActorCompanyID, buyerCompany)
	}
	if actor.ActorKind != domain.SettlementActorBuyer {
		t.Fatalf("legacy trusted spoofed actor: got %q", actor.ActorKind)
	}
	if actor.ActorCompanyID == carrierCompany {
		t.Fatal("test setup error")
	}
}

func TestSettlementActorResolverRejectsMissingTrustedHeaders(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	userID := uuid.New()
	companyID := uuid.New()
	resolver := NewSettlementActorResolver(stubMembershipLookup{
		memberships: []domain.UserCompanyMembership{{
			CompanyID: companyID, CompanyType: "CARRIER", RoleCodes: []string{"CARRIER_ADMIN"},
		}},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/billing-registers", nil)
	req.Header.Set("X-Tenant-ID", tenantID.String())
	req.Header.Set("X-User-ID", userID.String())
	q := req.URL.Query()
	q.Set("company_id", companyID.String())
	q.Set("actor", domain.SettlementActorCarrier)
	req.URL.RawQuery = q.Encode()

	_, err := resolver.FromRequest(req)
	if err == nil {
		t.Fatal("expected missing trusted headers to fail closed")
	}
}

func TestSettlementActorResolverRejectsActorKindSpoof(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	userID := uuid.New()
	companyID := uuid.New()
	resolver := NewSettlementActorResolver(stubMembershipLookup{
		memberships: []domain.UserCompanyMembership{{
			CompanyID: companyID, CompanyType: "CARRIER", RoleCodes: []string{"CARRIER_ADMIN"},
		}},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/billing-registers/id/calculate", nil)
	req.Header.Set("X-Tenant-ID", tenantID.String())
	req.Header.Set("X-User-ID", userID.String())
	req.Header.Set(domain.HeaderCompanyID, companyID.String())
	req.Header.Set(domain.HeaderActorKind, domain.SettlementActorBuyer)
	q := req.URL.Query()
	q.Set("company_id", companyID.String())
	q.Set("actor", domain.SettlementActorBuyer)
	req.URL.RawQuery = q.Encode()

	_, err := resolver.FromRequest(req)
	if err == nil {
		t.Fatal("expected carrier membership with buyer actor to be denied")
	}
}

func TestSettlementActorResolverAcceptsVerifiedCompanyContext(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	userID := uuid.New()
	companyID := uuid.New()
	resolver := NewSettlementActorResolver(stubMembershipLookup{
		memberships: []domain.UserCompanyMembership{{
			CompanyID: companyID, CompanyType: "SHIPPER", RoleCodes: []string{"SHIPPER_ADMIN"},
		}},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/billing-registers", nil)
	req.Header.Set("X-Tenant-ID", tenantID.String())
	req.Header.Set("X-User-ID", userID.String())
	req.Header.Set(domain.HeaderCompanyID, companyID.String())
	req.Header.Set(domain.HeaderActorKind, domain.SettlementActorBuyer)
	q := req.URL.Query()
	q.Set("company_id", companyID.String())
	req.URL.RawQuery = q.Encode()

	actor, err := resolver.FromRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actor.ActorCompanyID != companyID || actor.ActorKind != domain.SettlementActorBuyer {
		t.Fatalf("unexpected actor: %+v", actor)
	}
}

func TestSettlementActorResolverRejectsCompanyMembershipSpoof(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	userID := uuid.New()
	companyID := uuid.New()
	foreignCompany := uuid.New()
	resolver := NewSettlementActorResolver(stubMembershipLookup{
		memberships: []domain.UserCompanyMembership{{
			CompanyID: companyID, CompanyType: "CARRIER", RoleCodes: []string{"CARRIER_ADMIN"},
		}},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/billing-registers", nil)
	req.Header.Set("X-Tenant-ID", tenantID.String())
	req.Header.Set("X-User-ID", userID.String())
	req.Header.Set(domain.HeaderCompanyID, foreignCompany.String())
	req.Header.Set(domain.HeaderActorKind, domain.SettlementActorBuyer)
	q := req.URL.Query()
	q.Set("company_id", foreignCompany.String())
	req.URL.RawQuery = q.Encode()

	_, err := resolver.FromRequest(req)
	if err == nil {
		t.Fatal("expected company membership spoof to be denied")
	}
}

func TestSettlementActorResolverPlatformAdminRequiresMembership(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	userID := uuid.New()
	foreignCompany := uuid.New()
	resolver := NewSettlementActorResolver(stubMembershipLookup{
		memberships: nil,
		globalRoles: []string{domain.RolePlatformAdmin},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/billing-registers", nil)
	req.Header.Set("X-Tenant-ID", tenantID.String())
	req.Header.Set("X-User-ID", userID.String())
	req.Header.Set(domain.HeaderCompanyID, foreignCompany.String())
	req.Header.Set(domain.HeaderActorKind, domain.SettlementActorBuyer)
	q := req.URL.Query()
	q.Set("company_id", foreignCompany.String())
	req.URL.RawQuery = q.Encode()

	_, err := resolver.FromRequest(req)
	if err == nil {
		t.Fatal("expected platform admin without membership to be denied")
	}
}
