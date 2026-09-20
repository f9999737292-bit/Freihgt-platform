package rfxrbac

import (
	"os"
	"strings"
	"testing"

	"github.com/freight-platform/shared-go/rfx"
)

func TestE7ErpReadGatewayRouteParity(t *testing.T) {
	proxySource, err := os.ReadFile("../http/proxy.go")
	if err != nil {
		t.Fatalf("read proxy.go: %v", err)
	}
	if !strings.Contains(string(proxySource), "/api/v1/integrations/erp") {
		t.Fatal("gateway must proxy /api/v1/integrations/erp to rfx-service")
	}
	routes := rfx.E7ErpReadRoutes()
	if len(routes) != 4 {
		t.Fatalf("expected 4 ERP read routes, got %d", len(routes))
	}
	for _, route := range routes {
		if !rfx.IsIntegrationProtectedRoute(route.Method, route.GatewayPath) {
			t.Fatalf("route must be integration protected: %s %s", route.Method, route.GatewayPath)
		}
		if rfx.RequiresHumanAuth(route.Method, route.GatewayPath) {
			t.Fatalf("ERP GET must not require human auth: %s", route.GatewayPath)
		}
	}
}

func TestE7P2INT185GatewayExhaustiveParity(t *testing.T) {
	seen := map[string]bool{}
	for _, route := range rfx.E7IntegrationProtectedRoutes() {
		if seen[route.OpenAPIOperationID] {
			t.Fatalf("duplicate operationId %s", route.OpenAPIOperationID)
		}
		seen[route.OpenAPIOperationID] = true
		if !rfx.IsIntegrationProtectedRoute(route.Method, route.GatewayPath) {
			t.Fatalf("%s %s must be integration protected", route.Method, route.GatewayPath)
		}
		if rfx.RequiresHumanAuth(route.Method, route.GatewayPath) {
			t.Fatalf("%s %s must not require human JWT", route.Method, route.GatewayPath)
		}
	}
	if len(seen) != 8 {
		t.Fatalf("expected 8 unique protected operationIds, got %d", len(seen))
	}
}
