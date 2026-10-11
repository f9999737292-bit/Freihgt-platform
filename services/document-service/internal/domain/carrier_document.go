package domain

import (
	"strings"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/document-service/internal/platform/errors"
)

const MaxCarrierDocumentLimit = 50

// Operational shipment documents a carrier participant may read.
// Financial and claim types stay out of this contract.
var carrierOperationalDocumentTypes = []string{
	"ETRN", "EPD", "WAYBILL", "POD", "DISCREPANCY_ACT", "ECMR",
}

var carrierVisibleDocumentStatuses = []string{
	DocumentStatusReadyForSigning,
	DocumentStatusSigningInProgress,
	DocumentStatusSigned,
	DocumentStatusSentToOperator,
	DocumentStatusAccepted,
	DocumentStatusRejected,
	DocumentStatusArchived,
}

func CarrierOperationalDocumentTypes() []string {
	out := make([]string, len(carrierOperationalDocumentTypes))
	copy(out, carrierOperationalDocumentTypes)
	return out
}

func CarrierVisibleDocumentStatuses() []string {
	out := make([]string, len(carrierVisibleDocumentStatuses))
	copy(out, carrierVisibleDocumentStatuses)
	return out
}

func IsCarrierOperationalDocumentType(value string) bool {
	value = strings.TrimSpace(value)
	for _, item := range carrierOperationalDocumentTypes {
		if item == value {
			return true
		}
	}
	return false
}

func IsCarrierVisibleDocumentStatus(value string) bool {
	value = strings.TrimSpace(value)
	for _, item := range carrierVisibleDocumentStatuses {
		if item == value {
			return true
		}
	}
	return false
}

type CarrierDocumentFilter struct {
	TenantID         uuid.UUID
	CarrierCompanyID uuid.UUID
	ShipmentID       uuid.UUID
	DocumentType     *string
	DocumentStatus   *string
	Limit            int
	Offset           int
}

func ValidateCarrierDocumentFilter(f CarrierDocumentFilter) error {
	if f.TenantID == uuid.Nil {
		return apperrors.Unauthorized("trusted tenant context is required")
	}
	if f.CarrierCompanyID == uuid.Nil {
		return apperrors.Validation("carrier_company_id is required", map[string]any{"field": "carrier_company_id"})
	}
	if f.ShipmentID == uuid.Nil {
		return apperrors.Validation("shipment_id is required", map[string]any{"field": "shipment_id"})
	}
	if f.DocumentType != nil && !IsCarrierOperationalDocumentType(*f.DocumentType) {
		return apperrors.Validation("document_type is not available on the carrier document source", map[string]any{"field": "document_type"})
	}
	if f.DocumentStatus != nil && !IsCarrierVisibleDocumentStatus(*f.DocumentStatus) {
		return apperrors.Validation("document_status is not available on the carrier document source", map[string]any{"field": "document_status"})
	}
	if f.Limit <= 0 {
		return apperrors.Validation("limit must be greater than 0", map[string]any{"field": "limit"})
	}
	if f.Limit > MaxCarrierDocumentLimit {
		return apperrors.Validation("limit must be less than or equal to 50", map[string]any{"field": "limit"})
	}
	if f.Offset < 0 {
		return apperrors.Validation("offset must be greater than or equal to 0", map[string]any{"field": "offset"})
	}
	return nil
}
