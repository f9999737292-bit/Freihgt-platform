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
	if len(routes) != 2 {
		t.Fatalf("expected exactly 2 excel exchange routes, got %d", len(routes))
	}
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

	if !strings.Contains(serviceRouter, "excelExchangeFlagMiddleware") {
		t.Fatal("service excel exchange routes must be protected by excelExchangeFlagMiddleware")
	}
	if !strings.Contains(gatewayRouter, "excelExchangeFlagMiddleware") {
		t.Fatal("gateway excel exchange routes must be protected by excelExchangeFlagMiddleware")
	}

	for _, route := range routes {
		t.Run(route.Name, func(t *testing.T) {
			if route.RBACPolicy != "PolicyBuyerManage" {
				t.Fatalf("manifest RBAC=%q", route.RBACPolicy)
			}
			if !route.FeatureFlagProtected {
				t.Fatal("manifest must mark feature flag protection")
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
		})
	}
}

func assertExcelExchangeOpenAPIOperation(t *testing.T, openAPI string, route sharedrfx.ExcelExchangeRoute) {
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
	if !strings.Contains(pathBlock, "RFX_EXCEL_EXCHANGE_ENABLED") {
		t.Fatal("openapi description must document RFX_EXCEL_EXCHANGE_ENABLED feature flag")
	}
	if !strings.Contains(pathBlock, "BuyerManage") {
		t.Fatal("openapi description must document BuyerManage authorization")
	}

	switch route.Name {
	case "export_buyer_draft_xlsx":
		if !strings.Contains(pathBlock, "type: string") || !strings.Contains(pathBlock, "format: binary") {
			t.Fatal("openapi 200 response must declare string/binary workbook payload")
		}
		if !strings.Contains(pathBlock, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet") {
			t.Fatal("openapi 200 response must declare XLSX content type")
		}
	case "preview_buyer_draft_xlsx_import":
		if !strings.Contains(pathBlock, "multipart/form-data") {
			t.Fatal("openapi preview request must declare multipart/form-data")
		}
		if !strings.Contains(pathBlock, "RfxBuyerXlsxImportPreviewResponse") {
			t.Fatal("openapi preview must declare structured preview response schema")
		}
		if !strings.Contains(pathBlock, "'422':") {
			t.Fatal("openapi preview must declare 422 structured preview response")
		}
		if !strings.Contains(pathBlock, "'413':") {
			t.Fatal("openapi preview must declare 413 response")
		}
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
