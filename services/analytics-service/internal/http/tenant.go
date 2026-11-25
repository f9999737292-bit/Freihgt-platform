package http

import "context"

type tenantKey struct{}

type scopeKey struct{}

func withTenant(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantKey{}, tenantID)
}

func tenantFrom(ctx context.Context) string {
	value, _ := ctx.Value(tenantKey{}).(string)
	return value
}

func withScope(ctx context.Context, scope scopeValue) context.Context {
	return context.WithValue(ctx, scopeKey{}, scope)
}

func scopeFrom(ctx context.Context) (scopeValue, bool) {
	value, ok := ctx.Value(scopeKey{}).(scopeValue)
	return value, ok
}
