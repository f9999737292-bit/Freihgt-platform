package domain

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
)

const (
	ShipmentSourceType = "SHIPMENT"

	ShipmentSourceConfirmPath        = "/internal/v1/shipment-source-authority/confirm"
	ShipmentSourceRevalidatePath     = "/internal/v1/shipment-source-authority/revalidate"
	ShipmentSourceRevalidationHeader = "X-Company-Relationship-Revalidation-Token"

	ShipmentSourceCreateConfirmed      = "CONFIRMED"
	ShipmentSourceCreateNotFound       = "NOT_FOUND"
	ShipmentSourceCreateTenantMismatch = "TENANT_MISMATCH"
	ShipmentSourceCreateStale          = "STALE"
	ShipmentSourceCreateActorMismatch  = "ACTOR_MISMATCH"
	ShipmentSourceCreateUnavailable    = "UNAVAILABLE"

	ShipmentSourceMatch       = "MATCH"
	ShipmentSourceNotFound    = "NOT_FOUND"
	ShipmentSourceChanged     = "CHANGED"
	ShipmentSourceUnavailable = "UNAVAILABLE"

	shipmentRoleShipperAdmin     = "SHIPPER_ADMIN"
	shipmentRoleShipperLogist    = "SHIPPER_LOGIST"
	shipmentRoleForwarderManager = "FORWARDER_MANAGER"
)

// ErrShipmentSourceMissing means the shipment primary key does not exist.
// A deleted or unassigned row is still a row and is not this error.
var ErrShipmentSourceMissing = errors.New("shipment source missing")

// ShipmentSourceRecord is the shipment row fields that form the source fingerprint.
// CarrierAssigned is false when carrier_company_id is null. Live is deleted_at IS NULL.
type ShipmentSourceRecord struct {
	TenantID         uuid.UUID
	ID               uuid.UUID
	Version          int
	ShipperCompanyID uuid.UUID
	CarrierCompanyID uuid.UUID
	CarrierAssigned  bool
	Live             bool
}

// ShipmentCreatorAuthority is the shipper-side user proof for one company.
// Tenant-global PLATFORM_ADMIN replaces only the company role, not membership.
type ShipmentCreatorAuthority struct {
	Active                    bool
	ActiveMembership          bool
	CompanyRoles              []string
	TenantGlobalPlatformAdmin bool
}

// ShipmentSourceSubject is the create-time subject company-service asks the owner to confirm.
type ShipmentSourceSubject struct {
	SourceType              string
	SourceID                uuid.UUID
	SourceTenantID          uuid.UUID
	Fingerprint             string
	ShipperCarrierCompanyID uuid.UUID
	RequestingCompanyID     uuid.UUID
	RequestingUserID        uuid.UUID
}

// ShipmentSourceSnapshot is the acceptance-time fingerprint check.
type ShipmentSourceSnapshot struct {
	SourceType              string
	SourceID                uuid.UUID
	SourceTenantID          uuid.UUID
	Fingerprint             string
	ShipperCarrierCompanyID uuid.UUID
}

type shipmentFingerprintDocument struct {
	TenantID         string `json:"tenant_id"`
	ID               string `json:"id"`
	Version          int    `json:"version"`
	ShipperCompanyID string `json:"shipper_company_id"`
	CarrierCompanyID string `json:"carrier_company_id"`
	DeletedAtNull    bool   `json:"deleted_at_null"`
}

// ShipmentSourceFingerprint is the lowercase hex SHA-256 of the canonical shipment source document.
// The document field order is tenant, id, version, shipper company, carrier company, deleted_at_null.
func ShipmentSourceFingerprint(row ShipmentSourceRecord) (string, error) {
	if !row.Live || !row.CarrierAssigned || row.CarrierCompanyID == uuid.Nil || row.ID == uuid.Nil || row.TenantID == uuid.Nil || row.ShipperCompanyID == uuid.Nil {
		return "", errors.New("shipment source is not fingerprintable")
	}
	payload, err := json.Marshal(shipmentFingerprintDocument{
		TenantID:         row.TenantID.String(),
		ID:               row.ID.String(),
		Version:          row.Version,
		ShipperCompanyID: row.ShipperCompanyID.String(),
		CarrierCompanyID: row.CarrierCompanyID.String(),
		DeletedAtNull:    true,
	})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

// ShipmentSourceFingerprintMatch compares the owner fingerprint with the stored value.
// Different lengths are a mismatch. Equal lengths use a constant-time compare.
func ShipmentSourceFingerprintMatch(current, presented string) bool {
	if current == "" || len(current) != len(presented) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(current), []byte(presented)) == 1
}

// ShipmentCreatorAuthorized is the shipment request creator predicate.
// PLATFORM_ADMIN does not replace the active user or the active shipper-company membership.
// CARRIER_ADMIN and CARRIER_DISPATCHER are not creator roles.
func ShipmentCreatorAuthorized(actor ShipmentCreatorAuthority) bool {
	if !actor.Active || !actor.ActiveMembership {
		return false
	}
	if actor.TenantGlobalPlatformAdmin {
		return true
	}
	for _, code := range actor.CompanyRoles {
		switch code {
		case shipmentRoleShipperAdmin, shipmentRoleShipperLogist, shipmentRoleForwarderManager:
			return true
		}
	}
	return false
}
