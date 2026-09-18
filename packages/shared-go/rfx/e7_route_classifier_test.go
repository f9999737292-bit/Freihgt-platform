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

func TestProductionClassifierExcludesFixtureRoute(t *testing.T) {
	if IsIntegrationProtectedRoute(http.MethodGet, "/api/v1/integrations/erp/_fixture/protected") {
		t.Fatal("fixture route must not be in production integration classifier")
	}
}

func TestHumanRouteRequiresHumanAuth(t *testing.T) {
	if !RequiresHumanAuth(http.MethodGet, "/api/v1/rfx-events") {
		t.Fatal("expected human route to require human auth")
	}
}

func TestUpdatePreviewConcretePathIsIntegrationProtected(t *testing.T) {
	const eventID = "550e8400-e29b-41d4-a716-446655440000"
	path := "/api/v1/rfx-events/" + eventID + "/erp-import/preview"
	if !IsIntegrationProtectedRoute(http.MethodPost, path) {
		t.Fatal("UPDATE preview with a concrete event id must use integration auth")
	}
	if RequiresHumanAuth(http.MethodPost, path) {
		t.Fatal("UPDATE preview with a concrete event id must not require human auth")
	}
	if !IsIntegrationProtectedRoute(http.MethodPost, "/api/v1/rfx-events/{id}/erp-import/preview") {
		t.Fatal("literal OpenAPI template must still classify as integration protected")
	}
}

func TestUpdatePreviewNeighborsRemainHuman(t *testing.T) {
	const eventID = "550e8400-e29b-41d4-a716-446655440000"
	cases := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/rfx-events/" + eventID + "/erp-import/preview"},
		{http.MethodPost, "/api/v1/rfx-events/" + eventID},
		{http.MethodPost, "/api/v1/rfx-events/" + eventID + "/xlsx-import/preview"},
		{http.MethodPost, "/api/v1/rfx-events/" + eventID + "/erp-import/commit"},
		{http.MethodPost, "/api/v1/rfx-events/" + eventID + "/erp-import/preview/extra"},
		{http.MethodPost, "/api/v1/integrations/erp/rfx/drafts/preview/extra"},
	}
	for _, tc := range cases {
		if IsIntegrationProtectedRoute(tc.method, tc.path) {
			t.Fatalf("%s %s must not be integration protected", tc.method, tc.path)
		}
		if !RequiresHumanAuth(tc.method, tc.path) {
			t.Fatalf("%s %s must remain human authenticated", tc.method, tc.path)
		}
	}
}

func TestCreatePreviewExactPathIsIntegrationProtected(t *testing.T) {
	if !IsIntegrationProtectedRoute(http.MethodPost, "/api/v1/integrations/erp/rfx/drafts/preview") {
		t.Fatal("CREATE preview must be integration protected")
	}
	if RequiresHumanAuth(http.MethodPost, "/api/v1/integrations/erp/rfx/drafts/preview") {
		t.Fatal("CREATE preview must not require human auth")
	}
}
