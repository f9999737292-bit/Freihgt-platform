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
	PolicySearchNextLoad
	PolicyViewMarketplaceCapacities
	PolicyReadCompatibility
	PolicyManageCompatibilityRules
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
	case PolicyPublishCapacity, PolicyReadOwnCapacity, PolicyWithdrawCapacity, PolicyViewMarketplaceLoads, PolicySearchNextLoad:
		return actorKind == companycontext.ActorCarrier && routeauth.HasAnyRole(companyRoles, carrierRoles)
	case PolicyReadCompatibility:
		return (actorKind == companycontext.ActorBuyer && routeauth.HasAnyRole(companyRoles, shipperRoles)) ||
			(actorKind == companycontext.ActorCarrier && routeauth.HasAnyRole(companyRoles, carrierRoles))
	case PolicyManageCompatibilityRules:
		return (actorKind == companycontext.ActorBuyer && routeauth.HasAnyRole(companyRoles, map[string]struct{}{"SHIPPER_ADMIN": {}})) ||
			(actorKind == companycontext.ActorCarrier && routeauth.HasAnyRole(companyRoles, map[string]struct{}{"CARRIER_ADMIN": {}}))
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
	case PolicyReadCompatibility:
		return "insufficient permission to evaluate compatibility"
	case PolicyManageCompatibilityRules:
		return "insufficient permission to manage compatibility rules"
	default:
		return "insufficient permission"
	}
}
