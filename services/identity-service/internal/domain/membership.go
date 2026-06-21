package domain

import (
	"strings"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/identity-service/internal/platform/errors"
)

const (
	MembershipStatusActive    = "ACTIVE"
	MembershipStatusInvited   = "INVITED"
	MembershipStatusSuspended = "SUSPENDED"
	MembershipStatusDeleted   = "DELETED"
)

var allowedMembershipStatuses = map[string]struct{}{
	MembershipStatusActive:    {},
	MembershipStatusInvited:   {},
	MembershipStatusSuspended: {},
	MembershipStatusDeleted:   {},
}

type UserCompany struct {
	MembershipID     uuid.UUID
	CompanyID        uuid.UUID
	LegalName        string
	ShortName        *string
	CompanyType      string
	Position         *string
	MembershipStatus string
	Roles            []CompanyRole
}

type CompanyRole struct {
	RoleID uuid.UUID
	Code   string
	Name   string
}

type ListUserCompaniesFilter struct {
	TenantID uuid.UUID
	UserID   uuid.UUID
	Status   *string
}

type AssignCompanyRoleInput struct {
	TenantID  uuid.UUID
	UserID    uuid.UUID
	CompanyID uuid.UUID
	RoleID    uuid.UUID
}

func ValidateListUserCompaniesFilter(f ListUserCompaniesFilter) error {
	if f.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if f.UserID == uuid.Nil {
		return apperrors.Validation("user_id is required", map[string]any{"field": "user_id"})
	}
	if f.Status != nil {
		if err := validateMembershipStatus(*f.Status); err != nil {
			return err
		}
	}
	return nil
}

func ValidateAssignCompanyRoleInput(in AssignCompanyRoleInput) error {
	if in.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if in.UserID == uuid.Nil {
		return apperrors.Validation("user_id is required", map[string]any{"field": "user_id"})
	}
	if in.CompanyID == uuid.Nil {
		return apperrors.Validation("company_id is required", map[string]any{"field": "company_id"})
	}
	if in.RoleID == uuid.Nil {
		return apperrors.Validation("role_id is required", map[string]any{"field": "role_id"})
	}
	return nil
}

func validateMembershipStatus(status string) error {
	status = strings.TrimSpace(status)
	if _, ok := allowedMembershipStatuses[status]; !ok {
		return apperrors.Validation("invalid membership status", map[string]any{"field": "status", "value": status})
	}
	return nil
}
