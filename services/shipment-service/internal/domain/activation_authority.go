package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"strings"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

const (
	ReasonRelationshipVersionNotAuthoritative  = "RELATIONSHIP_VERSION_NOT_AUTHORITATIVE"
	ReasonRelationshipNotFound                 = "RELATIONSHIP_NOT_FOUND"
	ReasonRelationshipOperatingCompanyMismatch = "RELATIONSHIP_OPERATING_COMPANY_MISMATCH"
	ReasonRelationshipRevoked                  = "RELATIONSHIP_REVOKED"
	ReasonRelationshipAmbiguous                = "RELATIONSHIP_AMBIGUOUS"
	ReasonOperatingDriverNotValid              = "OPERATING_DRIVER_NOT_VALID"
	ReasonOperatingVehicleNotValid             = "OPERATING_VEHICLE_NOT_VALID"
	ReasonRelationshipStoreUnavailable         = "RELATIONSHIP_STORE_UNAVAILABLE"
)

// ActivationPinSubject is one server-derived participant.
// SourceCompanyID is transport.shipments.carrier_company_id, not the operating carrier.
type ActivationPinSubject struct {
	ShipmentTenantID uuid.UUID
	ShipmentID       uuid.UUID
	SourceCompanyID  uuid.UUID
}

type pinFingerprintDocument struct {
	OperatingTenantID         string                  `json:"operating_tenant_id"`
	OperatingCarrierCompanyID string                  `json:"operating_carrier_company_id"`
	Subjects                  []pinFingerprintSubject `json:"subjects"`
}

type pinFingerprintSubject struct {
	ShipmentTenantID         string `json:"shipment_tenant_id"`
	ShipmentID               string `json:"shipment_id"`
	ShipmentCarrierCompanyID string `json:"shipment_carrier_company_id"`
}

// PinSetFingerprint is the participant-set evidence. It is not the replay predicate.
func PinSetFingerprint(operatingTenantID, operatingCarrierCompanyID uuid.UUID, subjects []ActivationPinSubject) (string, error) {
	ordered := append([]ActivationPinSubject(nil), subjects...)
	slices.SortFunc(ordered, func(left, right ActivationPinSubject) int {
		if cmp := strings.Compare(left.ShipmentTenantID.String(), right.ShipmentTenantID.String()); cmp != 0 {
			return cmp
		}
		return strings.Compare(left.ShipmentID.String(), right.ShipmentID.String())
	})
	document := pinFingerprintDocument{
		OperatingTenantID:         operatingTenantID.String(),
		OperatingCarrierCompanyID: operatingCarrierCompanyID.String(),
		Subjects:                  make([]pinFingerprintSubject, 0, len(ordered)),
	}
	for _, subject := range ordered {
		document.Subjects = append(document.Subjects, pinFingerprintSubject{
			ShipmentTenantID:         subject.ShipmentTenantID.String(),
			ShipmentID:               subject.ShipmentID.String(),
			ShipmentCarrierCompanyID: subject.SourceCompanyID.String(),
		})
	}
	payload, err := json.Marshal(document)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

// ActivationRefusal maps an activation reason to the HTTP class in ADR-PLATFORM-003.
func ActivationRefusal(reason string) *apperrors.AppError {
	switch reason {
	case ReasonRelationshipVersionNotAuthoritative:
		return apperrors.Validation(reason, map[string]any{"reason": reason})
	case ReasonRelationshipNotFound:
		return apperrors.NotFound(reason)
	case ReasonRelationshipStoreUnavailable:
		return &apperrors.AppError{
			Code:    apperrors.CodeRelationshipStoreUnavailable,
			Message: reason,
			Details: map[string]any{"reason": reason},
		}
	default:
		return ProjectionConflict(reason)
	}
}

// JSONContainsRelationshipAuthority reports a client relationship id or version anywhere in the body.
func JSONContainsRelationshipAuthority(body []byte) bool {
	if len(strings.TrimSpace(string(body))) == 0 {
		return false
	}
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		return false
	}
	return relationshipAuthorityValue(value)
}

func relationshipAuthorityValue(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			switch key {
			case "relationship_id", "relationship_version", "relationship_version_id":
				return true
			}
			if relationshipAuthorityValue(child) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if relationshipAuthorityValue(child) {
				return true
			}
		}
	}
	return false
}
