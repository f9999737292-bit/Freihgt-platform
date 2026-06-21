package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/freight-platform/identity-service/internal/domain"
	apperrors "github.com/freight-platform/identity-service/internal/platform/errors"
)

type RoleStore interface {
	ListByTenant(ctx context.Context, tenantID uuid.UUID) ([]domain.Role, error)
	GetByID(ctx context.Context, roleID uuid.UUID) (*domain.Role, error)
	RoleAvailableForTenant(ctx context.Context, roleID, tenantID uuid.UUID) (bool, error)
	CompanyExists(ctx context.Context, companyID, tenantID uuid.UUID) (bool, error)
	AssignRole(ctx context.Context, in domain.AssignRoleInput) error
	ListUserRoles(ctx context.Context, userID, tenantID uuid.UUID) ([]domain.UserRoleAssignment, error)
}

type PermissionStore interface {
	ListByRoleID(ctx context.Context, roleID uuid.UUID) ([]domain.Permission, error)
}

type RoleService struct {
	roles       RoleStore
	permissions PermissionStore
	users       UserStore
}

func NewRoleService(roles RoleStore, permissions PermissionStore, users UserStore) *RoleService {
	return &RoleService{roles: roles, permissions: permissions, users: users}
}

func (s *RoleService) ListRoles(ctx context.Context, tenantID uuid.UUID) ([]domain.Role, error) {
	if tenantID == uuid.Nil {
		return nil, apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	return s.roles.ListByTenant(ctx, tenantID)
}

func (s *RoleService) ListRolePermissions(ctx context.Context, roleID uuid.UUID) ([]domain.Permission, error) {
	if roleID == uuid.Nil {
		return nil, apperrors.Validation("role_id is required", map[string]any{"field": "role_id"})
	}
	if _, err := s.roles.GetByID(ctx, roleID); err != nil {
		return nil, err
	}
	return s.permissions.ListByRoleID(ctx, roleID)
}

func (s *RoleService) AssignRole(ctx context.Context, in domain.AssignRoleInput) error {
	if in.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if in.UserID == uuid.Nil {
		return apperrors.Validation("user_id is required", map[string]any{"field": "user_id"})
	}
	if in.RoleID == uuid.Nil {
		return apperrors.Validation("role_id is required", map[string]any{"field": "role_id"})
	}

	if _, err := s.users.GetByID(ctx, in.UserID); err != nil {
		return err
	}

	available, err := s.roles.RoleAvailableForTenant(ctx, in.RoleID, in.TenantID)
	if err != nil {
		return err
	}
	if !available {
		return apperrors.NotFound("role not found")
	}

	if in.CompanyID != nil {
		exists, err := s.roles.CompanyExists(ctx, *in.CompanyID, in.TenantID)
		if err != nil {
			return err
		}
		if !exists {
			return apperrors.NotFound("company not found")
		}
	}

	return s.roles.AssignRole(ctx, in)
}

func (s *RoleService) ListUserRoles(ctx context.Context, userID, tenantID uuid.UUID) ([]domain.UserRoleAssignment, error) {
	if tenantID == uuid.Nil {
		return nil, apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if userID == uuid.Nil {
		return nil, apperrors.Validation("user_id is required", map[string]any{"field": "user_id"})
	}
	if _, err := s.users.GetByID(ctx, userID); err != nil {
		return nil, err
	}
	return s.roles.ListUserRoles(ctx, userID, tenantID)
}
