package http

import (
	_ "embed"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	sharedrfx "github.com/freight-platform/shared-go/rfx"
)

//go:embed router.go
var embeddedErpPreviewServiceRouter string

func TestE7ErpPreviewRouteParity(t *testing.T) {
	t.Parallel()
	routes := sharedrfx.E7ErpPreviewRoutes()
	if len(routes) != 2 {
		t.Fatalf("expected exactly 2 ERP preview routes, got %d", len(routes))
	}
	serviceRouter := embeddedErpPreviewServiceRouter
	rfxOpenAPI, err := readErpPreviewRepoFile(t, "packages/openapi/rfx-service.yaml")
	if err != nil {
		t.Fatalf("read rfx openapi: %v", err)
	}
	unifiedOpenAPI, err := readErpPreviewRepoFile(t, "packages/openapi/openapi.yaml")
	if err != nil {
		t.Fatalf("read unified openapi: %v", err)
	}
	gatewayRouter, err := readErpPreviewRepoFile(t, "services/api-gateway/internal/http/proxy.go")
	if err != nil {
		t.Fatalf("read gateway proxy: %v", err)
	}

	if !strings.Contains(serviceRouter, "erpIntegrationFlagMiddleware") {
		t.Fatal("service ERP preview routes must be protected by erpIntegrationFlagMiddleware")
	}
	if !strings.Contains(gatewayRouter, "/api/v1/integrations/erp") {
		t.Fatal("gateway must proxy /api/v1/integrations/erp to rfx-service")
	}

	for _, route := range routes {
		t.Run(route.Name, func(t *testing.T) {
			serviceNeedle := `.` + erpPreviewChiMethod(route.Method) + `("` + route.ServiceChiPath + `"`
			if !strings.Contains(serviceRouter, serviceNeedle) {
				t.Fatalf("service router missing %s", serviceNeedle)
			}
			for _, spec := range []struct {
				name string
				body string
			}{
				{name: "rfx-service", body: rfxOpenAPI},
				{name: "unified", body: unifiedOpenAPI},
			} {
				t.Run(spec.name, func(t *testing.T) {
					assertErpPreviewOpenAPIOperation(t, spec.body, route)
				})
			}
		})
	}
}

func assertErpPreviewOpenAPIOperation(t *testing.T, openAPI string, route sharedrfx.ErpMachineAuthRoute) {
	t.Helper()
	pathBlock := extractOpenAPIPathBlock(openAPI, route.OpenAPIPath)
	if pathBlock == "" {
		t.Fatalf("openapi missing path block %s", route.OpenAPIPath)
	}
	if !strings.Contains(pathBlock, strings.ToLower(route.Method)+":") {
		t.Fatalf("openapi path %s must declare %s", route.OpenAPIPath, route.Method)
	}
	if !strings.Contains(pathBlock, "operationId: "+route.OpenAPIOperationID) {
		t.Fatalf("openapi path %s missing operationId %s", route.OpenAPIPath, route.OpenAPIOperationID)
	}
	if !strings.Contains(pathBlock, "RFX_ERP_INTEGRATION_ENABLED") {
		t.Fatal("openapi description must document RFX_ERP_INTEGRATION_ENABLED feature flag")
	}
	if !strings.Contains(pathBlock, "ErpImportPreviewResponse") {
		t.Fatal("openapi preview must declare ErpImportPreviewResponse schema")
	}
	if !strings.Contains(pathBlock, "ErpImportPreviewRequest") {
		t.Fatal("openapi preview must declare ErpImportPreviewRequest schema")
	}
	if !strings.Contains(pathBlock, "'422':") || !strings.Contains(pathBlock, "'413':") {
		t.Fatal("openapi preview must declare 422 and 413 responses")
	}
	for _, code := range []string{"'401'", "'403'", "'404'", "'409'", "'500'"} {
		if !strings.Contains(pathBlock, code+":") {
			t.Fatalf("openapi path %s missing %s response", route.OpenAPIPath, code)
		}
	}
}

func erpPreviewChiMethod(method string) string {
	switch method {
	case "GET":
		return "Get"
	case "POST":
		return "Post"
	default:
		return method
	}
}

func readErpPreviewRepoFile(t *testing.T, rel string) (string, error) {
	t.Helper()
	root := erpPreviewModuleRepoRoot(t)
	content, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func erpPreviewModuleRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime caller unavailable")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
}
