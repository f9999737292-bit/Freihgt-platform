package ratesrbac

import "context"

type contextKey struct{}

// VerifiedContext holds gateway-verified company-scoped authorization context.
type VerifiedContext struct {
	TenantID        string
	UserID          string
	AuthToken       string
	CompanyID       string
	ActorKind       string
	CompanyRoles    []string
	IsPlatformAdmin bool
}

func WithVerifiedContext(ctx context.Context, vc VerifiedContext) context.Context {
	return context.WithValue(ctx, contextKey{}, vc)
}

func VerifiedContextFromContext(ctx context.Context) (VerifiedContext, bool) {
	value, ok := ctx.Value(contextKey{}).(VerifiedContext)
	return value, ok
}
