package transportorderrbac

import (
	"strings"

	"github.com/freight-platform/api-gateway/internal/companycontext"
	"github.com/freight-platform/api-gateway/internal/routeauth"
)

const rolePlatformAdmin = "PLATFORM_ADMIN"

// Policy covers public transport order routes enforced at API Gateway.
type Policy int

const (
	PolicyCreate Policy = iota
	PolicyRead
	PolicyList
	PolicyMutate
)

var createRoles = map[string]struct{}{
	rolePlatformAdmin:     {},
	"PROCUREMENT_MANAGER": {},
	"SHIPPER_ADMIN":       {},
	"FORWARDER_MANAGER":   {},
}

var readRoles = map[string]struct{}{
	rolePlatformAdmin:     {},
	"PROCUREMENT_MANAGER": {},
	"SHIPPER_ADMIN":       {},
	"SHIPPER_LOGIST":      {},
	"FORWARDER_MANAGER":   {},
}

var carrierReadRoles = map[string]struct{}{
	"CARRIER_ADMIN":      {},
	"CARRIER_DISPATCHER": {},
}

func hasPlatformAdmin(roles []string) bool {
	for _, role := range roles {
		if strings.EqualFold(strings.TrimSpace(role), rolePlatformAdmin) {
			return true
		}
	}
	return false
}

func policyAllows(policy Policy, companyRoles []string, actorKind string, isPlatformAdmin bool) bool {
	switch policy {
	case PolicyCreate, PolicyMutate:
		if actorKind != companycontext.ActorBuyer {
			return isPlatformAdmin
		}
		if isPlatformAdmin {
			return true
		}
		return routeauth.HasAnyRole(companyRoles, createRoles)
	case PolicyRead:
		if isPlatformAdmin {
			return true
		}
		if actorKind == companycontext.ActorBuyer {
			return routeauth.HasAnyRole(companyRoles, readRoles)
		}
		if actorKind == companycontext.ActorCarrier {
			return routeauth.HasAnyRole(companyRoles, carrierReadRoles)
		}
		return false
	case PolicyList:
		if isPlatformAdmin {
			return true
		}
		if actorKind != companycontext.ActorBuyer {
			return false
		}
		return routeauth.HasAnyRole(companyRoles, readRoles)
	default:
		return false
	}
}

func policyDenyMessage(policy Policy) string {
	switch policy {
	case PolicyCreate:
		return "transport order create access denied"
	case PolicyRead:
		return "transport order read access denied"
	case PolicyList:
		return "transport order list access denied"
	case PolicyMutate:
		return "transport order mutation access denied"
	default:
		return "transport order access denied"
	}
}

func policyMetricName(policy Policy) string {
	switch policy {
	case PolicyCreate:
		return "create"
	case PolicyRead:
		return "read"
	case PolicyList:
		return "list"
	case PolicyMutate:
		return "mutate"
	default:
		return "unknown"
	}
}
