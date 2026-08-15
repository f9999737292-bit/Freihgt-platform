package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

type RfxResponseRevision struct {
	ID                   uuid.UUID
	TenantID             uuid.UUID
	RfxResponseID        uuid.UUID
	RevisionNumber       int
	IsActive             bool
	PriceAmount          *float64
	CurrencyCode         string
	CapacityUnits        *float64
	TransitHours         *float64
	SLAScoreInput        *float64
	CarrierKPIInput      *float64
	ReliabilityInput     *float64
	Comment              *string
	SubmittedAt          *time.Time
	IdempotencyKey       *string
	CreatedAt            time.Time
	ParticipantCompanyID uuid.UUID
	RfxEventID           uuid.UUID
}

type SubmitResponseRevisionInput struct {
	TenantID             uuid.UUID
	RfxEventID           uuid.UUID
	RfxResponseID        uuid.UUID
	ParticipantCompanyID uuid.UUID
	PriceAmount          float64
	CurrencyCode         string
	CapacityUnits        float64
	TransitHours         float64
	SLAScoreInput        float64
	CarrierKPIInput      float64
	ReliabilityInput     float64
	Comment              *string
	IdempotencyKey       *string
	SubmittedBy          *uuid.UUID
}

func ValidateSubmitResponseRevisionInput(in SubmitResponseRevisionInput) error {
	if in.TenantID == uuid.Nil || in.RfxEventID == uuid.Nil || in.RfxResponseID == uuid.Nil {
		return apperrors.Validation("tenant_id, rfx_event_id and rfx_response_id are required", map[string]any{})
	}
	if in.ParticipantCompanyID == uuid.Nil {
		return apperrors.Validation("participant_company_id is required", map[string]any{"field": "participant_company_id"})
	}
	if in.PriceAmount < 0 {
		return apperrors.Validation("price_amount cannot be negative", map[string]any{"field": "price_amount"})
	}
	if strings.TrimSpace(in.CurrencyCode) == "" {
		in.CurrencyCode = "RUB"
	}
	return nil
}
