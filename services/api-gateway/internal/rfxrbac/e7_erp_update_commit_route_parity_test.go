package rfxrbac

import (
	"os"
	"strings"
	"testing"

	"github.com/freight-platform/shared-go/rfx"
)

func TestE7ErpUpdateCommitGatewayRouteParity(t *testing.T) {
	proxySource, err := os.ReadFile("../http/proxy.go")
	if err != nil {
		t.Fatalf("read proxy.go: %v", err)
	}
	if !strings.Contains(string(proxySource), "/api/v1/rfx-events") {
		t.Fatal("gateway must proxy /api/v1/rfx-events to rfx-service")
	}
	for _, route := range rfx.E7ErpUpdateCommitRoutes() {
		if !rfx.IsIntegrationProtectedRoute(route.Method, route.GatewayPath) {
			t.Fatalf("route must be integration protected: %s %s", route.Method, route.GatewayPath)
		}
		if rfx.RequiresHumanAuth(route.Method, route.GatewayPath) {
			t.Fatalf("UPDATE commit must not require human auth: %s", route.GatewayPath)
		}
	}
}
