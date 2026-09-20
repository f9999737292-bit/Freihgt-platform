package http

import (
	"strings"
	"testing"

	sharedrfx "github.com/freight-platform/shared-go/rfx"
)

func TestE7ErpReadRouteParity(t *testing.T) {
	t.Parallel()
	routes := sharedrfx.E7ErpReadRoutes()
	if len(routes) != 4 {
		t.Fatalf("expected exactly 4 ERP read routes, got %d", len(routes))
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
	if !strings.Contains(serviceRouter, "erpIntegrationFlagMiddleware") {
		t.Fatal("service ERP read routes must be protected by erpIntegrationFlagMiddleware")
	}

	for _, route := range routes {
		t.Run(route.Name, func(t *testing.T) {
			serviceNeedle := `.` + erpPreviewChiMethod(route.Method) + `("` + route.ServiceChiPath + `"`
			if !strings.Contains(serviceRouter, serviceNeedle) {
				t.Fatalf("service router missing %s", serviceNeedle)
			}
			if route.SuccessStatus != 200 {
				t.Fatalf("read success status must be 200, got %d", route.SuccessStatus)
			}
			for _, spec := range []struct {
				name string
				body string
			}{
				{name: "rfx-service", body: rfxOpenAPI},
				{name: "unified", body: unifiedOpenAPI},
			} {
				t.Run(spec.name, func(t *testing.T) {
					assertErpReadOpenAPIOperation(t, spec.body, route)
				})
			}
		})
	}
}

func assertErpReadOpenAPIOperation(t *testing.T, openAPI string, route sharedrfx.ErpMachineAuthRoute) {
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
	if !strings.Contains(pathBlock, "RFX_ERP_INTEGRATION_ENABLED") {
		t.Fatal("openapi description must document RFX_ERP_INTEGRATION_ENABLED feature flag")
	}
	if !strings.Contains(pathBlock, "'429':") {
		t.Fatal("openapi read must declare 429 for E2 rate limit")
	}
	switch route.OpenAPIOperationID {
	case "getErpRfxEventById", "getErpRfxByExternalId":
		if !strings.Contains(pathBlock, "ErpRfxDraftSummary") {
			t.Fatal("openapi RFx GET must declare ErpRfxDraftSummary")
		}
		if !strings.Contains(pathBlock, "'409':") {
			t.Fatal("openapi RFx GET must declare 409 for published events")
		}
	case "getErpImportAnalysisStatus":
		if !strings.Contains(pathBlock, "ErpAnalysisStatus") {
			t.Fatal("openapi analysis GET must declare ErpAnalysisStatus")
		}
	case "getErpIntegrationCapabilities":
		if !strings.Contains(pathBlock, "ErpIntegrationCapabilities") {
			t.Fatal("openapi capabilities GET must declare ErpIntegrationCapabilities")
		}
	}
	for _, code := range []string{"'401'", "'403'", "'404'", "'500'"} {
		if !strings.Contains(pathBlock, code+":") {
			t.Fatalf("openapi path %s missing %s response", route.OpenAPIPath, code)
		}
	}
}
