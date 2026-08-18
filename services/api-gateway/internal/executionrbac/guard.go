package executionrbac

import (
	"net/http"

	"github.com/freight-platform/api-gateway/internal/config"
	"github.com/freight-platform/api-gateway/internal/routeauth"
)

type Policy int

const (
	PolicyExecute Policy = iota
	PolicyRead
	PolicyStart
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
	case PolicyExecute:
		return CanExecuteOrder
	case PolicyRead:
		return CanReadExecution
	case PolicyStart:
		return CanStartExecution
	default:
		return func([]string) bool { return false }
	}
}

func policyDenyMessage(policy Policy) string {
	switch policy {
	case PolicyExecute:
		return "transport order execution access denied"
	case PolicyRead:
		return "transport order execution read access denied"
	case PolicyStart:
		return "transport order start execution access denied"
	default:
		return "order execution access denied"
	}
}
