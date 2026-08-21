package domain

import (
	"encoding/json"
	"time"

	apperrors "github.com/freight-platform/contract-rate-service/internal/platform/errors"
)

// NullableDatePatch distinguishes omitted, explicit null (clear), and set date values.
type NullableDatePatch struct {
	Present bool
	Clear   bool
	Value   *time.Time
}

func ParseNullableDatePatch(raw json.RawMessage, field string) (NullableDatePatch, error) {
	if raw == nil {
		return NullableDatePatch{}, nil
	}
	trimmed := string(raw)
	if trimmed == "null" {
		return NullableDatePatch{Present: true, Clear: true}, nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return NullableDatePatch{}, apperrors.Validation("invalid date format, expected YYYY-MM-DD or null", map[string]any{"field": field})
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return NullableDatePatch{}, apperrors.Validation("invalid date format, expected YYYY-MM-DD", map[string]any{"field": field})
	}
	parsed = parsed.UTC()
	return NullableDatePatch{Present: true, Value: &parsed}, nil
}

func ApplyNullableDatePatch(current *time.Time, patch NullableDatePatch) *time.Time {
	if !patch.Present {
		return current
	}
	if patch.Clear {
		return nil
	}
	return patch.Value
}
