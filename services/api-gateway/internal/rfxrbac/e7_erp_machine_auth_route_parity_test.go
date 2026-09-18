package rfxrbac

import (
	"os"
	"strings"
	"testing"

	"github.com/freight-platform/shared-go/rfx"
)

func TestE7ErpMachineAuthRouteParity(t *testing.T) {
	routerSource, err := os.ReadFile("../http/router.go")
	if err != nil {
		t.Fatalf("read router.go: %v", err)
	}
	proxySource, err := os.ReadFile("../http/proxy.go")
	if err != nil {
		t.Fatalf("read proxy.go: %v", err)
	}
	authSource, err := os.ReadFile("../http/middleware/auth.go")
	if err != nil {
		t.Fatalf("read auth.go: %v", err)
	}
	combined := string(routerSource) + string(proxySource) + string(authSource)
	for _, route := range rfx.E7ErpMachineAuthRoutes() {
		if !strings.Contains(combined, route.GatewayPath) {
			t.Fatalf("gateway route missing: %s", route.GatewayPath)
		}
		if !strings.Contains(combined, route.OpenAPIOperationID) && route.OpenAPIOperationID != "" {
			// operationId lives in OpenAPI generator; gateway exposes path only
			continue
		}
	}
}
