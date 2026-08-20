package domain

import (
	"strings"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/contract-rate-service/internal/platform/errors"
)

const MoneyScale = 2

const (
	HeaderTenantID   = "X-Tenant-ID"
	HeaderUserID     = "X-User-ID"
	HeaderCompanyID  = "X-Company-ID"
	HeaderActorKind  = "X-Actor-Kind"
	HeaderRequestID  = "X-Request-ID"
)

const (
	ActorKindBuyer   = "BUYER"
	ActorKindCarrier = "CARRIER"
)

const RolePlatformAdmin = "PLATFORM_ADMIN"

type UserCompanyMembership struct {
	CompanyID   uuid.UUID
	CompanyType string
	RoleCodes   []string
}

type ActorInput struct {
	TenantID       uuid.UUID
	ActorUserID    uuid.UUID
	ActorCompanyID uuid.UUID
	ActorKind      string
	IsPlatformAdmin bool
}

var buyerCompanyTypes = map[string]struct{}{
	"SHIPPER":   {},
	"FORWARDER": {},
	"LSP":       {},
}

var carrierCompanyTypes = map[string]struct{}{
	"CARRIER": {},
}

func HasPlatformAdminRole(roleCodes []string) bool {
	for _, code := range roleCodes {
		if strings.EqualFold(strings.TrimSpace(code), RolePlatformAdmin) {
			return true
		}
	}
	return false
}

func DeriveActorKind(companyType string, roleCodes []string) (string, error) {
	for _, code := range roleCodes {
		switch strings.ToUpper(strings.TrimSpace(code)) {
		case RolePlatformAdmin, "PROCUREMENT_MANAGER", "SHIPPER_ADMIN", "SHIPPER_LOGIST", "FORWARDER_MANAGER":
			return ActorKindBuyer, nil
		case "CARRIER_ADMIN", "CARRIER_DISPATCHER", "CARRIER_ACCOUNTANT":
			return ActorKindCarrier, nil
		}
	}
	typ := strings.ToUpper(strings.TrimSpace(companyType))
	if _, ok := buyerCompanyTypes[typ]; ok {
		return ActorKindBuyer, nil
	}
	if _, ok := carrierCompanyTypes[typ]; ok {
		return ActorKindCarrier, nil
	}
	return "", apperrors.Forbidden("company type cannot participate in contract rate management", nil)
}

func ResolveTrustedActor(
	tenantID, userID uuid.UUID,
	trustedCompanyID uuid.UUID,
	trustedActorKind string,
	memberships []UserCompanyMembership,
	isPlatformAdmin bool,
) (ActorInput, error) {
	if tenantID == uuid.Nil {
		return ActorInput{}, apperrors.Unauthorized("tenant context is required")
	}
	if userID == uuid.Nil {
		return ActorInput{}, apperrors.Unauthorized("user context is required")
	}
	if trustedCompanyID == uuid.Nil {
		return ActorInput{}, apperrors.Forbidden("verified company context is required", nil)
	}
	headerKind := strings.ToUpper(strings.TrimSpace(trustedActorKind))
	if headerKind != ActorKindBuyer && headerKind != ActorKindCarrier {
		return ActorInput{}, apperrors.Forbidden("verified actor context is required", nil)
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
			return ActorInput{}, apperrors.Forbidden("company_id does not match authenticated membership", nil)
		}
		return ActorInput{}, apperrors.Forbidden("platform admin must act within a validated company membership", nil)
	}

	derivedKind, err := DeriveActorKind(matched.CompanyType, matched.RoleCodes)
	if err != nil {
		return ActorInput{}, err
	}
	if derivedKind != headerKind {
		return ActorInput{}, apperrors.Forbidden("actor kind does not match verified company membership", nil)
	}

	return ActorInput{
		TenantID:        tenantID,
		ActorUserID:     userID,
		ActorCompanyID:  trustedCompanyID,
		ActorKind:       derivedKind,
		IsPlatformAdmin: isPlatformAdmin,
	}, nil
}

func (a ActorInput) RequireBuyerMutation() error {
	if a.IsPlatformAdmin {
		return nil
	}
	if a.ActorKind != ActorKindBuyer {
		return apperrors.Forbidden("buyer-side authorization required", nil)
	}
	return nil
}

func (a ActorInput) CanReadContract(buyerCompanyID, carrierCompanyID uuid.UUID) error {
	if a.IsPlatformAdmin {
		return nil
	}
	switch a.ActorKind {
	case ActorKindBuyer:
		if a.ActorCompanyID != buyerCompanyID {
			return apperrors.Forbidden("contract is outside buyer scope", nil)
		}
	case ActorKindCarrier:
		if a.ActorCompanyID != carrierCompanyID {
			return apperrors.Forbidden("contract is outside carrier scope", nil)
		}
	default:
		return apperrors.Forbidden("verified actor context is required", nil)
	}
	return nil
}

func ParseUUID(raw, field string) (uuid.UUID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return uuid.Nil, apperrors.Validation(field+" is required", map[string]any{"field": field})
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, apperrors.Validation("invalid "+field, map[string]any{"field": field})
	}
	return id, nil
}
