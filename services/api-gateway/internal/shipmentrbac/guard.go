package shipmentrbac

import (
	"net/http"

	"github.com/freight-platform/api-gateway/internal/config"
	"github.com/freight-platform/api-gateway/internal/routeauth"
)

type Policy int

const (
	PolicyCreate Policy = iota
	PolicyAccept
	PolicyUpdateStatus
	PolicyCancel
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
	case PolicyCreate:
		return CanCreateShipment
	case PolicyAccept:
		return CanAcceptShipment
	case PolicyUpdateStatus:
		return CanUpdateShipmentStatus
	case PolicyCancel:
		return CanCancelShipment
	default:
		return func([]string) bool { return false }
	}
}

func policyDenyMessage(policy Policy) string {
	switch policy {
	case PolicyCreate:
		return "shipment create access denied"
	case PolicyAccept:
		return "shipment accept access denied"
	case PolicyUpdateStatus:
		return "shipment status update access denied"
	case PolicyCancel:
		return "shipment cancel access denied"
	default:
		return "shipment mutation access denied"
	}
}
