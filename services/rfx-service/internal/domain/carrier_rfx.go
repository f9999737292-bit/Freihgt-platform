package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

const (
	CarrierOwnResponseNotStarted = "NOT_STARTED"
	CarrierResponseFilterOpen    = "OPEN_FOR_RESPONSE"
	CarrierResponseFilterResponded = "RESPONDED"
	CarrierResponseFilterNotResponded = "NOT_RESPONDED"
	CarrierResponseFilterClosed  = "CLOSED"
)

type CarrierInvitedRfxEvent struct {
	Event               RfxEvent
	ParticipantStatus   string
	OwnResponseStatus   string
	OwnResponseID       *uuid.UUID
	LotCount            int
	ParticipantCompanyID uuid.UUID
}

type ListCarrierInvitedEventsFilter struct {
	TenantID           uuid.UUID
	CarrierCompanyID   uuid.UUID
	Status             *string
	ResponseFilter     *string
	Search             *string
	Limit              int
	Offset             int
}

func ValidateListCarrierInvitedEventsFilter(f ListCarrierInvitedEventsFilter) error {
	if f.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if f.CarrierCompanyID == uuid.Nil {
		return apperrors.Validation("carrier_company_id is required", map[string]any{"field": "carrier_company_id"})
	}
	if f.Limit <= 0 || f.Limit > 100 {
		return apperrors.Validation("invalid limit", map[string]any{"field": "limit"})
	}
	if f.Offset < 0 {
		return apperrors.Validation("offset must be >= 0", map[string]any{"field": "offset"})
	}
	if f.ResponseFilter != nil {
		switch strings.ToUpper(strings.TrimSpace(*f.ResponseFilter)) {
		case CarrierResponseFilterOpen, CarrierResponseFilterResponded, CarrierResponseFilterNotResponded, CarrierResponseFilterClosed, "":
		default:
			return apperrors.Validation("invalid response_filter", map[string]any{"field": "response_filter", "value": *f.ResponseFilter})
		}
	}
	return nil
}

func DeriveOwnResponseStatus(responseStatus *string) string {
	if responseStatus == nil || strings.TrimSpace(*responseStatus) == "" {
		return CarrierOwnResponseNotStarted
	}
	return strings.ToUpper(strings.TrimSpace(*responseStatus))
}

func HasCarrierRole(roleCodes []string) bool {
	for _, code := range roleCodes {
		switch strings.ToUpper(strings.TrimSpace(code)) {
		case "CARRIER_ADMIN", "CARRIER_DISPATCHER":
			return true
		}
	}
	return false
}

func IsEventOpenForResponse(status string, deadline *time.Time, now time.Time) bool {
	switch status {
	case RfxStatusPublished, RfxStatusResponsesOpen:
		if deadline == nil {
			return true
		}
		return !now.UTC().After(deadline.UTC())
	default:
		return false
	}
}

func IsEventClosedForResponse(status string, deadline *time.Time, now time.Time) bool {
	if status == "RESPONSES_CLOSED" || status == "CANCELLED" || status == "ARCHIVED" {
		return true
	}
	if deadline != nil && now.UTC().After(deadline.UTC()) {
		return true
	}
	return false
}
