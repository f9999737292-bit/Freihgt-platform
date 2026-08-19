package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/payment-service/internal/domain"
)

type stubMembership struct {
	memberships []domain.UserCompanyMembership
	globalRoles []string
}

func (s stubMembership) ListUserCompanyMemberships(_ context.Context, _, _ uuid.UUID) ([]domain.UserCompanyMembership, error) {
	return s.memberships, nil
}

func (s stubMembership) ListUserGlobalRoleCodes(_ context.Context, _, _ uuid.UUID) ([]string, error) {
	return s.globalRoles, nil
}

func TestPaymentActorResolverTrustedHeaders(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	userID := uuid.New()
	companyID := uuid.New()
	resolver := NewPaymentActorResolver(stubMembership{
		memberships: []domain.UserCompanyMembership{{CompanyID: companyID, CompanyType: "SHIPPER"}},
	})

	t.Run("NO_TRUSTED_HEADERS=DENY", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/payment-obligations", nil)
		if _, err := resolver.FromRequest(req); err == nil {
			t.Fatal("expected missing headers to deny")
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/payment-obligations", nil)
	req.Header.Set(domain.HeaderTenantID, tenantID.String())
	req.Header.Set(domain.HeaderUserID, userID.String())
	req.Header.Set(domain.HeaderCompanyID, companyID.String())
	req.Header.Set(domain.HeaderActorKind, domain.PaymentActorBuyer)
	actor, err := resolver.FromRequest(req)
	if err != nil {
		t.Fatalf("valid trusted headers expected pass, got %v", err)
	}
	if actor.TenantID != tenantID || actor.ActorCompanyID != companyID {
		t.Fatal("actor context mismatch")
	}
}

func TestPlatformAdminWithoutMembershipDenied(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	userID := uuid.New()
	companyID := uuid.New()
	otherCompany := uuid.New()
	resolver := NewPaymentActorResolver(stubMembership{
		memberships: []domain.UserCompanyMembership{{CompanyID: otherCompany, CompanyType: "SHIPPER"}},
		globalRoles: []string{"PLATFORM_ADMIN"},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/payment-obligations", nil)
	req.Header.Set(domain.HeaderTenantID, tenantID.String())
	req.Header.Set(domain.HeaderUserID, userID.String())
	req.Header.Set(domain.HeaderCompanyID, companyID.String())
	req.Header.Set(domain.HeaderActorKind, domain.PaymentActorBuyer)
	if _, err := resolver.FromRequest(req); err == nil {
		t.Fatal("PLATFORM_ADMIN_NO_MEMBERSHIP=DENY expected")
	}
}
