package domain

import (
	"strings"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

const TemplateCloneOperationCloneEventFromTemplate = "CLONE_EVENT_FROM_TEMPLATE"

type CloneEventFromTemplateResult struct {
	Event                   RfxEvent
	DraftVersion            RfxVersion
	SourceTemplateID        uuid.UUID
	SourceTemplateVersionID uuid.UUID
	SourceVersionNumber     int
	SourceVersionStatus     string
	SourceVersionWarning    bool
}

func ValidateCloneEventFromTemplate(templateVersionID uuid.UUID, eventIn CreateRfxEventInput) error {
	if templateVersionID == uuid.Nil {
		return apperrors.Validation("template_version_id is required", map[string]any{"field": "template_version_id"})
	}
	return ValidateCreateRfxEventInput(eventIn)
}

func EnsureTemplateVersionEventCloneSource(status string) error {
	switch strings.TrimSpace(status) {
	case RfxVersionStatusPublished, RfxVersionStatusSuperseded:
		return nil
	case RfxVersionStatusDraft:
		return apperrors.Conflict("draft template version cannot be used as clone source", map[string]any{"field": "template_version_id", "status": status})
	default:
		return apperrors.Conflict("template version cannot be used as clone source", map[string]any{"field": "template_version_id", "status": status})
	}
}

type CloneEventFromTemplateIdempotencyPayload struct {
	TemplateVersionID uuid.UUID           `json:"template_version_id"`
	Event             CreateRfxEventInput `json:"event"`
}

func NewCloneEventFromTemplateIdempotencyPayload(templateVersionID uuid.UUID, eventIn CreateRfxEventInput) CloneEventFromTemplateIdempotencyPayload {
	return CloneEventFromTemplateIdempotencyPayload{
		TemplateVersionID: templateVersionID,
		Event:             eventIn,
	}
}
