package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/identity-service/internal/platform/errors"
)

const (
	UserStatusActive  = "ACTIVE"
	UserStatusDeleted = "DELETED"
)

var allowedLocales = map[string]struct{}{
	"ru-RU": {},
	"en-US": {},
	"zh-CN": {},
}

type User struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	Email           string
	Phone           *string
	PasswordHash    string
	FullName        string
	PreferredLocale string
	Status          string
	LastLoginAt     *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       *time.Time
	Version         int
}

type CreateUserInput struct {
	TenantID        uuid.UUID
	Email           string
	Phone           *string
	Password        string
	FullName        string
	PreferredLocale string
}

type UpdateUserInput struct {
	Phone           *string
	FullName        *string
	PreferredLocale *string
	Status          *string
}

type ListUsersFilter struct {
	TenantID uuid.UUID
	Status   *string
	Search   *string
	Limit    int
	Offset   int
}

type LoginInput struct {
	TenantID uuid.UUID
	Email    string
	Password string
}

func ValidateCreateUserInput(in CreateUserInput) error {
	if in.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if strings.TrimSpace(in.Email) == "" {
		return apperrors.Validation("email is required", map[string]any{"field": "email"})
	}
	if strings.TrimSpace(in.Password) == "" {
		return apperrors.Validation("password is required", map[string]any{"field": "password"})
	}
	if strings.TrimSpace(in.FullName) == "" {
		return apperrors.Validation("full_name is required", map[string]any{"field": "full_name"})
	}
	if err := validatePreferredLocale(in.PreferredLocale); err != nil {
		return err
	}
	return nil
}

func ValidateUpdateUserInput(in UpdateUserInput) error {
	if in.FullName != nil && strings.TrimSpace(*in.FullName) == "" {
		return apperrors.Validation("full_name cannot be empty", map[string]any{"field": "full_name"})
	}
	if in.PreferredLocale != nil {
		if err := validatePreferredLocale(*in.PreferredLocale); err != nil {
			return err
		}
	}
	if in.Status != nil && strings.TrimSpace(*in.Status) == "" {
		return apperrors.Validation("status cannot be empty", map[string]any{"field": "status"})
	}
	return nil
}

func ValidateListUsersFilter(f ListUsersFilter) error {
	if f.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
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
	return nil
}

func ValidateLoginInput(in LoginInput) error {
	if in.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if strings.TrimSpace(in.Email) == "" {
		return apperrors.Validation("email is required", map[string]any{"field": "email"})
	}
	if strings.TrimSpace(in.Password) == "" {
		return apperrors.Validation("password is required", map[string]any{"field": "password"})
	}
	return nil
}

func ParseUUID(value, field string) (uuid.UUID, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return uuid.Nil, apperrors.Validation(field+" is required", map[string]any{"field": field})
	}
	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, apperrors.Validation("invalid "+field, map[string]any{"field": field})
	}
	return id, nil
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func NormalizePreferredLocale(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "ru-RU"
	}
	return value
}

func validatePreferredLocale(value string) error {
	value = NormalizePreferredLocale(value)
	if _, ok := allowedLocales[value]; !ok {
		return apperrors.Validation("invalid preferred_locale", map[string]any{"field": "preferred_locale", "value": value})
	}
	return nil
}

func OptionalString(value *string) any {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return trimmed
}
