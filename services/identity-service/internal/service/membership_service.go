package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/freight-platform/identity-service/internal/domain"
	apperrors "github.com/freight-platform/identity-service/internal/platform/errors"
)

type MembershipStore interface {
	GetUserCompanies(ctx context.Context, filter domain.ListUserCompaniesFilter) ([]domain.UserCompany, error)
	ActiveMembershipExists(ctx context.Context, tenantID, userID, companyID uuid.UUID) (bool, error)
	AddCompanyRoleToUser(ctx context.Context, in domain.AssignCompanyRoleInput) error
	RemoveCompanyRoleFromUser(ctx context.Context, tenantID, userID, companyID, roleID uuid.UUID) error
}

type MembershipService struct {
	users       UserStore
	roles       RoleStore
	memberships MembershipStore
}

func NewMembershipService(users UserStore, roles RoleStore, memberships MembershipStore) *MembershipService {
	return &MembershipService{users: users, roles: roles, memberships: memberships}
}

func (s *MembershipService) GetUserCompanies(ctx context.Context, filter domain.ListUserCompaniesFilter) ([]domain.UserCompany, error) {
	if err := domain.ValidateListUserCompaniesFilter(filter); err != nil {
		return nil, err
	}
	if _, err := s.users.GetByID(ctx, filter.UserID); err != nil {
		return nil, err
	}
	return s.memberships.GetUserCompanies(ctx, filter)
}

func (s *MembershipService) AddCompanyRoleToUser(ctx context.Context, in domain.AssignCompanyRoleInput) error {
	if err := domain.ValidateAssignCompanyRoleInput(in); err != nil {
		return err
	}

	if _, err := s.users.GetByID(ctx, in.UserID); err != nil {
		return err
	}

	companyExists, err := s.roles.CompanyExists(ctx, in.CompanyID, in.TenantID)
	if err != nil {
		return err
	}
	if !companyExists {
		return apperrors.NotFound("company not found")
	}

	active, err := s.memberships.ActiveMembershipExists(ctx, in.TenantID, in.UserID, in.CompanyID)
	if err != nil {
		return err
	}
	if !active {
		return apperrors.NotFound("active membership not found")
	}

	available, err := s.roles.RoleAvailableForTenant(ctx, in.RoleID, in.TenantID)
	if err != nil {
		return err
	}
	if !available {
		return apperrors.NotFound("role not found")
	}

	return s.memberships.AddCompanyRoleToUser(ctx, in)
}

func (s *MembershipService) RemoveCompanyRoleFromUser(ctx context.Context, tenantID, userID, companyID, roleID uuid.UUID) error {
	if tenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if userID == uuid.Nil {
		return apperrors.Validation("user_id is required", map[string]any{"field": "user_id"})
	}
	if companyID == uuid.Nil {
		return apperrors.Validation("company_id is required", map[string]any{"field": "company_id"})
	}
	if roleID == uuid.Nil {
		return apperrors.Validation("role_id is required", map[string]any{"field": "role_id"})
	}

	if _, err := s.users.GetByID(ctx, userID); err != nil {
		return err
	}

	return s.memberships.RemoveCompanyRoleFromUser(ctx, tenantID, userID, companyID, roleID)
}
