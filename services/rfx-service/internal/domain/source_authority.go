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
	RfxSourceTypeAward     = "RFX_AWARD"
	RfxSourceTypeAwardLink = "AWARD_TRANSPORT_ORDER_LINK"

	RfxSourceConfirmPath        = "/internal/v1/rfx-source-authority/confirm"
	RfxSourceRevalidatePath     = "/internal/v1/rfx-source-authority/revalidate"
	RfxSourceRevalidationHeader = "X-Company-Relationship-Revalidation-Token"

	RfxSourceCreateConfirmed      = "CONFIRMED"
	RfxSourceCreateNotFound       = "NOT_FOUND"
	RfxSourceCreateTenantMismatch = "TENANT_MISMATCH"
	RfxSourceCreateStale          = "STALE"
	RfxSourceCreateActorMismatch  = "ACTOR_MISMATCH"
	RfxSourceCreateUnavailable    = "UNAVAILABLE"

	RfxSourceMatch       = "MATCH"
	RfxSourceNotFound    = "NOT_FOUND"
	RfxSourceChanged     = "CHANGED"
	RfxSourceUnavailable = "UNAVAILABLE"

	rfxRoleProcurementManager = "PROCUREMENT_MANAGER"
	rfxRoleShipperAdmin       = "SHIPPER_ADMIN"
	rfxRoleShipperLogist      = "SHIPPER_LOGIST"
	rfxRoleForwarderManager   = "FORWARDER_MANAGER"
)

// ErrRfxSourceMissing means the award or award-link primary key does not exist.
var ErrRfxSourceMissing = errors.New("rfx source missing")

// RfxAwardSourceRecord is the current award row plus the parent event state.
// rfx.rfx_awards has no status column. Acceptable state is the parent event:
// same tenant, not deleted, and status AWARDED.
type RfxAwardSourceRecord struct {
	TenantID         uuid.UUID
	ID               uuid.UUID
	Version          int
	RfxEventID       uuid.UUID
	CarrierCompanyID uuid.UUID
	EventPresent     bool
	EventTenantID    uuid.UUID
	EventID          uuid.UUID
	OwnerCompanyID   uuid.UUID
	EventStatus      string
	EventLive        bool
}

// RfxAwardLinkSourceRecord is the current award-link row plus its parent award and event.
type RfxAwardLinkSourceRecord struct {
	TenantID         uuid.UUID
	ID               uuid.UUID
	Version          int
	RfxAwardID       uuid.UUID
	RfxEventID       uuid.UUID
	TransportOrderID uuid.UUID
	CarrierCompanyID uuid.UUID
	BuyerCompanyID   uuid.UUID
	AwardPresent     bool
	AwardTenantID    uuid.UUID
	AwardEventID     uuid.UUID
	AwardCarrierID   uuid.UUID
	EventPresent     bool
	EventTenantID    uuid.UUID
	EventID          uuid.UUID
	OwnerCompanyID   uuid.UUID
	EventStatus      string
	EventLive        bool
}

// RfxCreatorAuthority is the buyer-side user proof for the event owner company.
// Tenant-global PLATFORM_ADMIN replaces only the company role, not membership.
type RfxCreatorAuthority struct {
	Active                    bool
	ActiveMembership          bool
	CompanyRoles              []string
	TenantGlobalPlatformAdmin bool
}

// RfxSourceSubject is the create-time subject company-service asks the owner to confirm.
type RfxSourceSubject struct {
	SourceType              string
	SourceID                uuid.UUID
	SourceTenantID          uuid.UUID
	Fingerprint             string
	ShipperCarrierCompanyID uuid.UUID
	RequestingCompanyID     uuid.UUID
	RequestingUserID        uuid.UUID
}

// RfxSourceSnapshot is the acceptance-time fingerprint check.
type RfxSourceSnapshot struct {
	SourceType              string
	SourceID                uuid.UUID
	SourceTenantID          uuid.UUID
	Fingerprint             string
	ShipperCarrierCompanyID uuid.UUID
}

type rfxAwardFingerprintDocument struct {
	TenantID         string `json:"tenant_id"`
	ID               string `json:"id"`
	Version          int    `json:"version"`
	RfxEventID       string `json:"rfx_event_id"`
	CarrierCompanyID string `json:"carrier_company_id"`
}

type rfxAwardLinkFingerprintDocument struct {
	TenantID         string `json:"tenant_id"`
	ID               string `json:"id"`
	Version          int    `json:"version"`
	RfxAwardID       string `json:"rfx_award_id"`
	TransportOrderID string `json:"transport_order_id"`
	CarrierCompanyID string `json:"carrier_company_id"`
}

// AwardSourceAcceptable is the live award predicate.
// The award row has no status of its own. The parent event must be the same
// tenant, not deleted, and AWARDED. That is the status written when the award
// is created and the status required before an award link can exist.
func AwardSourceAcceptable(row RfxAwardSourceRecord) bool {
	if row.ID == uuid.Nil || row.TenantID == uuid.Nil || row.RfxEventID == uuid.Nil || row.CarrierCompanyID == uuid.Nil || row.OwnerCompanyID == uuid.Nil {
		return false
	}
	return row.EventPresent && row.EventLive && row.EventStatus == RfxStatusAwarded && row.EventTenantID == row.TenantID && row.EventID == row.RfxEventID
}

// AwardLinkSourceAcceptable is the live award-link predicate.
// The parent award must exist in the same tenant, on the same event, with the
// same carrier. The event must be an acceptable award event. The link buyer
// must be that event owner, and the transport order id must be the id stored
// on this link.
func AwardLinkSourceAcceptable(row RfxAwardLinkSourceRecord) bool {
	if row.ID == uuid.Nil || row.TenantID == uuid.Nil || row.RfxAwardID == uuid.Nil || row.RfxEventID == uuid.Nil || row.TransportOrderID == uuid.Nil || row.CarrierCompanyID == uuid.Nil || row.BuyerCompanyID == uuid.Nil {
		return false
	}
	if !row.AwardPresent || row.AwardTenantID != row.TenantID || row.AwardEventID != row.RfxEventID || row.AwardCarrierID != row.CarrierCompanyID {
		return false
	}
	if !row.EventPresent || !row.EventLive || row.EventStatus != RfxStatusAwarded || row.EventTenantID != row.TenantID || row.EventID != row.RfxEventID || row.OwnerCompanyID == uuid.Nil {
		return false
	}
	return row.BuyerCompanyID == row.OwnerCompanyID
}

// RfxAwardSourceFingerprint is the lowercase hex SHA-256 of the canonical award document.
// The document field order is tenant, id, version, rfx event, carrier company.
func RfxAwardSourceFingerprint(row RfxAwardSourceRecord) (string, error) {
	if row.ID == uuid.Nil || row.TenantID == uuid.Nil || row.RfxEventID == uuid.Nil || row.CarrierCompanyID == uuid.Nil {
		return "", errors.New("rfx award source is not fingerprintable")
	}
	payload, err := json.Marshal(rfxAwardFingerprintDocument{
		TenantID:         row.TenantID.String(),
		ID:               row.ID.String(),
		Version:          row.Version,
		RfxEventID:       row.RfxEventID.String(),
		CarrierCompanyID: row.CarrierCompanyID.String(),
	})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

// RfxAwardLinkSourceFingerprint is the lowercase hex SHA-256 of the canonical award-link document.
// The document field order is tenant, id, version, award, transport order, carrier company.
func RfxAwardLinkSourceFingerprint(row RfxAwardLinkSourceRecord) (string, error) {
	if row.ID == uuid.Nil || row.TenantID == uuid.Nil || row.RfxAwardID == uuid.Nil || row.TransportOrderID == uuid.Nil || row.CarrierCompanyID == uuid.Nil {
		return "", errors.New("rfx award link source is not fingerprintable")
	}
	payload, err := json.Marshal(rfxAwardLinkFingerprintDocument{
		TenantID:         row.TenantID.String(),
		ID:               row.ID.String(),
		Version:          row.Version,
		RfxAwardID:       row.RfxAwardID.String(),
		TransportOrderID: row.TransportOrderID.String(),
		CarrierCompanyID: row.CarrierCompanyID.String(),
	})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

// RfxSourceFingerprintMatch compares the owner fingerprint with the stored value.
// Different lengths are a mismatch. Equal lengths use a constant-time compare.
func RfxSourceFingerprintMatch(current, presented string) bool {
	if current == "" || len(current) != len(presented) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(current), []byte(presented)) == 1
}

// RfxCreatorAuthorized is the award and award-link request creator predicate.
// PLATFORM_ADMIN does not replace the active user or the active owner-company membership.
// A company-scoped PLATFORM_ADMIN row is not the tenant-global override.
// CARRIER_ADMIN does not create the shipper-side request.
func RfxCreatorAuthorized(actor RfxCreatorAuthority) bool {
	if !actor.Active || !actor.ActiveMembership {
		return false
	}
	if actor.TenantGlobalPlatformAdmin {
		return true
	}
	for _, code := range actor.CompanyRoles {
		switch code {
		case rfxRoleProcurementManager, rfxRoleShipperAdmin, rfxRoleShipperLogist, rfxRoleForwarderManager:
			return true
		}
	}
	return false
}
