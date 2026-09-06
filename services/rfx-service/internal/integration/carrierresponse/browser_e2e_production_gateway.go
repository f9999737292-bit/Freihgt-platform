//go:build integration

package carrierresponse

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func startBrowserProductionGateway(t *testing.T, rfxServiceURL, webProcurementOrigin string, identity *browserIdentityStub) (string, *browserGatewayProcess) {
	t.Helper()
	root, err := repoRoot()
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	openAPIDir := filepath.Join(root, "packages", "openapi")
	env := []string{
		"AUTH_ENABLED=true",
		"JWT_SECRET=" + browserCarrierE2EJWTSecret,
		"RFX_SERVICE_URL=" + strings.TrimRight(rfxServiceURL, "/"),
		"IDENTITY_SERVICE_URL=" + identity.URL(),
		"CORS_ALLOWED_ORIGINS=" + webProcurementOrigin,
		"RATE_LIMIT_ENABLED=false",
		"OPENAPI_DIR=" + openAPIDir,
		"LOG_LEVEL=error",
		"ENVIRONMENT=test",
	}
	gatewayURL, proc := startProductionGatewayProcess(t, env)
	verifyBrowserGatewayHealth(t, gatewayURL)
	verifyBrowserGatewayRfxRoute(t, gatewayURL, rfxServiceURL)
	return gatewayURL, proc
}

func verifyBrowserGatewayHealth(t *testing.T, gatewayURL string) {
	t.Helper()
	resp, err := http.Get(gatewayURL + "/health")
	if err != nil {
		t.Fatalf("gateway health: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("gateway health status=%d", resp.StatusCode)
	}
}

func verifyBrowserGatewayRfxRoute(t *testing.T, gatewayURL, rfxServiceURL string) {
	t.Helper()
	routes := readGatewayRoutes(t, gatewayURL)
	wantTarget := strings.TrimRight(rfxServiceURL, "/") + "/v1/rfx-events"
	found := false
	for _, route := range routes {
		if route["prefix"] == "/api/v1/rfx-events" && route["target"] == wantTarget {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("gateway /routes missing rfx-events target=%q routes=%v", wantTarget, routes)
	}
}

func browserGatewayEnvForStack(t *testing.T, webPort string) string {
	t.Helper()
	return fmt.Sprintf("http://127.0.0.1:%s", webPort)
}
