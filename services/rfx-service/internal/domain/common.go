package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
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

func ParseDate(value, field string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil, apperrors.Validation("invalid "+field, map[string]any{"field": field, "format": "YYYY-MM-DD"})
	}
	return &parsed, nil
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

func ValidateFutureDeadline(deadline *time.Time, field string) error {
	if deadline == nil {
		return nil
	}
	if deadline.Before(time.Now().UTC()) {
		return apperrors.Validation(field+" cannot be in the past", map[string]any{"field": field})
	}
	return nil
}

func ValidateDateRange(from, to *time.Time) error {
	if from != nil && to != nil && to.Before(*from) {
		return apperrors.Validation("valid_to cannot be earlier than valid_from", map[string]any{"field": "valid_to"})
	}
	return nil
}

func ValidateNonNegativeAmount(value float64, field string) error {
	if value < 0 {
		return apperrors.Validation(field+" cannot be negative", map[string]any{"field": field})
	}
	return nil
}

const TransportModeRoad = "ROAD"

func ValidateTransportMode(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if value != TransportModeRoad {
		return apperrors.Validation("transport_mode must be ROAD", map[string]any{"field": "transport_mode"})
	}
	return nil
}

func NormalizeTransportMode(value string) string {
	if strings.TrimSpace(value) == "" {
		return TransportModeRoad
	}
	return strings.TrimSpace(value)
}

func optionalString(value *string) any {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return trimmed
}
