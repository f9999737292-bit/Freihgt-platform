package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
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

func ParseDateTime(value, field string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, apperrors.Validation("invalid "+field, map[string]any{"field": field, "format": "RFC3339"})
	}
	return &parsed, nil
}

func ParseRequiredDateTime(value, field string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, apperrors.Validation(field+" is required", map[string]any{"field": field})
	}
	return ParseDateTime(value, field)
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

func NormalizeCountryCode(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "RU"
	}
	return strings.ToUpper(value)
}

func NormalizeTimezone(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "ru-RU"
	}
	return value
}
