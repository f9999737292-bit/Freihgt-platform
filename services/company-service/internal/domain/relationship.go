package domain

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
)

const (
	UserStatusActive = "ACTIVE"

	SourceTypeShipment  = "SHIPMENT"
	SourceTypeRfxAward  = "RFX_AWARD"
	SourceTypeAwardLink = "AWARD_TRANSPORT_ORDER_LINK"

	RequestStatusPending   = "PENDING"
	RequestStatusAccepted  = "ACCEPTED"
	RequestStatusExpired   = "EXPIRED"
	RequestStatusCancelled = "CANCELLED"

	RelationshipStatusActive  = "ACTIVE"
	RelationshipStatusRevoked = "REVOKED"

	RoleShipperAdmin       = "SHIPPER_ADMIN"
	RoleShipperLogist      = "SHIPPER_LOGIST"
	RoleForwarderManager   = "FORWARDER_MANAGER"
	RoleProcurementManager = "PROCUREMENT_MANAGER"
	RoleCarrierAdmin       = "CARRIER_ADMIN"

	CallerShipmentService = "shipment-service"
	CallerRfxService      = "rfx-service"

	SourceMatch       SourceDecision = "MATCH"
	SourceNotFound    SourceDecision = "NOT_FOUND"
	SourceChanged     SourceDecision = "CHANGED"
	SourceUnavailable SourceDecision = "UNAVAILABLE"

	SourceCreateConfirmed      SourceCreateDecision = "CONFIRMED"
	SourceCreateNotFound       SourceCreateDecision = "NOT_FOUND"
	SourceCreateTenantMismatch SourceCreateDecision = "TENANT_MISMATCH"
	SourceCreateStale          SourceCreateDecision = "STALE"
	SourceCreateActorMismatch  SourceCreateDecision = "ACTOR_MISMATCH"
	SourceCreateUnavailable    SourceCreateDecision = "UNAVAILABLE"
)

type SourceDecision string

type SourceCreateDecision string

type ActorAuthority struct {
	Active                    bool
	ActiveMembership          bool
	CompanyRoles              []string
	TenantGlobalPlatformAdmin bool
}

type Principal struct {
	UserID   uuid.UUID
	TenantID uuid.UUID
}

type SourceSnapshot struct {
	SourceType              string
	SourceID                uuid.UUID
	SourceTenantID          uuid.UUID
	Fingerprint             string
	ShipperCarrierCompanyID uuid.UUID
}

type SourceRevalidator interface {
	Revalidate(ctx context.Context, source SourceSnapshot) (SourceDecision, error)
}

type SourceCreateSubject struct {
	CallerService           string
	SourceType              string
	SourceID                uuid.UUID
	SourceTenantID          uuid.UUID
	Fingerprint             string
	ShipperCarrierCompanyID uuid.UUID
	RequestingCompanyID     uuid.UUID
	RequestingUserID        uuid.UUID
}

type SourceCreateAuthority interface {
	ConfirmCreate(ctx context.Context, subject SourceCreateSubject) (SourceCreateDecision, error)
}

type PersistCreate struct {
	CallerService           string
	SourceType              string
	SourceID                uuid.UUID
	SourceTenantID          uuid.UUID
	Fingerprint             string
	ShipperCarrierCompanyID uuid.UUID
	RequestingCompanyID     uuid.UUID
	RequestingUserID        uuid.UUID
	IdempotencyKey          string
	RequestHash             []byte
	TokenHash               []byte
	CorrelationID           string
}

type PersistCreateResult struct {
	RequestID uuid.UUID
	Status    string
	Created   bool
}

type PersistAccept struct {
	ActorTenantID      uuid.UUID
	ActorUserID        uuid.UUID
	OperatingCompanyID uuid.UUID
	IdempotencyKey     string
	BodyHash           []byte
	TokenHash          []byte
	TokenPresent       bool
	RequestID          *uuid.UUID
	CorrelationID      string
}

type PersistAcceptResult struct {
	RelationshipID      uuid.UUID
	RelationshipVersion int
	RequestStatus       string
	Created             bool
}

type VisibleRequest struct {
	RequestID uuid.UUID
	Status    string
}

func CanCreateRelationshipRequest(actor ActorAuthority, sourceType string) bool {
	if !actor.Active || !actor.ActiveMembership {
		return false
	}
	if actor.TenantGlobalPlatformAdmin {
		return true
	}
	for _, code := range actor.CompanyRoles {
		if creatorRole(sourceType, code) {
			return true
		}
	}
	return false
}

func CanAcceptRelationship(actor ActorAuthority) bool {
	if !actor.Active || !actor.ActiveMembership {
		return false
	}
	if actor.TenantGlobalPlatformAdmin {
		return true
	}
	for _, code := range actor.CompanyRoles {
		if code == RoleCarrierAdmin {
			return true
		}
	}
	return false
}

func CanReadRelationshipRequest(actor ActorAuthority) bool {
	if !actor.Active || !actor.ActiveMembership {
		return false
	}
	if actor.TenantGlobalPlatformAdmin {
		return true
	}
	for _, code := range actor.CompanyRoles {
		switch code {
		case RoleShipperAdmin, RoleShipperLogist, RoleForwarderManager, RoleProcurementManager, RoleCarrierAdmin:
			return true
		}
	}
	return false
}

func creatorRole(sourceType, code string) bool {
	switch sourceType {
	case SourceTypeShipment:
		return code == RoleShipperAdmin || code == RoleShipperLogist || code == RoleForwarderManager
	case SourceTypeRfxAward, SourceTypeAwardLink:
		return code == RoleProcurementManager || code == RoleShipperAdmin || code == RoleShipperLogist || code == RoleForwarderManager
	default:
		return false
	}
}

func ValidSourceType(sourceType string) bool {
	switch sourceType {
	case SourceTypeShipment, SourceTypeRfxAward, SourceTypeAwardLink:
		return true
	default:
		return false
	}
}

func CallerAllowsSource(callerService, sourceType string) bool {
	switch callerService {
	case CallerShipmentService:
		return sourceType == SourceTypeShipment
	case CallerRfxService:
		return sourceType == SourceTypeRfxAward || sourceType == SourceTypeAwardLink
	default:
		return false
	}
}

func CreateRequestHash(sourceType string, sourceID uuid.UUID, fingerprint string, carrierCompanyID uuid.UUID) []byte {
	payload := map[string]string{
		"shipper_carrier_company_id":    carrierCompanyID.String(),
		"source_id":                     sourceID.String(),
		"source_type":                   sourceType,
		"source_version_or_fingerprint": fingerprint,
	}
	raw, _ := json.Marshal(payload)
	sum := sha256.Sum256(raw)
	return sum[:]
}

func AcceptBodyHash(companyID uuid.UUID, requestID *uuid.UUID) []byte {
	locator := ""
	if requestID != nil {
		locator = requestID.String()
	}
	payload := map[string]string{
		"operating_company_id":    companyID.String(),
		"relationship_request_id": locator,
	}
	raw, _ := json.Marshal(payload)
	sum := sha256.Sum256(raw)
	return sum[:]
}

func NewAcceptanceToken() ([]byte, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func EncodeAcceptanceToken(raw []byte) string {
	return base64.RawURLEncoding.EncodeToString(raw)
}

func DecodeAcceptanceToken(encoded string) ([]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(raw) != 32 {
		return nil, errors.New("invalid acceptance token")
	}
	return raw, nil
}

func FingerprintAcceptanceToken(pepper string, raw []byte) []byte {
	mac := hmac.New(sha256.New, []byte(pepper))
	_, _ = mac.Write(raw)
	return mac.Sum(nil)
}
