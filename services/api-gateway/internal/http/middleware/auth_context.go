package middleware

import (
	"context"
	"errors"
)

type authContextKey struct{}

// AuthContext holds verified identity data set by auth middleware after JWT validation.
type AuthContext struct {
	UserID              string
	TenantID            string
	Email               string
	AuthToken           string
	RequestedCompanyID  string
}

var ErrAuthContextMissing = errors.New("verified auth context is missing")

func WithAuthContext(ctx context.Context, ac AuthContext) context.Context {
	return context.WithValue(ctx, authContextKey{}, ac)
}

func AuthContextFromContext(ctx context.Context) (AuthContext, bool) {
	value, ok := ctx.Value(authContextKey{}).(AuthContext)
	return value, ok
}

func MustAuthContext(ctx context.Context) (AuthContext, error) {
	ac, ok := AuthContextFromContext(ctx)
	if !ok || ac.TenantID == "" {
		return AuthContext{}, ErrAuthContextMissing
	}
	return ac, nil
}

func StripUntrustedIdentityHeaders(header interface {
	Del(key string)
}) {
	header.Del("X-Tenant-ID")
	header.Del("X-User-ID")
	header.Del("X-User-Email")
	header.Del("X-User-Roles")
	header.Del("X-Company-ID")
	header.Del("X-Actor-Kind")
	header.Del("X-Internal-Service-Token")
}
