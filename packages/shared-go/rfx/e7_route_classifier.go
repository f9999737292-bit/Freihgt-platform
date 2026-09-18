package rfx

import (
	"net/http"
	"strings"
)

// RouteClass describes how api-gateway authenticates a request.
type RouteClass int

const (
	RouteClassPublicOAuthToken RouteClass = iota
	RouteClassIntegrationProtected
	RouteClassHumanAuthenticated
)

const (
	publicOAuthTokenMethod = http.MethodPost
	publicOAuthTokenPath   = "/api/v1/integrations/oauth/token"
)

// ClassifyGatewayRoute returns the auth class for a gateway request path.
// Query strings are ignored; only the exact path segment is considered.
func ClassifyGatewayRoute(method, rawPath string) RouteClass {
	method = strings.ToUpper(strings.TrimSpace(method))
	path := NormalizeGatewayPath(rawPath)
	if method == publicOAuthTokenMethod && path == publicOAuthTokenPath {
		return RouteClassPublicOAuthToken
	}
	for _, route := range E7IntegrationProtectedRoutes() {
		if strings.EqualFold(route.Method, method) && route.GatewayPath == path {
			return RouteClassIntegrationProtected
		}
	}
	return RouteClassHumanAuthenticated
}

// IsPublicOAuthTokenRoute reports whether method/path is the exact public OAuth token endpoint.
func IsPublicOAuthTokenRoute(method, rawPath string) bool {
	return ClassifyGatewayRoute(method, rawPath) == RouteClassPublicOAuthToken
}

// IsIntegrationProtectedRoute reports whether method/path requires integration bearer auth.
func IsIntegrationProtectedRoute(method, rawPath string) bool {
	return ClassifyGatewayRoute(method, rawPath) == RouteClassIntegrationProtected
}

// RequiresHumanAuth reports whether the gateway human-auth middleware must validate a user JWT.
func RequiresHumanAuth(method, rawPath string) bool {
	switch ClassifyGatewayRoute(method, rawPath) {
	case RouteClassPublicOAuthToken, RouteClassIntegrationProtected:
		return false
	default:
		return true
	}
}

// NormalizeGatewayPath strips query strings and trailing slashes for stable route matching.
func NormalizeGatewayPath(rawPath string) string {
	path := strings.TrimSpace(rawPath)
	if path == "" {
		return "/"
	}
	if idx := strings.Index(path, "?"); idx >= 0 {
		path = path[:idx]
	}
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}

// E7IntegrationProtectedRoutes returns integration routes that require machine bearer auth.
func E7IntegrationProtectedRoutes() []ErpMachineAuthRoute {
	return []ErpMachineAuthRoute{
		{
			Name:        "integration_auth_fixture",
			Method:      http.MethodGet,
			GatewayPath: "/api/v1/integrations/erp/_fixture/protected",
			Public:      false,
		},
	}
}
