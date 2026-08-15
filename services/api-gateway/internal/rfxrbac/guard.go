package rfxrbac

import (
	"net/http"

	"github.com/freight-platform/api-gateway/internal/config"
	"github.com/freight-platform/api-gateway/internal/routeauth"
)

type Policy int

const (
	PolicyEvaluate Policy = iota
	PolicyApproveAward
	PolicyFinalizeAward
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
	case PolicyEvaluate:
		return CanEvaluate
	case PolicyApproveAward:
		return CanApproveAward
	case PolicyFinalizeAward:
		return CanFinalizeAward
	default:
		return func([]string) bool { return false }
	}
}

func policyDenyMessage(policy Policy) string {
	switch policy {
	case PolicyEvaluate:
		return "tender evaluation access denied"
	case PolicyApproveAward:
		return "tender award approval access denied"
	case PolicyFinalizeAward:
		return "tender award finalize access denied"
	default:
		return "tender access denied"
	}
}
