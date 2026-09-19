package rfxrbac

import (
	"os"
	"strings"
	"testing"

	"github.com/freight-platform/shared-go/rfx"
)

func TestE7ErpCreateCommitGatewayRouteParity(t *testing.T) {
	proxySource, err := os.ReadFile("../http/proxy.go")
	if err != nil {
		t.Fatalf("read proxy.go: %v", err)
	}
	if !strings.Contains(string(proxySource), "/api/v1/integrations/erp") {
		t.Fatal("gateway must proxy /api/v1/integrations/erp to rfx-service")
	}
	for _, route := range rfx.E7ErpCreateCommitRoutes() {
		if !rfx.IsIntegrationProtectedRoute(route.Method, route.GatewayPath) {
			t.Fatalf("route must be integration protected: %s %s", route.Method, route.GatewayPath)
		}
		if rfx.RequiresHumanAuth(route.Method, route.GatewayPath) {
			t.Fatalf("CREATE commit must not require human auth: %s", route.GatewayPath)
		}
	}
}
