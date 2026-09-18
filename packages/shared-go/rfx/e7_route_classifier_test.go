package rfx

import (
	"net/http"
	"testing"
)

func TestPublicOAuthTokenExactMatch(t *testing.T) {
	if !IsPublicOAuthTokenRoute(http.MethodPost, "/api/v1/integrations/oauth/token") {
		t.Fatal("expected exact oauth token route to be public")
	}
}

func TestPublicOAuthTokenNeighborProtected(t *testing.T) {
	cases := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/integrations/oauth/token"},
		{http.MethodPut, "/api/v1/integrations/oauth/token"},
		{http.MethodPost, "/api/v1/integrations/oauth/token/extra"},
	}
	for _, tc := range cases {
		if IsPublicOAuthTokenRoute(tc.method, tc.path) {
			t.Fatalf("%s %s must not be public oauth token route", tc.method, tc.path)
		}
	}
	if !IsPublicOAuthTokenRoute(http.MethodPost, "/api/v1/integrations/oauth/token?grant_type=client_credentials") {
		t.Fatal("query string must not break oauth token route recognition")
	}
}

func TestIntegrationFixtureRouteProtected(t *testing.T) {
	if !IsIntegrationProtectedRoute(http.MethodGet, "/api/v1/integrations/erp/_fixture/protected") {
		t.Fatal("expected fixture route to require integration auth")
	}
}

func TestHumanRouteRequiresHumanAuth(t *testing.T) {
	if !RequiresHumanAuth(http.MethodGet, "/api/v1/rfx-events") {
		t.Fatal("expected human route to require human auth")
	}
}
