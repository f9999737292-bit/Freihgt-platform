package http

import (
	_ "embed"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	sharedrfx "github.com/freight-platform/shared-go/rfx"
)

//go:embed router.go
var embeddedServiceRouter string

func TestE7LateSubmissionFiveRouteParity(t *testing.T) {
	t.Parallel()
	routes := sharedrfx.E7LateSubmissionRoutes()
	if len(routes) != 5 {
		t.Fatalf("expected exactly 5 late submission routes, got %d", len(routes))
	}

	serviceRouter := embeddedServiceRouter
	openAPI, err := readRepoFileFromModuleRoot(t, "packages/openapi/rfx-service.yaml")
	if err != nil {
		t.Fatalf("read openapi: %v", err)
	}
	gatewayRouter, err := readRepoFileFromModuleRoot(t, "services/api-gateway/internal/http/router.go")
	if err != nil {
		t.Fatalf("read gateway router: %v", err)
	}

	lateRouteCount := strings.Count(serviceRouter, "late-submission-requests")
	if lateRouteCount != 5 {
		t.Fatalf("service router must declare exactly 5 late-submission routes, got %d", lateRouteCount)
	}
	if !strings.Contains(serviceRouter, "lateSubmissionFlagMiddleware") {
		t.Fatal("service late submission routes must be protected by lateSubmissionFlagMiddleware")
	}

	for _, route := range routes {
		t.Run(route.Name, func(t *testing.T) {
			serviceNeedle := `.` + chiMethod(route.Method) + `("` + route.ServiceChiPath + `"`
			if !strings.Contains(serviceRouter, serviceNeedle) {
				t.Fatalf("service router missing %s", serviceNeedle)
			}
			gatewayNeedle := `.` + chiMethod(route.Method) + `("` + route.GatewayPath + `"`
			if !strings.Contains(gatewayRouter, gatewayNeedle) {
				t.Fatalf("gateway router missing %s", gatewayNeedle)
			}
			if !strings.Contains(gatewayRouter, "WithPolicy(rfxrbac."+route.RBACPolicy+")") {
				t.Fatalf("gateway router missing RBAC policy %s for %s", route.RBACPolicy, route.Name)
			}
			if !strings.Contains(openAPI, route.OpenAPIPath+":") {
				t.Fatalf("openapi missing path %s", route.OpenAPIPath)
			}
			if !strings.Contains(openAPI, "operationId: "+route.OpenAPIOperationID) {
				t.Fatalf("openapi missing operationId %s", route.OpenAPIOperationID)
			}
			if route.IdempotencyRequired && !strings.Contains(openAPI, "Idempotency-Key") {
				t.Fatalf("openapi must document Idempotency-Key for %s", route.Name)
			}
		})
	}

	extraLateRoutes := regexp.MustCompile(`late-submission-requests`).FindAllString(serviceRouter, -1)
	if len(extraLateRoutes) != 5 {
		t.Fatalf("unexpected late-submission route count in service router: %d", len(extraLateRoutes))
	}
}

func readRepoFileFromModuleRoot(t *testing.T, rel string) (string, error) {
	t.Helper()
	root := moduleRepoRoot(t)
	content, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func chiMethod(method string) string {
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

func moduleRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime caller unavailable")
	}
	// .../services/rfx-service/internal/http/<this file>
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
}
