package handlers

import (
	"strings"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/document-service/internal/platform/errors"
)

func reconcileTenant(trusted uuid.UUID, supplied string) error {
	supplied = strings.TrimSpace(supplied)
	if supplied == "" {
		return nil
	}
	id, err := uuid.Parse(supplied)
	if err != nil || id != trusted {
		return apperrors.Unauthorized("tenant does not match trusted tenant")
	}
	return nil
}
