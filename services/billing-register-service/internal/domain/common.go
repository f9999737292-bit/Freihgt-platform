package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/billing-register-service/internal/platform/errors"
)

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

func ParseOptionalUUID(value, field string) (*uuid.UUID, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	id, err := uuid.Parse(value)
	if err != nil {
		return nil, apperrors.Validation("invalid "+field, map[string]any{"field": field})
	}
	return &id, nil
}

func ParseDate(value, field string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, apperrors.Validation(field+" is required", map[string]any{"field": field})
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, apperrors.Validation("invalid "+field, map[string]any{"field": field, "format": "YYYY-MM-DD"})
	}
	return parsed, nil
}

func ParseOptionalDate(value, field string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := ParseDate(value, field)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func ValidateListPagination(limit, offset int) error {
	if limit <= 0 {
		return apperrors.Validation("limit must be greater than 0", map[string]any{"field": "limit"})
	}
	if limit > 100 {
		return apperrors.Validation("limit must be less than or equal to 100", map[string]any{"field": "limit"})
	}
	if offset < 0 {
		return apperrors.Validation("offset must be greater than or equal to 0", map[string]any{"field": "offset"})
	}
	return nil
}

func ValidateNonNegativeAmount(value float64, field string) error {
	if value < 0 {
		return apperrors.Validation(field+" cannot be negative", map[string]any{"field": field})
	}
	return nil
}

func FormatDate(value time.Time) string {
	return value.Format("2006-01-02")
}

func FormatOptionalDate(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.Format("2006-01-02")
	return &formatted
}
