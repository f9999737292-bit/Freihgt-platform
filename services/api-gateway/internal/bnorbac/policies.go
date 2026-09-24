package bnorbac

import (
	"strings"

	"github.com/freight-platform/api-gateway/internal/companycontext"
	"github.com/freight-platform/api-gateway/internal/routeauth"
)

type Policy int

const (
	PolicyPublishLoad Policy = iota
	PolicyReadOwnLoad
	PolicyWithdrawLoad
	PolicyPublishCapacity
	PolicyReadOwnCapacity
	PolicyWithdrawCapacity
	PolicyViewMarketplaceLoads
	PolicyViewMarketplaceCapacities
)

var shipperRoles = map[string]struct{}{
	"SHIPPER_ADMIN":     {},
	"SHIPPER_LOGIST":    {},
	"FORWARDER_MANAGER": {},
}

var carrierRoles = map[string]struct{}{
	"CARRIER_ADMIN":      {},
	"CARRIER_DISPATCHER": {},
}

func hasPlatformAdmin(roles []string) bool {
	for _, role := range roles {
		if strings.EqualFold(strings.TrimSpace(role), "PLATFORM_ADMIN") {
			return true
		}
	}
	return false
}

func policyAllows(policy Policy, companyRoles []string, actorKind string, isPlatformAdmin bool) bool {
	if isPlatformAdmin || routeauth.HasAnyRole(companyRoles, map[string]struct{}{"PLATFORM_ADMIN": {}}) {
		return true
	}
	switch policy {
	case PolicyPublishLoad, PolicyReadOwnLoad, PolicyWithdrawLoad, PolicyViewMarketplaceCapacities:
		return actorKind == companycontext.ActorBuyer && routeauth.HasAnyRole(companyRoles, shipperRoles)
	case PolicyPublishCapacity, PolicyReadOwnCapacity, PolicyWithdrawCapacity, PolicyViewMarketplaceLoads:
		return actorKind == companycontext.ActorCarrier && routeauth.HasAnyRole(companyRoles, carrierRoles)
	default:
		return false
	}
}

func policyDenyMessage(policy Policy) string {
	switch policy {
	case PolicyPublishLoad:
		return "insufficient permission to publish a load opportunity"
	case PolicyWithdrawLoad:
		return "insufficient permission to withdraw a load opportunity"
	case PolicyPublishCapacity:
		return "insufficient permission to publish capacity"
	case PolicyWithdrawCapacity:
		return "insufficient permission to withdraw capacity"
	case PolicyViewMarketplaceLoads:
		return "insufficient permission to view marketplace loads"
	case PolicyViewMarketplaceCapacities:
		return "insufficient permission to view marketplace capacity"
	default:
		return "insufficient permission"
	}
}
