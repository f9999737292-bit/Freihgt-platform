package paymentrbac

import (
	"net/http"

	"github.com/freight-platform/api-gateway/internal/companycontext"
	"github.com/freight-platform/api-gateway/internal/config"
	"github.com/freight-platform/api-gateway/internal/routeauth"
)

type Policy int

const (
	PolicyRead Policy = iota
	PolicyCreate
	PolicyAllocate
	PolicyReconcile
)

type Guard struct {
	inner *routeauth.Guard
}

func NewGuard(cfg config.Config, proxy http.Handler) *Guard {
	enforced := companycontext.NewEnforcer(cfg).Middleware(proxy)
	return &Guard{inner: routeauth.NewGuard(cfg, enforced)}
}

func (g *Guard) WithPolicy(policy Policy) http.HandlerFunc {
	return g.inner.WithPolicy(routeauth.Policy{
		Allow:       policyAllow(policy),
		DenyMessage: policyDenyMessage(policy),
	})
}

var readPaymentRoles = map[string]struct{}{
	"PLATFORM_ADMIN":     {},
	"SHIPPER_ADMIN":      {},
	"SHIPPER_LOGIST":     {},
	"FINANCE_MANAGER":    {},
	"FORWARDER_MANAGER":  {},
	"CARRIER_ADMIN":      {},
	"CARRIER_ACCOUNTANT": {},
}

var createPaymentRoles = map[string]struct{}{
	"PLATFORM_ADMIN":     {},
	"SHIPPER_ADMIN":      {},
	"FINANCE_MANAGER":    {},
	"CARRIER_ADMIN":      {},
	"CARRIER_ACCOUNTANT": {},
}

var allocatePaymentRoles = map[string]struct{}{
	"PLATFORM_ADMIN":     {},
	"SHIPPER_ADMIN":      {},
	"FINANCE_MANAGER":    {},
	"CARRIER_ADMIN":      {},
	"CARRIER_ACCOUNTANT": {},
}

var reconcilePaymentRoles = map[string]struct{}{
	"PLATFORM_ADMIN":     {},
	"SHIPPER_ADMIN":      {},
	"FINANCE_MANAGER":    {},
	"CARRIER_ADMIN":      {},
	"CARRIER_ACCOUNTANT": {},
}

func policyAllow(policy Policy) func([]string) bool {
	switch policy {
	case PolicyRead:
		return func(roles []string) bool { return routeauth.HasAnyRole(roles, readPaymentRoles) }
	case PolicyCreate:
		return func(roles []string) bool { return routeauth.HasAnyRole(roles, createPaymentRoles) }
	case PolicyAllocate:
		return func(roles []string) bool { return routeauth.HasAnyRole(roles, allocatePaymentRoles) }
	case PolicyReconcile:
		return func(roles []string) bool { return routeauth.HasAnyRole(roles, reconcilePaymentRoles) }
	default:
		return func([]string) bool { return false }
	}
}

func policyDenyMessage(policy Policy) string {
	switch policy {
	case PolicyRead:
		return "payment read access denied"
	case PolicyCreate:
		return "payment create access denied"
	case PolicyAllocate:
		return "payment allocation access denied"
	case PolicyReconcile:
		return "payment reconciliation access denied"
	default:
		return "payment access denied"
	}
}
