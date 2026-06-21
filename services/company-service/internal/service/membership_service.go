package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/company-service/internal/domain"
	apperrors "github.com/freight-platform/company-service/internal/platform/errors"
)

type MembershipStore interface {
	UserExistsInTenant(ctx context.Context, userID, tenantID uuid.UUID) (bool, error)
	RoleAvailableForTenant(ctx context.Context, roleID, tenantID uuid.UUID) (bool, error)
	GetMembershipByCompanyAndUser(ctx context.Context, companyID, userID uuid.UUID) (*domain.Membership, *time.Time, error)
	CreateMembership(ctx context.Context, in domain.CreateMembershipInput) (*domain.Membership, error)
	ReactivateMembership(ctx context.Context, membershipID uuid.UUID, position *string, version int) (*domain.Membership, error)
	GetMembershipByID(ctx context.Context, membershipID, companyID uuid.UUID) (*domain.Membership, error)
	GetCompanyMembers(ctx context.Context, filter domain.ListCompanyMembersFilter) ([]domain.CompanyMember, int, error)
	UpdateMembership(ctx context.Context, membershipID, companyID uuid.UUID, in domain.UpdateMembershipInput) (*domain.Membership, error)
	SoftDeleteMembership(ctx context.Context, membershipID, companyID uuid.UUID) (*domain.Membership, error)
	AddUserRoleForCompany(ctx context.Context, tenantID, userID, companyID, roleID uuid.UUID) error
	RemoveUserRolesForCompany(ctx context.Context, tenantID, userID, companyID uuid.UUID) error
}

type MembershipService struct {
	companies   CompanyStore
	memberships MembershipStore
}

func NewMembershipService(companies CompanyStore, memberships MembershipStore) *MembershipService {
	return &MembershipService{companies: companies, memberships: memberships}
}

func (s *MembershipService) AddMember(ctx context.Context, in domain.CreateMembershipInput) (*domain.Membership, error) {
	if err := domain.ValidateCreateMembershipInput(in); err != nil {
		return nil, err
	}

	company, err := s.companies.GetByID(ctx, in.CompanyID)
	if err != nil {
		return nil, err
	}
	if company.TenantID != in.TenantID {
		return nil, apperrors.Validation("company does not belong to tenant", map[string]any{"field": "tenant_id"})
	}

	userExists, err := s.memberships.UserExistsInTenant(ctx, in.UserID, in.TenantID)
	if err != nil {
		return nil, err
	}
	if !userExists {
		return nil, apperrors.NotFound("user not found")
	}

	if in.RoleID != nil {
		available, err := s.memberships.RoleAvailableForTenant(ctx, *in.RoleID, in.TenantID)
		if err != nil {
			return nil, err
		}
		if !available {
			return nil, apperrors.NotFound("role not found")
		}
	}

	var result *domain.Membership
	membership, deletedAt, err := s.memberships.GetMembershipByCompanyAndUser(ctx, in.CompanyID, in.UserID)
	if err == nil {
		if deletedAt == nil {
			return nil, apperrors.Conflict("membership already exists", map[string]any{
				"company_id": in.CompanyID.String(),
				"user_id":    in.UserID.String(),
			})
		}
		result, err = s.memberships.ReactivateMembership(ctx, membership.ID, in.Position, membership.Version)
	} else {
		var notFound *apperrors.AppError
		if !errors.As(err, &notFound) || notFound.Code != apperrors.CodeNotFound {
			return nil, err
		}
		result, err = s.memberships.CreateMembership(ctx, in)
	}
	if err != nil {
		return nil, err
	}

	if in.RoleID != nil {
		if err := s.memberships.AddUserRoleForCompany(ctx, in.TenantID, in.UserID, in.CompanyID, *in.RoleID); err != nil {
			return nil, err
		}
	}

	return result, nil
}

func (s *MembershipService) ListMembers(ctx context.Context, filter domain.ListCompanyMembersFilter) ([]domain.CompanyMember, int, error) {
	if filter.Limit == 0 {
		filter.Limit = 20
	}
	if err := domain.ValidateListCompanyMembersFilter(filter); err != nil {
		return nil, 0, err
	}
	if _, err := s.companies.GetByID(ctx, filter.CompanyID); err != nil {
		return nil, 0, err
	}
	return s.memberships.GetCompanyMembers(ctx, filter)
}

func (s *MembershipService) UpdateMember(ctx context.Context, membershipID, companyID uuid.UUID, in domain.UpdateMembershipInput) (*domain.Membership, error) {
	if membershipID == uuid.Nil {
		return nil, apperrors.Validation("membership_id is required", map[string]any{"field": "membership_id"})
	}
	if err := domain.ValidateUpdateMembershipInput(in); err != nil {
		return nil, err
	}
	return s.memberships.UpdateMembership(ctx, membershipID, companyID, in)
}

func (s *MembershipService) RemoveMember(ctx context.Context, membershipID, companyID uuid.UUID) error {
	if membershipID == uuid.Nil {
		return apperrors.Validation("membership_id is required", map[string]any{"field": "membership_id"})
	}

	membership, err := s.memberships.SoftDeleteMembership(ctx, membershipID, companyID)
	if err != nil {
		return err
	}

	if err := s.memberships.RemoveUserRolesForCompany(ctx, membership.TenantID, membership.UserID, companyID); err != nil {
		return err
	}
	return nil
}
