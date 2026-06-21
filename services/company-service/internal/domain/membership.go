package domain

import (
	"strings"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/company-service/internal/platform/errors"
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

type Membership struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	CompanyID uuid.UUID
	UserID    uuid.UUID
	Position  *string
	Status    string
	CreatedAt string
	UpdatedAt string
	Version   int
}

type CreateMembershipInput struct {
	TenantID  uuid.UUID
	CompanyID uuid.UUID
	UserID    uuid.UUID
	Position  *string
	RoleID    *uuid.UUID
}

type UpdateMembershipInput struct {
	Position *string
	Status   *string
}

type ListCompanyMembersFilter struct {
	TenantID  uuid.UUID
	CompanyID uuid.UUID
	Status    *string
	Limit     int
	Offset    int
}

type CompanyMember struct {
	MembershipID uuid.UUID
	UserID       uuid.UUID
	Email        string
	FullName     string
	Phone        *string
	Position     *string
	Status       string
	Roles        []MemberRole
}

type MemberRole struct {
	RoleID uuid.UUID
	Code   string
	Name   string
}

func ValidateCreateMembershipInput(in CreateMembershipInput) error {
	if in.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if in.CompanyID == uuid.Nil {
		return apperrors.Validation("company_id is required", map[string]any{"field": "company_id"})
	}
	if in.UserID == uuid.Nil {
		return apperrors.Validation("user_id is required", map[string]any{"field": "user_id"})
	}
	return nil
}

func ValidateUpdateMembershipInput(in UpdateMembershipInput) error {
	if in.Status != nil {
		if err := validateMembershipStatus(*in.Status); err != nil {
			return err
		}
	}
	return nil
}

func ValidateListCompanyMembersFilter(f ListCompanyMembersFilter) error {
	if f.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if f.CompanyID == uuid.Nil {
		return apperrors.Validation("company_id is required", map[string]any{"field": "company_id"})
	}
	if f.Limit <= 0 {
		return apperrors.Validation("limit must be greater than 0", map[string]any{"field": "limit"})
	}
	if f.Limit > 100 {
		return apperrors.Validation("limit must be less than or equal to 100", map[string]any{"field": "limit"})
	}
	if f.Offset < 0 {
		return apperrors.Validation("offset must be greater than or equal to 0", map[string]any{"field": "offset"})
	}
	if f.Status != nil {
		if err := validateMembershipStatus(*f.Status); err != nil {
			return err
		}
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
