package domain

import (
	"strings"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/billing-register-service/internal/platform/errors"
)

const (
	HeaderCompanyID  = "X-Company-ID"
	HeaderActorKind  = "X-Actor-Kind"
	RolePlatformAdmin = "PLATFORM_ADMIN"
)

var buyerCompanyTypes = map[string]struct{}{
	"SHIPPER":   {},
	"FORWARDER": {},
	"LSP":       {},
}

var carrierCompanyTypes = map[string]struct{}{
	"CARRIER": {},
}

// UserCompanyMembership is the canonical membership view for settlement actor resolution.
type UserCompanyMembership struct {
	CompanyID   uuid.UUID
	CompanyType string
	RoleCodes   []string
}

// DeriveSettlementActorKind maps verified company type and roles to billing actor kind.
func DeriveSettlementActorKind(companyType string, roleCodes []string) (string, error) {
	for _, code := range roleCodes {
		switch strings.ToUpper(strings.TrimSpace(code)) {
		case RolePlatformAdmin, "PROCUREMENT_MANAGER", "SHIPPER_ADMIN", "SHIPPER_LOGIST", "FORWARDER_MANAGER":
			return SettlementActorBuyer, nil
		case "CARRIER_ADMIN", "CARRIER_DISPATCHER", "CARRIER_ACCOUNTANT":
			return SettlementActorCarrier, nil
		}
	}
	typ := strings.ToUpper(strings.TrimSpace(companyType))
	if _, ok := buyerCompanyTypes[typ]; ok {
		return SettlementActorBuyer, nil
	}
	if _, ok := carrierCompanyTypes[typ]; ok {
		return SettlementActorCarrier, nil
	}
	return "", apperrors.Forbidden("company type cannot participate in freight billing")
}

// ResolveTrustedSettlementActor validates trusted gateway headers against membership SSOT.
func ResolveTrustedSettlementActor(
	tenantID, userID uuid.UUID,
	trustedCompanyID uuid.UUID,
	trustedActorKind string,
	memberships []UserCompanyMembership,
	isPlatformAdmin bool,
) (SettlementActorInput, error) {
	if tenantID == uuid.Nil {
		return SettlementActorInput{}, apperrors.Unauthorized("tenant context is required")
	}
	if userID == uuid.Nil {
		return SettlementActorInput{}, apperrors.Unauthorized("user context is required")
	}
	if trustedCompanyID == uuid.Nil {
		return SettlementActorInput{}, apperrors.Forbidden("verified company context is required")
	}
	headerKind := strings.ToUpper(strings.TrimSpace(trustedActorKind))
	if headerKind != SettlementActorBuyer && headerKind != SettlementActorCarrier {
		return SettlementActorInput{}, apperrors.Forbidden("verified actor context is required")
	}

	var matched *UserCompanyMembership
	for i := range memberships {
		if memberships[i].CompanyID == trustedCompanyID {
			matched = &memberships[i]
			break
		}
	}
	if matched == nil {
		if !isPlatformAdmin {
			return SettlementActorInput{}, apperrors.Forbidden("company_id does not match authenticated membership")
		}
		return SettlementActorInput{}, apperrors.Forbidden("platform admin must act within a validated company membership")
	}

	derivedKind, err := DeriveSettlementActorKind(matched.CompanyType, matched.RoleCodes)
	if err != nil {
		return SettlementActorInput{}, err
	}
	if derivedKind != headerKind {
		return SettlementActorInput{}, apperrors.Forbidden("actor kind does not match verified company membership")
	}

	return SettlementActorInput{
		TenantID:       tenantID,
		ActorCompanyID: trustedCompanyID,
		ActorKind:      derivedKind,
		ActorUserID:    userID,
	}, nil
}

func HasPlatformAdminRole(roleCodes []string) bool {
	for _, code := range roleCodes {
		if strings.EqualFold(strings.TrimSpace(code), RolePlatformAdmin) {
			return true
		}
	}
	return false
}
