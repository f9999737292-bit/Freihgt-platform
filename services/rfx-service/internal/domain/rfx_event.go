package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

const (
	RfxStatusDraft     = "DRAFT"
	RfxStatusPublished = "PUBLISHED"
	RfxStatusCancelled = "CANCELLED"
)

var allowedRfxTypes = map[string]struct{}{
	"RFI": {}, "RFQ": {}, "RFP": {}, "RFG": {}, "RFT": {}, "SPOT_RFQ": {},
	"MINI_TENDER": {}, "LANE_TENDER": {}, "CONTRACT_TENDER": {}, "SEASONAL_TENDER": {},
	"PROJECT_TENDER": {}, "REVERSE_AUCTION": {},
}

var allowedRfxCategories = map[string]struct{}{
	"FREIGHT": {}, "WAREHOUSING": {}, "CUSTOMS": {}, "INSURANCE": {}, "PACKAGING": {},
	"FUEL": {}, "VEHICLE_SERVICE": {}, "GOODS": {}, "GENERAL_SERVICE": {},
}

type RfxEvent struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	RfxNumber        string
	RfxType          string
	Category         string
	Title            string
	Description      *string
	OwnerCompanyID   uuid.UUID
	Status           string
	CurrencyCode     *string
	ValidFrom        *time.Time
	ValidTo          *time.Time
	ResponseDeadline *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Version          int
}

type CreateRfxEventInput struct {
	TenantID         uuid.UUID
	RfxNumber        string
	RfxType          string
	Category         string
	Title            string
	Description      *string
	OwnerCompanyID   uuid.UUID
	CurrencyCode     *string
	ValidFrom        *time.Time
	ValidTo          *time.Time
	ResponseDeadline *time.Time
}

type UpdateRfxEventInput struct {
	Title            *string
	Description      *string
	ResponseDeadline *time.Time
}

type ListRfxEventsFilter struct {
	TenantID       uuid.UUID
	RfxType        *string
	Category       *string
	Status         *string
	OwnerCompanyID *uuid.UUID
	Limit          int
	Offset         int
}

func ValidateCreateRfxEventInput(in CreateRfxEventInput) error {
	if in.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if strings.TrimSpace(in.RfxNumber) == "" {
		return apperrors.Validation("rfx_number is required", map[string]any{"field": "rfx_number"})
	}
	if err := validateRfxType(in.RfxType); err != nil {
		return err
	}
	if err := validateRfxCategory(in.Category); err != nil {
		return err
	}
	if strings.TrimSpace(in.Title) == "" {
		return apperrors.Validation("title is required", map[string]any{"field": "title"})
	}
	if in.OwnerCompanyID == uuid.Nil {
		return apperrors.Validation("owner_company_id is required", map[string]any{"field": "owner_company_id"})
	}
	if err := ValidateDateRange(in.ValidFrom, in.ValidTo); err != nil {
		return err
	}
	return ValidateFutureDeadline(in.ResponseDeadline, "response_deadline")
}

func ValidateListRfxEventsFilter(f ListRfxEventsFilter) error {
	if f.TenantID == uuid.Nil {
		return apperrors.Validation("tenant_id is required", map[string]any{"field": "tenant_id"})
	}
	if f.Limit <= 0 || f.Limit > 100 {
		return apperrors.Validation("invalid limit", map[string]any{"field": "limit"})
	}
	if f.Offset < 0 {
		return apperrors.Validation("offset must be >= 0", map[string]any{"field": "offset"})
	}
	return nil
}

func ValidatePublishRfxEvent(status string) error {
	if status != RfxStatusDraft {
		return apperrors.Validation("rfx event can only be published from DRAFT status", map[string]any{"field": "status", "status": status})
	}
	return nil
}

func ValidateCancelRfxEvent(status string) error {
	if status != RfxStatusDraft && status != RfxStatusPublished {
		return apperrors.Validation("rfx event can only be cancelled from DRAFT or PUBLISHED status", map[string]any{"field": "status", "status": status})
	}
	return nil
}

func ValidateUpdateRfxEvent(status string) error {
	if status != RfxStatusDraft {
		return apperrors.Validation("rfx event can only be updated in DRAFT status", map[string]any{"field": "status", "status": status})
	}
	return nil
}

func validateRfxType(value string) error {
	value = strings.TrimSpace(value)
	if _, ok := allowedRfxTypes[value]; !ok {
		return apperrors.Validation("invalid rfx_type", map[string]any{"field": "rfx_type", "value": value})
	}
	return nil
}

func validateRfxCategory(value string) error {
	value = strings.TrimSpace(value)
	if _, ok := allowedRfxCategories[value]; !ok {
		return apperrors.Validation("invalid category", map[string]any{"field": "category", "value": value})
	}
	return nil
}
