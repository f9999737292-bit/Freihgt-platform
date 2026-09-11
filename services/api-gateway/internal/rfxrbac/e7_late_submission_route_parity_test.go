package rfxrbac

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	sharedrfx "github.com/freight-platform/shared-go/rfx"
)

func TestE7LateSubmissionGatewayRouteParity(t *testing.T) {
	t.Parallel()
	routes := sharedrfx.E7LateSubmissionRoutes()
	if len(routes) != 5 {
		t.Fatalf("expected exactly 5 late submission routes, got %d", len(routes))
	}

	routerSource, err := readGatewayRouter(t)
	if err != nil {
		t.Fatalf("read gateway router: %v", err)
	}

	lateCount := strings.Count(routerSource, "late-submission-requests")
	if lateCount != 5 {
		t.Fatalf("gateway router must declare exactly 5 late-submission routes, got %d", lateCount)
	}

	for _, route := range routes {
		t.Run(route.Name, func(t *testing.T) {
			if !strings.Contains(routerSource, `.`+chiMethod(route.Method)+`("`+route.GatewayPath+`"`) {
				t.Fatalf("gateway missing %s %s", route.Method, route.GatewayPath)
			}
			if !strings.Contains(routerSource, "WithPolicy(rfxrbac."+route.RBACPolicy+")") &&
				!strings.Contains(routerSource, "WithPolicy("+route.RBACPolicy+")") {
				t.Fatalf("gateway missing RBAC policy %s", route.RBACPolicy)
			}
			switch route.RBACPolicy {
			case "PolicyCarrierRespond", "PolicyCarrierRead", "PolicyBuyerRead", "PolicyBuyerManage":
			default:
				t.Fatalf("unknown RBAC policy %s", route.RBACPolicy)
			}
		})
	}
}

func chiMethod(method string) string {
	switch method {
	case "GET":
		return "Get"
	case "POST":
		return "Post"
	default:
		return method
	}
}

func readGatewayRouter(t *testing.T) (string, error) {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", os.ErrNotExist
	}
	// .../services/api-gateway/internal/rfxrbac/<this file>
	path := filepath.Join(filepath.Dir(file), "..", "http", "router.go")
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}
