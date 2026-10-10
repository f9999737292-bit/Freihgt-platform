package companycontext

import (
	"strings"

	apperrors "github.com/freight-platform/api-gateway/internal/platform/errors"
	"github.com/freight-platform/api-gateway/internal/routeauth"
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

// carrierReadRoleCodes are the fleet and carrier-list roles that must be
// present on the selected carrier membership. They match fleetrbac PolicyView
// except PLATFORM_ADMIN, which is not a company-scoped carrier role.
// CARRIER_ACCOUNTANT is intentionally absent.
var carrierReadRoleCodes = map[string]struct{}{
	"CARRIER_ADMIN":      {},
	"CARRIER_DISPATCHER": {},
}

// executionBuyerRoleCodes are the execution-detail buyer roles that must be
// present on the selected membership. They match executionrbac PolicyRead
// buyer and platform-admin roles. PROCUREMENT_MANAGER is not included.
var executionBuyerRoleCodes = map[string]struct{}{
	"PLATFORM_ADMIN":    {},
	"SHIPPER_ADMIN":     {},
	"SHIPPER_LOGIST":    {},
	"FORWARDER_MANAGER": {},
}

// MembershipAllowsCarrierRead is true only when the selected membership is a
// carrier company and that same membership includes CARRIER_ADMIN or
// CARRIER_DISPATCHER. A role on another company, CARRIER_ACCOUNTANT, and
// DRIVER are not sufficient.
func MembershipAllowsCarrierRead(companyType string, roleCodes []string) bool {
	if strings.ToUpper(strings.TrimSpace(companyType)) != "CARRIER" {
		return false
	}
	return routeauth.HasAnyRole(normalizedRoleCodes(roleCodes), carrierReadRoleCodes)
}

// shipperShipmentReadRoleCodes are the customer shipment-read roles that must
// be present on the selected shipper membership. FORWARDER_MANAGER and
// PROCUREMENT_MANAGER are not shipper customer roles.
var shipperShipmentReadRoleCodes = map[string]struct{}{
	"SHIPPER_ADMIN":  {},
	"SHIPPER_LOGIST": {},
}

// MembershipAllowsShipperShipmentRead is true only when the selected
// membership is a shipper company and that same membership includes
// SHIPPER_ADMIN or SHIPPER_LOGIST. A role on another company, a forwarder or
// LSP company, and tenant-global PLATFORM_ADMIN are not sufficient.
func MembershipAllowsShipperShipmentRead(companyType string, roleCodes []string) bool {
	if strings.ToUpper(strings.TrimSpace(companyType)) != "SHIPPER" {
		return false
	}
	return routeauth.HasAnyRole(normalizedRoleCodes(roleCodes), shipperShipmentReadRoleCodes)
}

func normalizedRoleCodes(roleCodes []string) []string {
	out := make([]string, 0, len(roleCodes))
	for _, code := range roleCodes {
		code = strings.ToUpper(strings.TrimSpace(code))
		if code != "" {
			out = append(out, code)
		}
	}
	return out
}

func hasTenantPlatformAdmin(roleCodes []string) bool {
	return routeauth.HasAnyRole(normalizedRoleCodes(roleCodes), map[string]struct{}{
		"PLATFORM_ADMIN": {},
	})
}

// ExecutionActorFromMembership derives the execution actor from the selected
// membership only. A carrier operating role on that carrier company wins.
// Otherwise a buyer or platform-admin role on that same membership yields BUYER.
func ExecutionActorFromMembership(companyType string, roleCodes []string) (string, error) {
	roles := normalizedRoleCodes(roleCodes)
	if MembershipAllowsCarrierRead(companyType, roles) {
		return ActorCarrier, nil
	}
	if routeauth.HasAnyRole(roles, executionBuyerRoleCodes) {
		return ActorBuyer, nil
	}
	return "", apperrors.Forbidden("company is not authorized for carrier access")
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
