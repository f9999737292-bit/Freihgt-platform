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
var embeddedExcelExchangeServiceRouter string

func TestE7ExcelExchangeRouteParity(t *testing.T) {
	t.Parallel()
	routes := sharedrfx.E7ExcelExchangeRoutes()
	if len(routes) != 1 {
		t.Fatalf("expected exactly 1 excel exchange route, got %d", len(routes))
	}
	route := routes[0]

	serviceRouter := embeddedExcelExchangeServiceRouter
	rfxOpenAPI, err := readExcelExchangeRepoFile(t, "packages/openapi/rfx-service.yaml")
	if err != nil {
		t.Fatalf("read rfx openapi: %v", err)
	}
	unifiedOpenAPI, err := readExcelExchangeRepoFile(t, "packages/openapi/openapi.yaml")
	if err != nil {
		t.Fatalf("read unified openapi: %v", err)
	}
	gatewayRouter, err := readExcelExchangeRepoFile(t, "services/api-gateway/internal/http/router.go")
	if err != nil {
		t.Fatalf("read gateway router: %v", err)
	}

	if route.Method != "GET" {
		t.Fatalf("manifest method=%q want GET", route.Method)
	}
	if route.OpenAPIPath != "/api/v1/rfx-events/{id}/xlsx-export" {
		t.Fatalf("manifest path=%q", route.OpenAPIPath)
	}
	if route.OpenAPIOperationID != "get_export_buyer_draft_rfx_event_as_xlsx_workbook" {
		t.Fatalf("manifest operationId=%q", route.OpenAPIOperationID)
	}
	if route.RBACPolicy != "PolicyBuyerManage" {
		t.Fatalf("manifest RBAC=%q", route.RBACPolicy)
	}
	if !route.FeatureFlagProtected {
		t.Fatal("manifest must mark feature flag protection")
	}

	if strings.Count(serviceRouter, "xlsx-export") != 1 {
		t.Fatalf("service router must declare exactly 1 xlsx-export route, got %d", strings.Count(serviceRouter, "xlsx-export"))
	}
	if !strings.Contains(serviceRouter, "excelExchangeFlagMiddleware") {
		t.Fatal("service excel exchange routes must be protected by excelExchangeFlagMiddleware")
	}

	serviceNeedle := `.` + excelExchangeChiMethod(route.Method) + `("` + route.ServiceChiPath + `"`
	if !strings.Contains(serviceRouter, serviceNeedle) {
		t.Fatalf("service router missing %s", serviceNeedle)
	}
	gatewayNeedle := `.` + excelExchangeChiMethod(route.Method) + `("` + route.GatewayPath + `"`
	if !strings.Contains(gatewayRouter, gatewayNeedle) {
		t.Fatalf("gateway router missing %s", gatewayNeedle)
	}
	if !strings.Contains(gatewayRouter, "WithPolicy(rfxrbac."+route.RBACPolicy+")") {
		t.Fatalf("gateway router missing RBAC policy %s", route.RBACPolicy)
	}
	if !strings.Contains(gatewayRouter, "excelExchangeFlagMiddleware") {
		t.Fatal("gateway excel exchange routes must be protected by excelExchangeFlagMiddleware")
	}

	for _, spec := range []struct {
		name string
		body string
	}{
		{name: "rfx-service", body: rfxOpenAPI},
		{name: "unified", body: unifiedOpenAPI},
	} {
		t.Run(spec.name, func(t *testing.T) {
			assertExcelExchangeOpenAPIOperation(t, spec.body, route)
		})
	}
}

func assertExcelExchangeOpenAPIOperation(t *testing.T, openAPI string, route sharedrfx.ExcelExchangeRoute) {
	t.Helper()

	pathBlock := extractOpenAPIPathBlock(openAPI, route.OpenAPIPath)
	if pathBlock == "" {
		t.Fatalf("openapi missing path block %s", route.OpenAPIPath)
	}
	if !strings.Contains(pathBlock, "get:") {
		t.Fatalf("openapi path %s must declare GET", route.OpenAPIPath)
	}
	if !strings.Contains(pathBlock, "operationId: "+route.OpenAPIOperationID) {
		t.Fatalf("openapi path %s missing operationId %s", route.OpenAPIPath, route.OpenAPIOperationID)
	}
	if !strings.Contains(pathBlock, "RFX_EXCEL_EXCHANGE_ENABLED") {
		t.Fatal("openapi description must document RFX_EXCEL_EXCHANGE_ENABLED feature flag")
	}
	if !strings.Contains(pathBlock, "BuyerManage") {
		t.Fatal("openapi description must document BuyerManage authorization")
	}
	if !strings.Contains(pathBlock, "type: string") || !strings.Contains(pathBlock, "format: binary") {
		t.Fatal("openapi 200 response must declare string/binary workbook payload")
	}
	if !strings.Contains(pathBlock, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet") {
		t.Fatal("openapi 200 response must declare XLSX content type")
	}
	for _, code := range []string{"'401'", "'403'", "'404'", "'409'", "'500'"} {
		if !strings.Contains(pathBlock, code+":") {
			t.Fatalf("openapi path %s missing %s response", route.OpenAPIPath, code)
		}
	}
}

func extractOpenAPIPathBlock(spec, path string) string {
	lines := strings.Split(spec, "\n")
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == path+":" {
			start = i
			break
		}
	}
	if start < 0 {
		return ""
	}
	var block []string
	baseIndent := leadingSpaces(lines[start])
	for i := start; i < len(lines); i++ {
		line := lines[i]
		if i > start {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" && leadingSpaces(line) <= baseIndent && strings.HasSuffix(trimmed, ":") && !strings.HasPrefix(trimmed, "'") {
				break
			}
		}
		block = append(block, line)
	}
	return strings.Join(block, "\n")
}

func leadingSpaces(line string) int {
	n := 0
	for _, r := range line {
		if r != ' ' {
			break
		}
		n++
	}
	return n
}

func excelExchangeChiMethod(method string) string {
	switch method {
	case "GET":
		return "Get"
	case "POST":
		return "Post"
	case "PUT":
		return "Put"
	case "PATCH":
		return "Patch"
	case "DELETE":
		return "Delete"
	default:
		return method
	}
}

func readExcelExchangeRepoFile(t *testing.T, rel string) (string, error) {
	t.Helper()
	root := excelExchangeModuleRepoRoot(t)
	content, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func excelExchangeModuleRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime caller unavailable")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
}
