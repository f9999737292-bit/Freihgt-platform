package http

import (
	"strings"
	"testing"

	sharedrfx "github.com/freight-platform/shared-go/rfx"
)

func TestE7P2INT185ExhaustiveRouteOpenAPIParity(t *testing.T) {
	t.Parallel()
	protected := sharedrfx.E7IntegrationProtectedRoutes()
	if len(protected) != 8 {
		t.Fatalf("expected 8 integration-protected ERP routes (E3-E6), got %d", len(protected))
	}
	seenIDs := map[string]string{}
	seenPaths := map[string]string{}
	rfxOpenAPI, err := readErpPreviewRepoFile(t, "packages/openapi/rfx-service.yaml")
	if err != nil {
		t.Fatalf("read rfx openapi: %v", err)
	}
	unifiedOpenAPI, err := readErpPreviewRepoFile(t, "packages/openapi/openapi.yaml")
	if err != nil {
		t.Fatalf("read unified openapi: %v", err)
	}
	serviceRouter := embeddedErpPreviewServiceRouter
	for _, route := range protected {
		if other, ok := seenIDs[route.OpenAPIOperationID]; ok {
			t.Fatalf("duplicate operationId %s on %s and %s", route.OpenAPIOperationID, other, route.Name)
		}
		seenIDs[route.OpenAPIOperationID] = route.Name
		key := route.Method + " " + route.OpenAPIPath
		if other, ok := seenPaths[key]; ok {
			t.Fatalf("duplicate path %s on %s and %s", key, other, route.Name)
		}
		seenPaths[key] = route.Name
		if !sharedrfx.IsIntegrationProtectedRoute(route.Method, route.GatewayPath) {
			t.Fatalf("%s %s must classify as integration protected", route.Method, route.GatewayPath)
		}
		if sharedrfx.RequiresHumanAuth(route.Method, route.GatewayPath) {
			t.Fatalf("%s %s must not require human JWT", route.Method, route.GatewayPath)
		}
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
			block := extractOpenAPIPathBlock(spec.body, route.OpenAPIPath)
			if block == "" {
				t.Fatalf("%s missing OpenAPI path %s", spec.name, route.OpenAPIPath)
			}
			if !strings.Contains(block, "operationId: "+route.OpenAPIOperationID) {
				t.Fatalf("%s path %s missing operationId %s", spec.name, route.OpenAPIPath, route.OpenAPIOperationID)
			}
		}
	}
	for _, id := range []string{
		"postErpRfxDraftCreatePreview",
		"postErpRfxDraftUpdatePreview",
		"postErpRfxDraftCreateCommit",
		"postErpRfxDraftUpdateCommit",
		"getErpRfxEventById",
		"getErpRfxByExternalId",
		"getErpImportAnalysisStatus",
		"getErpIntegrationCapabilities",
	} {
		if _, ok := seenIDs[id]; !ok {
			t.Fatalf("manifest missing required operationId %s", id)
		}
	}
	if strings.Contains(rfxOpenAPI, "operationId: getErpRfxEventById") && !strings.Contains(serviceRouter, `Get("/rfx-events/{id}"`) {
		t.Fatal("OpenAPI-only GET by ID is forbidden")
	}
}
