package rfxrbac

import (
	"net/http"

	"github.com/freight-platform/api-gateway/internal/config"
	"github.com/freight-platform/api-gateway/internal/routeauth"
)

type Policy int

const (
	PolicyBuyerManage Policy = iota
	PolicyBuyerRead
	PolicyCarrierRespond
	PolicyCarrierRead
	PolicyCombinedRead
	PolicyAcceptBid
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
	case PolicyBuyerManage:
		return CanBuyerManage
	case PolicyBuyerRead:
		return CanBuyerRead
	case PolicyCarrierRespond:
		return CanCarrierRespond
	case PolicyCarrierRead:
		return CanCarrierRead
	case PolicyCombinedRead:
		return CanReadFreightRequests
	case PolicyAcceptBid:
		return CanBuyerManage
	default:
		return func([]string) bool { return false }
	}
}

func policyDenyMessage(policy Policy) string {
	switch policy {
	case PolicyBuyerManage:
		return "rfx buyer manage access denied"
	case PolicyBuyerRead:
		return "rfx buyer read access denied"
	case PolicyCarrierRespond:
		return "rfx carrier respond access denied"
	case PolicyCarrierRead:
		return "rfx carrier read access denied"
	case PolicyCombinedRead:
		return "rfx read access denied"
	case PolicyAcceptBid:
		return "bid accept access denied"
	default:
		return "rfx access denied"
	}
}
