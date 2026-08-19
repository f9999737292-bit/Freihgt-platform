package settlementrbac

import (
	"net/http"

	"github.com/freight-platform/api-gateway/internal/config"
	"github.com/freight-platform/api-gateway/internal/routeauth"
)

type Policy int

const (
	PolicyRead Policy = iota
	PolicyMutate
)

type Guard struct {
	inner *routeauth.Guard
}

func NewGuard(cfg config.Config, proxy http.Handler) *Guard {
	return &Guard{inner: routeauth.NewGuard(cfg, proxy)}
}

func (g *Guard) WithPolicy(policy Policy) http.HandlerFunc {
	return g.inner.WithPolicy(routeauth.Policy{
		Allow:       policyAllow(policy),
		DenyMessage: policyDenyMessage(policy),
	})
}

var readSettlementRoles = map[string]struct{}{
	"PLATFORM_ADMIN":     {},
	"SHIPPER_ADMIN":      {},
	"SHIPPER_LOGIST":     {},
	"FORWARDER_MANAGER":  {},
	"CARRIER_ADMIN":      {},
	"CARRIER_DISPATCHER": {},
	"CARRIER_ACCOUNTANT": {},
}

var mutateSettlementRoles = map[string]struct{}{
	"PLATFORM_ADMIN":     {},
	"SHIPPER_ADMIN":      {},
	"SHIPPER_LOGIST":     {},
	"CARRIER_ADMIN":      {},
	"CARRIER_DISPATCHER": {},
	"CARRIER_ACCOUNTANT": {},
}

func policyAllow(policy Policy) func([]string) bool {
	switch policy {
	case PolicyRead:
		return func(roles []string) bool { return routeauth.HasAnyRole(roles, readSettlementRoles) }
	case PolicyMutate:
		return func(roles []string) bool { return routeauth.HasAnyRole(roles, mutateSettlementRoles) }
	default:
		return func([]string) bool { return false }
	}
}

func policyDenyMessage(policy Policy) string {
	switch policy {
	case PolicyRead:
		return "freight settlement read access denied"
	case PolicyMutate:
		return "freight settlement mutation access denied"
	default:
		return "freight settlement access denied"
	}
}
