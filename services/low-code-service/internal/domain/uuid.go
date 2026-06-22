package domain

import (
	"fmt"
	"strings"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/low-code-service/internal/platform/errors"
)

func ParseUUID(raw, field string) (uuid.UUID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return uuid.Nil, apperrors.Validation(fmt.Sprintf("%s is required", field), map[string]any{"field": field})
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, apperrors.Validation(fmt.Sprintf("invalid %s", field), map[string]any{"field": field, "value": raw})
	}
	return id, nil
}
