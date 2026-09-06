package domain

import (
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

const (
	RfxResponseStatusDraft     = "DRAFT"
	RfxResponseStatusSubmitted = "SUBMITTED"

	RfxStatusResponsesOpen = "RESPONSES_OPEN"
)

type RfxResponse struct {
	ID                   uuid.UUID
	TenantID             uuid.UUID
	RfxEventID           uuid.UUID
	ParticipantCompanyID uuid.UUID
	RfxVersionID         *uuid.UUID
	Status               string
	SubmittedAt          *time.Time
	SaveVersion          int64
	LastSavedAt          *time.Time
	LastSavedBy          *uuid.UUID
	CompletionPercent    float64
	CommercialScore      *float64
	ManualScore          *float64
	TotalScore           *float64
	EvaluationRank       *int
	CreatedAt            time.Time
	UpdatedAt            time.Time
	Version              int
	OfferLines           []RfxResponseOfferLine
}

type EvaluationResponseView struct {
	Response            RfxResponse
	ParticipantStatus   string
	TotalAmount         float64
	CurrencyCode        string
	Comparable          bool
	Shortlisted         bool
	Awarded             bool
	OfferComplete       bool
}

type CreateRfxResponseInput struct {
	TenantID             uuid.UUID
	RfxEventID           uuid.UUID
	ParticipantCompanyID uuid.UUID
	RfxVersionID         *uuid.UUID
}

func ValidateCreateRfxResponse(eventStatus string) error {
	if eventStatus != RfxStatusPublished && eventStatus != RfxStatusResponsesOpen {
		return apperrors.Validation("rfx event must be PUBLISHED or RESPONSES_OPEN to accept responses", map[string]any{"field": "status", "status": eventStatus})
	}
	return nil
}

func ValidateSubmitRfxResponse(status string) error {
	if status != RfxResponseStatusDraft {
		return apperrors.Validation("rfx response can only be submitted from DRAFT status", map[string]any{"field": "status", "status": status})
	}
	return nil
}

func ValidateUpdateDraftResponse(status string) error {
	if status != RfxResponseStatusDraft {
		return apperrors.Validation("rfx response commercial data can only be updated in DRAFT status", map[string]any{"field": "status", "status": status})
	}
	return nil
}

func ValidateCreateRfxResponseInput(in CreateRfxResponseInput) error {
	if in.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if in.RfxEventID == uuid.Nil {
		return apperrors.Validation("rfx_event_id is required", map[string]any{"field": "rfx_event_id"})
	}
	if in.ParticipantCompanyID == uuid.Nil {
		return apperrors.Validation("participant_company_id is required", map[string]any{"field": "participant_company_id"})
	}
	return nil
}
