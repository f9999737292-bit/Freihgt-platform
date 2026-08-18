package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

type RfxAward struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	RfxEventID       uuid.UUID
	RfxResponseID    uuid.UUID
	CarrierCompanyID uuid.UUID
	TotalAmount      *float64
	CurrencyCode     *string
	AwardedBy        *uuid.UUID
	AwardedAt        time.Time
	Version          int
}

type CreateRfxAwardInput struct {
	TenantID         uuid.UUID
	RfxEventID       uuid.UUID
	RfxResponseID    uuid.UUID
	CarrierCompanyID uuid.UUID
	TotalAmount      *float64
	CurrencyCode     *string
	AwardedBy        uuid.UUID
}

func ValidateAwardEventStatus(status string) error {
	switch status {
	case RfxStatusEvaluationInProgress, RfxStatusShortlisted, RfxStatusResponsesClosed, RfxStatusResponsesOpen, RfxStatusPublished:
		return nil
	default:
		return apperrors.Validation("rfx event is not in an awardable status", map[string]any{"field": "status", "status": status})
	}
}

func ValidateManualScore(score float64) error {
	if score < 0 || score > 100 {
		return apperrors.Validation("manual score must be between 0 and 100", map[string]any{"field": "manual_score"})
	}
	return nil
}

func NormalizeCurrencyCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}
