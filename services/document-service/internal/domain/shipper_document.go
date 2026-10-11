package domain

import (
	"strings"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/document-service/internal/platform/errors"
)

const MaxShipperDocumentLimit = 50

// Operational shipment documents a shipper participant may read.
// Financial and claim types stay out of this contract.
var shipperOperationalDocumentTypes = []string{
	"ETRN", "EPD", "WAYBILL", "POD", "DISCREPANCY_ACT", "ECMR",
}

// Customer-visible lifecycle. DRAFT and CANCELLED stay internal.
var shipperVisibleDocumentStatuses = []string{
	DocumentStatusReadyForSigning,
	DocumentStatusSigningInProgress,
	DocumentStatusSigned,
	DocumentStatusSentToOperator,
	DocumentStatusAccepted,
	DocumentStatusRejected,
	DocumentStatusArchived,
}

func ShipperOperationalDocumentTypes() []string {
	out := make([]string, len(shipperOperationalDocumentTypes))
	copy(out, shipperOperationalDocumentTypes)
	return out
}

func ShipperVisibleDocumentStatuses() []string {
	out := make([]string, len(shipperVisibleDocumentStatuses))
	copy(out, shipperVisibleDocumentStatuses)
	return out
}

func IsShipperOperationalDocumentType(value string) bool {
	value = strings.TrimSpace(value)
	for _, item := range shipperOperationalDocumentTypes {
		if item == value {
			return true
		}
	}
	return false
}

func IsShipperVisibleDocumentStatus(value string) bool {
	value = strings.TrimSpace(value)
	for _, item := range shipperVisibleDocumentStatuses {
		if item == value {
			return true
		}
	}
	return false
}

type ShipperDocumentFilter struct {
	TenantID         uuid.UUID
	ShipperCompanyID uuid.UUID
	ShipmentID       uuid.UUID
	DocumentType     *string
	DocumentStatus   *string
	Limit            int
	Offset           int
}

func ValidateShipperDocumentFilter(f ShipperDocumentFilter) error {
	if f.TenantID == uuid.Nil {
		return apperrors.Unauthorized("trusted tenant context is required")
	}
	if f.ShipperCompanyID == uuid.Nil {
		return apperrors.Validation("shipper_company_id is required", map[string]any{"field": "shipper_company_id"})
	}
	if f.ShipmentID == uuid.Nil {
		return apperrors.Validation("shipment_id is required", map[string]any{"field": "shipment_id"})
	}
	if f.DocumentType != nil && !IsShipperOperationalDocumentType(*f.DocumentType) {
		return apperrors.Validation("document_type is not available on the shipper document source", map[string]any{"field": "document_type"})
	}
	if f.DocumentStatus != nil && !IsShipperVisibleDocumentStatus(*f.DocumentStatus) {
		return apperrors.Validation("document_status is not available on the shipper document source", map[string]any{"field": "document_status"})
	}
	if f.Limit <= 0 {
		return apperrors.Validation("limit must be greater than 0", map[string]any{"field": "limit"})
	}
	if f.Limit > MaxShipperDocumentLimit {
		return apperrors.Validation("limit must be less than or equal to 50", map[string]any{"field": "limit"})
	}
	if f.Offset < 0 {
		return apperrors.Validation("offset must be greater than or equal to 0", map[string]any{"field": "offset"})
	}
	return nil
}
