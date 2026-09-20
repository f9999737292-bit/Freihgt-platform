package rfxrbac

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCarrierInvitedEventGatewayRouteParity(t *testing.T) {
	t.Parallel()
	routerSource, err := readExcelExchangeGatewayRouter(t)
	if err != nil {
		t.Fatalf("read gateway router: %v", err)
	}
	if !strings.Contains(routerSource, `r.Get("/api/v1/carrier/rfx-events/{id}", rfxGuard.WithPolicy(rfxrbac.PolicyCarrierRead))`) {
		t.Fatal("gateway missing PolicyCarrierRead for GET /api/v1/carrier/rfx-events/{id}")
	}
	if !strings.Contains(routerSource, `r.Get("/api/v1/rfx-events/{id}", rfxGuard.WithPolicy(rfxrbac.PolicyBuyerRead))`) {
		t.Fatal("buyer GET /api/v1/rfx-events/{id} must remain PolicyBuyerRead")
	}

	openAPI, err := readCarrierInvitedOpenAPI(t, "packages/openapi/rfx-service.yaml")
	if err != nil {
		t.Fatalf("read rfx openapi: %v", err)
	}
	if !strings.Contains(openAPI, "/api/v1/carrier/rfx-events/{id}:") {
		t.Fatal("openapi missing carrier invited event path")
	}
	if !strings.Contains(openAPI, "operationId: get_carrier_invited_rfx_event") {
		t.Fatal("openapi missing get_carrier_invited_rfx_event")
	}
}

func readCarrierInvitedOpenAPI(t *testing.T, rel string) (string, error) {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", os.ErrNotExist
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
	content, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return "", err
	}
	return string(content), nil
}
