package service

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/company-service/internal/domain"
	apperrors "github.com/freight-platform/company-service/internal/platform/errors"
)

type mockMembershipAuthorizer struct {
	hasMembership map[uuid.UUID]bool
	companyRoles  map[uuid.UUID][]string
	globalRoles   []string
	companyIDs    []uuid.UUID
}

func (m *mockMembershipAuthorizer) UserHasActiveMembership(_ context.Context, _, _ uuid.UUID, companyID uuid.UUID) (bool, error) {
	return m.hasMembership[companyID], nil
}

func (m *mockMembershipAuthorizer) ListActiveCompanyIDsForUser(_ context.Context, _, _ uuid.UUID) ([]uuid.UUID, error) {
	return append([]uuid.UUID(nil), m.companyIDs...), nil
}

func (m *mockMembershipAuthorizer) ListUserCompanyRoleCodes(_ context.Context, _, _ uuid.UUID, companyID uuid.UUID) ([]string, error) {
	return append([]string(nil), m.companyRoles[companyID]...), nil
}

func (m *mockMembershipAuthorizer) ListUserGlobalRoleCodes(_ context.Context, _, _ uuid.UUID) ([]string, error) {
	return append([]string(nil), m.globalRoles...), nil
}

func TestCompanyAuthorizerCrossCompanyReadDenied(t *testing.T) {
	t.Parallel()

	tenantA := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	userA := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	companyA1 := uuid.MustParse("cccccccc-cccc-cccc-cccc-ccccccccccc1")
	companyA2 := uuid.MustParse("cccccccc-cccc-cccc-cccc-ccccccccccc2")

	authz := NewCompanyAuthorizer(&mockMembershipAuthorizer{
		hasMembership: map[uuid.UUID]bool{companyA1: true, companyA2: false},
		companyRoles:  map[uuid.UUID][]string{companyA1: {"CARRIER_DISPATCHER"}},
	})

	if err := authz.AuthorizeRead(context.Background(), tenantA, userA, companyA1); err != nil {
		t.Fatalf("own company read denied: %v", err)
	}
	if err := authz.AuthorizeRead(context.Background(), tenantA, userA, companyA2); err == nil {
		t.Fatal("expected cross-company read denial")
	} else if appErr, ok := err.(*apperrors.AppError); !ok || appErr.Code != apperrors.CodeNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestCompanyAuthorizerCrossCompanyPatchDenied(t *testing.T) {
	t.Parallel()

	tenantA := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	userA := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	companyA1 := uuid.MustParse("cccccccc-cccc-cccc-cccc-ccccccccccc1")
	companyA2 := uuid.MustParse("cccccccc-cccc-cccc-cccc-ccccccccccc2")

	authz := NewCompanyAuthorizer(&mockMembershipAuthorizer{
		hasMembership: map[uuid.UUID]bool{companyA1: true, companyA2: false},
		companyRoles: map[uuid.UUID][]string{
			companyA1: {"CARRIER_ADMIN"},
			companyA2: {"CARRIER_ADMIN"},
		},
	})

	if err := authz.AuthorizeUpdate(context.Background(), tenantA, userA, companyA1); err != nil {
		t.Fatalf("own company update denied: %v", err)
	}
	if err := authz.AuthorizeUpdate(context.Background(), tenantA, userA, companyA2); err == nil {
		t.Fatal("expected cross-company update denial")
	}
}

func TestCompanyAuthorizerDeleteRequiresPlatformAdmin(t *testing.T) {
	t.Parallel()

	tenantA := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	userA := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")

	authz := NewCompanyAuthorizer(&mockMembershipAuthorizer{globalRoles: nil})
	if err := authz.AuthorizeDelete(context.Background(), tenantA, userA); err == nil {
		t.Fatal("expected delete denial for non-platform admin")
	}

	authzAdmin := NewCompanyAuthorizer(&mockMembershipAuthorizer{globalRoles: []string{domain.RolePlatformAdmin}})
	if err := authzAdmin.AuthorizeDelete(context.Background(), tenantA, userA); err != nil {
		t.Fatalf("platform admin delete denied: %v", err)
	}
}

func TestCompanyAuthorizerCrossTenantCreateDenied(t *testing.T) {
	t.Parallel()

	tenantA := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	tenantB := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	userA := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")

	authz := NewCompanyAuthorizer(&mockMembershipAuthorizer{globalRoles: []string{domain.RolePlatformAdmin}})
	if err := authz.AuthorizeCreate(context.Background(), tenantA, userA, tenantB); err == nil {
		t.Fatal("expected cross-tenant create denial")
	}
}

func TestCompanyAuthorizerListScopedToMemberships(t *testing.T) {
	t.Parallel()

	tenantA := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	userA := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	companyA1 := uuid.MustParse("cccccccc-cccc-cccc-cccc-ccccccccccc1")

	authz := NewCompanyAuthorizer(&mockMembershipAuthorizer{
		companyIDs: []uuid.UUID{companyA1},
	})

	ids, all, err := authz.ListScopeCompanyIDs(context.Background(), tenantA, userA)
	if err != nil {
		t.Fatalf("list scope: %v", err)
	}
	if all {
		t.Fatal("expected membership-scoped list")
	}
	if len(ids) != 1 || ids[0] != companyA1 {
		t.Fatalf("unexpected scope ids: %v", ids)
	}
}
