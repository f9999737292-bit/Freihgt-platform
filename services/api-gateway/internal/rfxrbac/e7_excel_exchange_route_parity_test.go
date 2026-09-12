package rfxrbac

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	sharedrfx "github.com/freight-platform/shared-go/rfx"
)

func TestE7ExcelExchangeGatewayRouteParity(t *testing.T) {
	t.Parallel()
	routes := sharedrfx.E7ExcelExchangeRoutes()
	if len(routes) != 2 {
		t.Fatalf("expected exactly 2 excel exchange routes, got %d", len(routes))
	}

	routerSource, err := readExcelExchangeGatewayRouter(t)
	if err != nil {
		t.Fatalf("read gateway router: %v", err)
	}

	if strings.Count(routerSource, "xlsx-export") != 1 {
		t.Fatalf("gateway router must declare exactly 1 xlsx-export route, got %d", strings.Count(routerSource, "xlsx-export"))
	}
	if strings.Count(routerSource, "xlsx-import/preview") != 1 {
		t.Fatalf("gateway router must declare exactly 1 xlsx-import/preview route, got %d", strings.Count(routerSource, "xlsx-import/preview"))
	}
	if !strings.Contains(routerSource, "excelExchangeFlagMiddleware") {
		t.Fatal("gateway excel exchange routes must be protected by excelExchangeFlagMiddleware")
	}

	for _, route := range routes {
		t.Run(route.Name, func(t *testing.T) {
			if !strings.Contains(routerSource, `.`+excelExchangeChiMethod(route.Method)+`("`+route.GatewayPath+`"`) {
				t.Fatalf("gateway missing %s %s", route.Method, route.GatewayPath)
			}
			if !strings.Contains(routerSource, "WithPolicy(rfxrbac."+route.RBACPolicy+")") &&
				!strings.Contains(routerSource, "WithPolicy("+route.RBACPolicy+")") {
				t.Fatalf("gateway missing RBAC policy %s", route.RBACPolicy)
			}
			switch route.RBACPolicy {
			case "PolicyBuyerManage":
			default:
				t.Fatalf("unknown RBAC policy %s", route.RBACPolicy)
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
	default:
		return method
	}
}

func readExcelExchangeGatewayRouter(t *testing.T) (string, error) {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", os.ErrNotExist
	}
	path := filepath.Join(filepath.Dir(file), "..", "http", "router.go")
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}
