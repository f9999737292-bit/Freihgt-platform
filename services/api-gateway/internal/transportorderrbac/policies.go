package transportorderrbac

import (
	"strings"

	"github.com/freight-platform/api-gateway/internal/routeauth"
)

const rolePlatformAdmin = "PLATFORM_ADMIN"

// PolicyCreate covers priced transport order creation (POST /api/v1/transport-orders).
type Policy int

const (
	PolicyCreate Policy = iota
)

// createRoles — canonical buyer-side roles allowed to create priced transport orders.
// Aligned with RFx buyer-manage and contract-rate mutate paths (Wave2 shipper path).
var createRoles = map[string]struct{}{
	rolePlatformAdmin:     {},
	"PROCUREMENT_MANAGER": {},
	"SHIPPER_ADMIN":       {},
	"FORWARDER_MANAGER":   {},
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
	case PolicyCreate:
		if actorKind != "BUYER" {
			return false
		}
		if isPlatformAdmin {
			return true
		}
		return routeauth.HasAnyRole(companyRoles, createRoles)
	default:
		return false
	}
}

func policyDenyMessage(policy Policy) string {
	switch policy {
	case PolicyCreate:
		return "transport order create access denied"
	default:
		return "transport order access denied"
	}
}

func policyMetricName(policy Policy) string {
	switch policy {
	case PolicyCreate:
		return "create"
	default:
		return "unknown"
	}
}
