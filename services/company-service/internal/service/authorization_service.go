package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/freight-platform/company-service/internal/domain"
	apperrors "github.com/freight-platform/company-service/internal/platform/errors"
)

type MembershipAuthorizer interface {
	UserHasActiveMembership(ctx context.Context, tenantID, userID, companyID uuid.UUID) (bool, error)
	ListActiveCompanyIDsForUser(ctx context.Context, tenantID, userID uuid.UUID) ([]uuid.UUID, error)
	ListUserCompanyRoleCodes(ctx context.Context, tenantID, userID, companyID uuid.UUID) ([]string, error)
	ListUserGlobalRoleCodes(ctx context.Context, tenantID, userID uuid.UUID) ([]string, error)
}

type CompanyAuthorizer struct {
	memberships MembershipAuthorizer
}

func NewCompanyAuthorizer(memberships MembershipAuthorizer) *CompanyAuthorizer {
	return &CompanyAuthorizer{memberships: memberships}
}

func (a *CompanyAuthorizer) isPlatformAdmin(ctx context.Context, tenantID, userID uuid.UUID) (bool, error) {
	roles, err := a.memberships.ListUserGlobalRoleCodes(ctx, tenantID, userID)
	if err != nil {
		return false, err
	}
	return domain.HasPlatformAdminRole(roles), nil
}

func (a *CompanyAuthorizer) AuthorizeCreate(ctx context.Context, tenantID, userID uuid.UUID, bodyTenantID uuid.UUID) error {
	if bodyTenantID != uuid.Nil && bodyTenantID != tenantID {
		return apperrors.Forbidden("tenant_id does not match authenticated tenant")
	}
	ok, err := a.isPlatformAdmin(ctx, tenantID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return apperrors.Forbidden("company creation access denied")
	}
	return nil
}

func (a *CompanyAuthorizer) AuthorizeRead(ctx context.Context, tenantID, userID, companyID uuid.UUID) error {
	if ok, err := a.isPlatformAdmin(ctx, tenantID, userID); err != nil {
		return err
	} else if ok {
		return nil
	}
	has, err := a.memberships.UserHasActiveMembership(ctx, tenantID, userID, companyID)
	if err != nil {
		return err
	}
	if !has {
		return apperrors.NotFound("company not found")
	}
	return nil
}

func (a *CompanyAuthorizer) AuthorizeUpdate(ctx context.Context, tenantID, userID, companyID uuid.UUID) error {
	if ok, err := a.isPlatformAdmin(ctx, tenantID, userID); err != nil {
		return err
	} else if ok {
		return nil
	}
	has, err := a.memberships.UserHasActiveMembership(ctx, tenantID, userID, companyID)
	if err != nil {
		return err
	}
	if !has {
		return apperrors.Forbidden("company update access denied")
	}
	roles, err := a.memberships.ListUserCompanyRoleCodes(ctx, tenantID, userID, companyID)
	if err != nil {
		return err
	}
	if !domain.HasCompanyAdminRole(roles) {
		return apperrors.Forbidden("company update access denied")
	}
	return nil
}

func (a *CompanyAuthorizer) AuthorizeDelete(ctx context.Context, tenantID, userID uuid.UUID) error {
	ok, err := a.isPlatformAdmin(ctx, tenantID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return apperrors.Forbidden("company deletion access denied")
	}
	return nil
}

func (a *CompanyAuthorizer) ListScopeCompanyIDs(ctx context.Context, tenantID, userID uuid.UUID) ([]uuid.UUID, bool, error) {
	if ok, err := a.isPlatformAdmin(ctx, tenantID, userID); err != nil {
		return nil, false, err
	} else if ok {
		return nil, true, nil
	}
	ids, err := a.memberships.ListActiveCompanyIDsForUser(ctx, tenantID, userID)
	if err != nil {
		return nil, false, err
	}
	return ids, false, nil
}

func (a *CompanyAuthorizer) AuthorizeManageMembers(ctx context.Context, tenantID, userID, companyID uuid.UUID) error {
	return a.AuthorizeUpdate(ctx, tenantID, userID, companyID)
}

func (a *CompanyAuthorizer) AuthorizeListMembers(ctx context.Context, tenantID, userID, companyID uuid.UUID) error {
	return a.AuthorizeRead(ctx, tenantID, userID, companyID)
}
