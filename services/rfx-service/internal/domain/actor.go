package domain

import (
	"strings"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

// ActorContext carries trusted identity from API Gateway headers.
type ActorContext struct {
	TenantID uuid.UUID
	UserID   uuid.UUID
}

func (a ActorContext) Validate() error {
	if a.TenantID == uuid.Nil {
		return apperrors.Unauthorized("tenant context is required")
	}
	return nil
}

// ActorKind classifies caller for bid/response visibility rules.
type ActorKind int

const (
	ActorKindUnknown ActorKind = iota
	ActorKindBuyer
	ActorKindCarrier
)

var buyerCompanyTypes = map[string]struct{}{
	"SHIPPER":   {},
	"FORWARDER": {},
	"LSP":       {},
}

var carrierCompanyTypes = map[string]struct{}{
	"CARRIER": {},
}

func ClassifyActorKind(companyTypes []string, roleCodes []string) ActorKind {
	for _, code := range roleCodes {
		switch strings.ToUpper(strings.TrimSpace(code)) {
		case "PLATFORM_ADMIN", "PROCUREMENT_MANAGER", "SHIPPER_ADMIN", "SHIPPER_LOGIST", "FORWARDER_MANAGER":
			return ActorKindBuyer
		case "CARRIER_ADMIN", "CARRIER_DISPATCHER":
			return ActorKindCarrier
		}
	}
	for _, t := range companyTypes {
		if _, ok := buyerCompanyTypes[strings.ToUpper(strings.TrimSpace(t))]; ok {
			return ActorKindBuyer
		}
	}
	for _, t := range companyTypes {
		if _, ok := carrierCompanyTypes[strings.ToUpper(strings.TrimSpace(t))]; ok {
			return ActorKindCarrier
		}
	}
	return ActorKindUnknown
}

func ResolveCarrierCompanyID(requested uuid.UUID, memberships []uuid.UUID) (uuid.UUID, error) {
	if len(memberships) == 0 {
		return uuid.Nil, apperrors.Forbidden("carrier company membership is required")
	}
	if requested != uuid.Nil {
		for _, id := range memberships {
			if id == requested {
				return id, nil
			}
		}
		return uuid.Nil, apperrors.Forbidden("carrier_company_id does not match authenticated membership")
	}
	if len(memberships) == 1 {
		return memberships[0], nil
	}
	return uuid.Nil, apperrors.Validation("carrier_company_id is required when user belongs to multiple carrier companies", map[string]any{"field": "carrier_company_id"})
}

func CanViewAllBids(kind ActorKind) bool {
	return kind == ActorKindBuyer
}

func CanViewBid(kind ActorKind, actorCarrierCompanyID, bidCarrierCompanyID uuid.UUID) bool {
	if kind == ActorKindBuyer {
		return true
	}
	if kind == ActorKindCarrier {
		return actorCarrierCompanyID != uuid.Nil && actorCarrierCompanyID == bidCarrierCompanyID
	}
	return false
}
