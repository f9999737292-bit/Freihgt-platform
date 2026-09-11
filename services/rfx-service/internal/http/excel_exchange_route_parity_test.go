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

	serviceRouter := embeddedExcelExchangeServiceRouter
	gatewayRouter, err := readExcelExchangeRepoFile(t, "services/api-gateway/internal/http/router.go")
	if err != nil {
		t.Fatalf("read gateway router: %v", err)
	}

	if strings.Count(serviceRouter, "xlsx-export") != 1 {
		t.Fatalf("service router must declare exactly 1 xlsx-export route, got %d", strings.Count(serviceRouter, "xlsx-export"))
	}
	if !strings.Contains(serviceRouter, "excelExchangeFlagMiddleware") {
		t.Fatal("service excel exchange routes must be protected by excelExchangeFlagMiddleware")
	}

	for _, route := range routes {
		t.Run(route.Name, func(t *testing.T) {
			serviceNeedle := `.` + excelExchangeChiMethod(route.Method) + `("` + route.ServiceChiPath + `"`
			if !strings.Contains(serviceRouter, serviceNeedle) {
				t.Fatalf("service router missing %s", serviceNeedle)
			}
			gatewayNeedle := `.` + excelExchangeChiMethod(route.Method) + `("` + route.GatewayPath + `"`
			if !strings.Contains(gatewayRouter, gatewayNeedle) {
				t.Fatalf("gateway router missing %s", gatewayNeedle)
			}
			if !strings.Contains(gatewayRouter, "WithPolicy(rfxrbac."+route.RBACPolicy+")") {
				t.Fatalf("gateway router missing RBAC policy %s for %s", route.RBACPolicy, route.Name)
			}
			if !strings.Contains(gatewayRouter, "excelExchangeFlagMiddleware") {
				t.Fatal("gateway excel exchange routes must be protected by excelExchangeFlagMiddleware")
			}
		})
	}
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
