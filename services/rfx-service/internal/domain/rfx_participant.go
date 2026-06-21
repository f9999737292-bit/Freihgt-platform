package domain

import (
	"strings"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

const (
	ParticipantStatusInvited           = "INVITED"
	ParticipantStatusResponseSubmitted = "RESPONSE_SUBMITTED"
)

type RfxParticipant struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	RfxEventID      uuid.UUID
	CompanyID       uuid.UUID
	ParticipantType string
	Status          string
	InvitedAt       *string
}

type AddRfxParticipantInput struct {
	TenantID        uuid.UUID
	RfxEventID      uuid.UUID
	CompanyID       uuid.UUID
	ParticipantType string
}

func ValidateAddRfxParticipantInput(in AddRfxParticipantInput) error {
	if in.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if in.RfxEventID == uuid.Nil {
		return apperrors.Validation("rfx_event_id is required", map[string]any{"field": "rfx_event_id"})
	}
	if in.CompanyID == uuid.Nil {
		return apperrors.Validation("company_id is required", map[string]any{"field": "company_id"})
	}
	if strings.TrimSpace(in.ParticipantType) == "" {
		return apperrors.Validation("participant_type is required", map[string]any{"field": "participant_type"})
	}
	return nil
}
