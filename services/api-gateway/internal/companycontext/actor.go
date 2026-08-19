package companycontext

import (
	"strings"

	apperrors "github.com/freight-platform/api-gateway/internal/platform/errors"
)

const (
	HeaderCompanyID = "X-Company-ID"
	HeaderActorKind = "X-Actor-Kind"

	ActorBuyer   = "BUYER"
	ActorCarrier = "CARRIER"
)

var buyerCompanyTypes = map[string]struct{}{
	"SHIPPER":   {},
	"FORWARDER": {},
	"LSP":       {},
}

var carrierCompanyTypes = map[string]struct{}{
	"CARRIER": {},
}

func DeriveActorKind(companyType string, roleCodes []string) (string, error) {
	for _, code := range roleCodes {
		switch strings.ToUpper(strings.TrimSpace(code)) {
		case "PLATFORM_ADMIN", "PROCUREMENT_MANAGER", "SHIPPER_ADMIN", "SHIPPER_LOGIST", "FORWARDER_MANAGER":
			return ActorBuyer, nil
		case "CARRIER_ADMIN", "CARRIER_DISPATCHER", "CARRIER_ACCOUNTANT":
			return ActorCarrier, nil
		}
	}
	typ := strings.ToUpper(strings.TrimSpace(companyType))
	if _, ok := buyerCompanyTypes[typ]; ok {
		return ActorBuyer, nil
	}
	if _, ok := carrierCompanyTypes[typ]; ok {
		return ActorCarrier, nil
	}
	return "", apperrors.Forbidden("company type cannot participate in freight billing")
}

func StripUntrustedCompanyHeaders(header interface {
	Del(key string)
}) {
	header.Del(HeaderCompanyID)
	header.Del(HeaderActorKind)
}
