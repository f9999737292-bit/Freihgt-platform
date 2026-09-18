package rfxrbac

import (
	"os"
	"strings"
	"testing"

	"github.com/freight-platform/shared-go/rfx"
)

func TestE7ErpPreviewGatewayRouteParity(t *testing.T) {
	routerSource, err := os.ReadFile("../http/router.go")
	if err != nil {
		t.Fatalf("read router.go: %v", err)
	}
	proxySource, err := os.ReadFile("../http/proxy.go")
	if err != nil {
		t.Fatalf("read proxy.go: %v", err)
	}
	classifierSource, err := os.ReadFile("../../../../packages/shared-go/rfx/e7_route_classifier.go")
	if err != nil {
		t.Fatalf("read e7_route_classifier.go: %v", err)
	}
	combined := string(routerSource) + string(proxySource) + string(classifierSource)
	for _, route := range rfx.E7ErpPreviewRoutes() {
		switch route.Name {
		case "erp_create_draft_preview":
			if !strings.Contains(combined, "/api/v1/integrations/erp") {
				t.Fatal("gateway must proxy /api/v1/integrations/erp to rfx-service")
			}
		case "erp_update_draft_preview":
			if !strings.Contains(combined, "/api/v1/rfx-events") {
				t.Fatal("gateway must proxy /api/v1/rfx-events to rfx-service")
			}
		default:
			t.Fatalf("unknown route name: %s", route.Name)
		}
		if !rfx.IsIntegrationProtectedRoute(route.Method, route.GatewayPath) {
			t.Fatalf("route must be integration protected: %s %s", route.Method, route.GatewayPath)
		}
		if route.Name == "erp_update_draft_preview" {
			concrete := "/api/v1/rfx-events/550e8400-e29b-41d4-a716-446655440000/erp-import/preview"
			if !rfx.IsIntegrationProtectedRoute(route.Method, concrete) {
				t.Fatalf("concrete UPDATE preview path must be integration protected: %s", concrete)
			}
			if rfx.RequiresHumanAuth(route.Method, concrete) {
				t.Fatal("concrete UPDATE preview path must not require human auth")
			}
		}
	}
}
