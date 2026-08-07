package fleetrbac

import (
	"net/http"

	"github.com/freight-platform/api-gateway/internal/config"
	"github.com/freight-platform/api-gateway/internal/routeauth"
)

type Policy int

const (
	PolicyView Policy = iota
	PolicyCreate
	PolicyAssign
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

func policyAllow(policy Policy) func([]string) bool {
	switch policy {
	case PolicyView:
		return CanViewFleet
	case PolicyCreate:
		return CanCreateFleet
	case PolicyAssign:
		return CanAssignFleet
	default:
		return func([]string) bool { return false }
	}
}

func policyDenyMessage(policy Policy) string {
	switch policy {
	case PolicyView:
		return "driver and vehicle view access denied"
	case PolicyCreate:
		return "driver and vehicle create access denied"
	case PolicyAssign:
		return "shipment fleet assignment access denied"
	default:
		return "fleet access denied"
	}
}
