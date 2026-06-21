package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/company-service/internal/domain"
	apperrors "github.com/freight-platform/company-service/internal/platform/errors"
)

type mockMembershipStore struct {
	userExistsFn         func(ctx context.Context, userID, tenantID uuid.UUID) (bool, error)
	roleAvailableFn      func(ctx context.Context, roleID, tenantID uuid.UUID) (bool, error)
	getByCompanyAndUserFn func(ctx context.Context, companyID, userID uuid.UUID) (*domain.Membership, *time.Time, error)
	createFn             func(ctx context.Context, in domain.CreateMembershipInput) (*domain.Membership, error)
	getCompanyMembersFn  func(ctx context.Context, filter domain.ListCompanyMembersFilter) ([]domain.CompanyMember, int, error)
}

func (m *mockMembershipStore) UserExistsInTenant(ctx context.Context, userID, tenantID uuid.UUID) (bool, error) {
	return m.userExistsFn(ctx, userID, tenantID)
}

func (m *mockMembershipStore) RoleAvailableForTenant(ctx context.Context, roleID, tenantID uuid.UUID) (bool, error) {
	return m.roleAvailableFn(ctx, roleID, tenantID)
}

func (m *mockMembershipStore) GetMembershipByCompanyAndUser(ctx context.Context, companyID, userID uuid.UUID) (*domain.Membership, *time.Time, error) {
	return m.getByCompanyAndUserFn(ctx, companyID, userID)
}

func (m *mockMembershipStore) CreateMembership(ctx context.Context, in domain.CreateMembershipInput) (*domain.Membership, error) {
	return m.createFn(ctx, in)
}

func (m *mockMembershipStore) ReactivateMembership(context.Context, uuid.UUID, *string, int) (*domain.Membership, error) {
	return nil, nil
}

func (m *mockMembershipStore) GetMembershipByID(context.Context, uuid.UUID, uuid.UUID) (*domain.Membership, error) {
	return nil, nil
}

func (m *mockMembershipStore) GetCompanyMembers(ctx context.Context, filter domain.ListCompanyMembersFilter) ([]domain.CompanyMember, int, error) {
	return m.getCompanyMembersFn(ctx, filter)
}

func (m *mockMembershipStore) UpdateMembership(context.Context, uuid.UUID, uuid.UUID, domain.UpdateMembershipInput) (*domain.Membership, error) {
	return nil, nil
}

func (m *mockMembershipStore) SoftDeleteMembership(context.Context, uuid.UUID, uuid.UUID) (*domain.Membership, error) {
	return nil, nil
}

func (m *mockMembershipStore) AddUserRoleForCompany(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) error {
	return nil
}

func (m *mockMembershipStore) RemoveUserRolesForCompany(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
	return nil
}

func TestMembershipServiceAddMember(t *testing.T) {
	t.Parallel()

	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	companyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	userID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	svc := NewMembershipService(&mockCompanyStore{
		getByIDFn: func(context.Context, uuid.UUID) (*domain.Company, error) {
			return &domain.Company{ID: companyID, TenantID: tenantID}, nil
		},
	}, &mockMembershipStore{
		userExistsFn: func(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
			return true, nil
		},
		roleAvailableFn: func(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
			return true, nil
		},
		getByCompanyAndUserFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Membership, *time.Time, error) {
			return nil, nil, apperrors.NotFound("membership not found")
		},
		createFn: func(_ context.Context, in domain.CreateMembershipInput) (*domain.Membership, error) {
			return &domain.Membership{
				ID:        uuid.New(),
				TenantID:  in.TenantID,
				CompanyID: in.CompanyID,
				UserID:    in.UserID,
				Status:    domain.MembershipStatusActive,
				CreatedAt: time.Now().UTC().Format(time.RFC3339),
			}, nil
		},
		getCompanyMembersFn: func(context.Context, domain.ListCompanyMembersFilter) ([]domain.CompanyMember, int, error) {
			return nil, 0, nil
		},
	})

	position := "Логист"
	membership, err := svc.AddMember(context.Background(), domain.CreateMembershipInput{
		TenantID:  tenantID,
		CompanyID: companyID,
		UserID:    userID,
		Position:  &position,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if membership.Status != domain.MembershipStatusActive {
		t.Fatalf("unexpected status: %s", membership.Status)
	}
}

func TestMembershipServiceAddMemberDuplicateConflict(t *testing.T) {
	t.Parallel()

	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	companyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	userID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	svc := NewMembershipService(&mockCompanyStore{
		getByIDFn: func(context.Context, uuid.UUID) (*domain.Company, error) {
			return &domain.Company{ID: companyID, TenantID: tenantID}, nil
		},
	}, &mockMembershipStore{
		userExistsFn: func(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
			return true, nil
		},
		roleAvailableFn: func(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
			return true, nil
		},
		getByCompanyAndUserFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Membership, *time.Time, error) {
			return &domain.Membership{ID: uuid.New()}, nil, nil
		},
		createFn: func(context.Context, domain.CreateMembershipInput) (*domain.Membership, error) {
			return nil, errors.New("should not create")
		},
		getCompanyMembersFn: func(context.Context, domain.ListCompanyMembersFilter) ([]domain.CompanyMember, int, error) {
			return nil, 0, nil
		},
	})

	_, err := svc.AddMember(context.Background(), domain.CreateMembershipInput{
		TenantID:  tenantID,
		CompanyID: companyID,
		UserID:    userID,
	})
	if err == nil {
		t.Fatalf("expected conflict error")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeConflict {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestMembershipServiceListMembers(t *testing.T) {
	t.Parallel()

	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	companyID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	userID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	svc := NewMembershipService(&mockCompanyStore{
		getByIDFn: func(context.Context, uuid.UUID) (*domain.Company, error) {
			return &domain.Company{ID: companyID, TenantID: tenantID}, nil
		},
	}, &mockMembershipStore{
		userExistsFn: func(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
			return true, nil
		},
		roleAvailableFn: func(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
			return true, nil
		},
		getByCompanyAndUserFn: func(context.Context, uuid.UUID, uuid.UUID) (*domain.Membership, *time.Time, error) {
			return nil, nil, apperrors.NotFound("membership not found")
		},
		createFn: func(context.Context, domain.CreateMembershipInput) (*domain.Membership, error) {
			return nil, nil
		},
		getCompanyMembersFn: func(_ context.Context, filter domain.ListCompanyMembersFilter) ([]domain.CompanyMember, int, error) {
			if filter.Limit != 20 {
				t.Fatalf("expected default limit 20, got %d", filter.Limit)
			}
			return []domain.CompanyMember{{
				MembershipID: uuid.New(),
				UserID:       userID,
				Email:        "user@example.com",
				FullName:     "Иван Иванов",
				Status:       domain.MembershipStatusActive,
			}}, 1, nil
		},
	})

	members, _, err := svc.ListMembers(context.Background(), domain.ListCompanyMembersFilter{
		TenantID:  tenantID,
		CompanyID: companyID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(members))
	}
}
