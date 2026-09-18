package integrationauth

import (
	"net/http"
	"strings"
)

const (
	HeaderIntegrationPrincipalID = "X-Integration-Principal-ID"
	HeaderIntegrationScopes      = "X-Integration-Scopes"
	HeaderAuthScheme             = "X-Integration-Auth-Scheme"
	HeaderActorKind              = "X-Actor-Kind"

	AuthSchemeOAuth  = "OAUTH"
	AuthSchemeAPIKey = "API_KEY"
	ActorKindIntegration = "INTEGRATION"

	APIKeyPrefix = "bt_live_"
)

var untrustedIntegrationHeaders = []string{
	HeaderIntegrationPrincipalID,
	HeaderIntegrationScopes,
	HeaderAuthScheme,
	"X-Integration-Tenant-ID",
	"X-Integration-Company-ID",
	"x-integration-principal-id",
	"x-integration-scopes",
	"x-integration-auth-scheme",
	"x-integration-tenant-id",
	"x-integration-company-id",
}

// StripUntrustedIntegrationHeaders removes client-supplied integration identity headers.
func StripUntrustedIntegrationHeaders(header http.Header) {
	for _, key := range untrustedIntegrationHeaders {
		header.Del(key)
	}
	for name := range header {
		lower := strings.ToLower(name)
		switch lower {
		case strings.ToLower(HeaderIntegrationPrincipalID),
			strings.ToLower(HeaderIntegrationScopes),
			strings.ToLower(HeaderAuthScheme):
			header.Del(name)
		}
	}
}

// InjectTrustedIntegrationHeaders sets verified integration context on the downstream request.
func InjectTrustedIntegrationHeaders(header http.Header, ctx AuthenticatedContext) {
	StripUntrustedIntegrationHeaders(header)
	header.Set(HeaderIntegrationPrincipalID, ctx.PrincipalID.String())
	header.Set("X-Tenant-ID", ctx.TenantID.String())
	header.Set("X-Company-ID", ctx.CompanyID.String())
	header.Set(HeaderIntegrationScopes, strings.Join(ctx.Scopes, " "))
	header.Set(HeaderAuthScheme, ctx.AuthScheme)
	header.Set(HeaderActorKind, ActorKindIntegration)
	header.Del("X-User-ID")
	header.Del("X-User-Email")
	header.Del("X-User-Roles")
}
