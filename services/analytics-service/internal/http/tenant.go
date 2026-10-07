package http

import "context"

type tenantKey struct{}

func withTenant(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantKey{}, tenantID)
}

func tenantFrom(ctx context.Context) string {
	value, _ := ctx.Value(tenantKey{}).(string)
	return value
}
