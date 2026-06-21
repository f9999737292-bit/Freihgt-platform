package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/identity-service/internal/domain"
	apperrors "github.com/freight-platform/identity-service/internal/platform/errors"
)

type mockMembershipStore struct {
	getUserCompaniesFn       func(ctx context.Context, filter domain.ListUserCompaniesFilter) ([]domain.UserCompany, error)
	activeMembershipExistsFn func(ctx context.Context, tenantID, userID, companyID uuid.UUID) (bool, error)
	addCompanyRoleFn         func(ctx context.Context, in domain.AssignCompanyRoleInput) error
	removeCompanyRoleFn      func(ctx context.Context, tenantID, userID, companyID, roleID uuid.UUID) error
}

func (m *mockMembershipStore) GetUserCompanies(ctx context.Context, filter domain.ListUserCompaniesFilter) ([]domain.UserCompany, error) {
	return m.getUserCompaniesFn(ctx, filter)
}

func (m *mockMembershipStore) ActiveMembershipExists(ctx context.Context, tenantID, userID, companyID uuid.UUID) (bool, error) {
	return m.activeMembershipExistsFn(ctx, tenantID, userID, companyID)
}

func (m *mockMembershipStore) AddCompanyRoleToUser(ctx context.Context, in domain.AssignCompanyRoleInput) error {
	return m.addCompanyRoleFn(ctx, in)
}

func (m *mockMembershipStore) RemoveCompanyRoleFromUser(ctx context.Context, tenantID, userID, companyID, roleID uuid.UUID) error {
	return m.removeCompanyRoleFn(ctx, tenantID, userID, companyID, roleID)
}

type mockRoleStoreForMembership struct {
	companyExistsFn     func(ctx context.Context, companyID, tenantID uuid.UUID) (bool, error)
	roleAvailableForFn  func(ctx context.Context, roleID, tenantID uuid.UUID) (bool, error)
}

func (m *mockRoleStoreForMembership) ListByTenant(context.Context, uuid.UUID) ([]domain.Role, error) {
	return nil, nil
}

func (m *mockRoleStoreForMembership) GetByID(context.Context, uuid.UUID) (*domain.Role, error) {
	return nil, nil
}

func (m *mockRoleStoreForMembership) RoleAvailableForTenant(ctx context.Context, roleID, tenantID uuid.UUID) (bool, error) {
	return m.roleAvailableForFn(ctx, roleID, tenantID)
}

func (m *mockRoleStoreForMembership) CompanyExists(ctx context.Context, companyID, tenantID uuid.UUID) (bool, error) {
	return m.companyExistsFn(ctx, companyID, tenantID)
}

func (m *mockRoleStoreForMembership) AssignRole(context.Context, domain.AssignRoleInput) error {
	return nil
}

func (m *mockRoleStoreForMembership) ListUserRoles(context.Context, uuid.UUID, uuid.UUID) ([]domain.UserRoleAssignment, error) {
	return nil, nil
}

type mockUserStoreForMembership struct {
	getByIDFn func(ctx context.Context, id uuid.UUID) (*domain.User, error)
}

func (m *mockUserStoreForMembership) Create(context.Context, domain.CreateUserInput, string) (*domain.User, error) {
	return nil, nil
}

func (m *mockUserStoreForMembership) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	return m.getByIDFn(ctx, id)
}

func (m *mockUserStoreForMembership) List(context.Context, domain.ListUsersFilter) ([]domain.User, int, error) {
	return nil, 0, nil
}

func (m *mockUserStoreForMembership) Update(context.Context, uuid.UUID, domain.UpdateUserInput) (*domain.User, error) {
	return nil, nil
}

func (m *mockUserStoreForMembership) SoftDelete(context.Context, uuid.UUID) error {
	return nil
}

func TestMembershipServiceGetUserCompanies(t *testing.T) {
	t.Parallel()

	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	userID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	companyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	svc := NewMembershipService(&mockUserStoreForMembership{
		getByIDFn: func(context.Context, uuid.UUID) (*domain.User, error) {
			return &domain.User{ID: userID, TenantID: tenantID}, nil
		},
	}, &mockRoleStoreForMembership{}, &mockMembershipStore{
		getUserCompaniesFn: func(_ context.Context, filter domain.ListUserCompaniesFilter) ([]domain.UserCompany, error) {
			if filter.UserID != userID {
				t.Fatalf("unexpected user id")
			}
			return []domain.UserCompany{{
				MembershipID:     uuid.New(),
				CompanyID:        companyID,
				LegalName:        "ООО Ромашка",
				CompanyType:      "SHIPPER",
				MembershipStatus: domain.MembershipStatusActive,
			}}, nil
		},
		activeMembershipExistsFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (bool, error) {
			return true, nil
		},
		addCompanyRoleFn: func(context.Context, domain.AssignCompanyRoleInput) error {
			return nil
		},
		removeCompanyRoleFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) error {
			return nil
		},
	})

	companies, err := svc.GetUserCompanies(context.Background(), domain.ListUserCompaniesFilter{
		TenantID: tenantID,
		UserID:   userID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(companies) != 1 {
		t.Fatalf("expected 1 company, got %d", len(companies))
	}
}

func TestMembershipServiceAddCompanyRoleToUser(t *testing.T) {
	t.Parallel()

	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	userID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	companyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	roleID := uuid.MustParse("44444444-4444-4444-4444-444444444444")

	called := false
	svc := NewMembershipService(&mockUserStoreForMembership{
		getByIDFn: func(context.Context, uuid.UUID) (*domain.User, error) {
			return &domain.User{ID: userID, TenantID: tenantID}, nil
		},
	}, &mockRoleStoreForMembership{
		companyExistsFn: func(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
			return true, nil
		},
		roleAvailableForFn: func(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
			return true, nil
		},
	}, &mockMembershipStore{
		getUserCompaniesFn: func(context.Context, domain.ListUserCompaniesFilter) ([]domain.UserCompany, error) {
			return nil, nil
		},
		activeMembershipExistsFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (bool, error) {
			return true, nil
		},
		addCompanyRoleFn: func(_ context.Context, in domain.AssignCompanyRoleInput) error {
			called = true
			if in.RoleID != roleID {
				t.Fatalf("unexpected role id")
			}
			return nil
		},
		removeCompanyRoleFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) error {
			return nil
		},
	})

	err := svc.AddCompanyRoleToUser(context.Background(), domain.AssignCompanyRoleInput{
		TenantID:  tenantID,
		UserID:    userID,
		CompanyID: companyID,
		RoleID:    roleID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatalf("expected AddCompanyRoleToUser to be called")
	}
}

func TestMembershipServiceAddCompanyRoleRequiresActiveMembership(t *testing.T) {
	t.Parallel()

	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	userID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	companyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	roleID := uuid.MustParse("44444444-4444-4444-4444-444444444444")

	svc := NewMembershipService(&mockUserStoreForMembership{
		getByIDFn: func(context.Context, uuid.UUID) (*domain.User, error) {
			return &domain.User{ID: userID, TenantID: tenantID}, nil
		},
	}, &mockRoleStoreForMembership{
		companyExistsFn: func(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
			return true, nil
		},
		roleAvailableForFn: func(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
			return true, nil
		},
	}, &mockMembershipStore{
		getUserCompaniesFn: func(context.Context, domain.ListUserCompaniesFilter) ([]domain.UserCompany, error) {
			return nil, nil
		},
		activeMembershipExistsFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (bool, error) {
			return false, nil
		},
		addCompanyRoleFn: func(context.Context, domain.AssignCompanyRoleInput) error {
			return nil
		},
		removeCompanyRoleFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) error {
			return nil
		},
	})

	err := svc.AddCompanyRoleToUser(context.Background(), domain.AssignCompanyRoleInput{
		TenantID:  tenantID,
		UserID:    userID,
		CompanyID: companyID,
		RoleID:    roleID,
	})
	if err == nil {
		t.Fatalf("expected not found error")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}
