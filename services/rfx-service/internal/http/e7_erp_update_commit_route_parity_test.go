package http

import (
	"strings"
	"testing"

	sharedrfx "github.com/freight-platform/shared-go/rfx"
)

func TestE7ErpUpdateCommitRouteParity(t *testing.T) {
	t.Parallel()
	routes := sharedrfx.E7ErpUpdateCommitRoutes()
	if len(routes) != 1 {
		t.Fatalf("expected exactly 1 ERP UPDATE commit route, got %d", len(routes))
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
		t.Fatal("service ERP commit routes must be protected by erpIntegrationFlagMiddleware")
	}

	for _, route := range routes {
		t.Run(route.Name, func(t *testing.T) {
			serviceNeedle := `.` + erpPreviewChiMethod(route.Method) + `("` + route.ServiceChiPath + `"`
			if !strings.Contains(serviceRouter, serviceNeedle) {
				t.Fatalf("service router missing %s", serviceNeedle)
			}
			if route.SuccessStatus != 200 {
				t.Fatalf("UPDATE commit success status must be 200, got %d", route.SuccessStatus)
			}
			for _, spec := range []struct {
				name string
				body string
			}{
				{name: "rfx-service", body: rfxOpenAPI},
				{name: "unified", body: unifiedOpenAPI},
			} {
				t.Run(spec.name, func(t *testing.T) {
					assertErpUpdateCommitOpenAPIOperation(t, spec.body, route)
				})
			}
		})
	}
}

func assertErpUpdateCommitOpenAPIOperation(t *testing.T, openAPI string, route sharedrfx.ErpMachineAuthRoute) {
	t.Helper()
	pathBlock := extractOpenAPIPathBlock(openAPI, route.OpenAPIPath)
	if pathBlock == "" {
		t.Fatalf("openapi missing path block %s", route.OpenAPIPath)
	}
	if !strings.Contains(pathBlock, "operationId: "+route.OpenAPIOperationID) {
		t.Fatalf("openapi path %s missing operationId %s", route.OpenAPIPath, route.OpenAPIOperationID)
	}
	if strings.Contains(pathBlock, "postErpRfxDraftCreateCommit") ||
		strings.Contains(pathBlock, "postErpRfxDraftCreatePreview") ||
		strings.Contains(pathBlock, "postErpRfxDraftUpdatePreview") {
		t.Fatal("UPDATE commit must not reuse preview or CREATE commit operationIds")
	}
	if !strings.Contains(pathBlock, "RFX_ERP_INTEGRATION_ENABLED") {
		t.Fatal("openapi description must document RFX_ERP_INTEGRATION_ENABLED feature flag")
	}
	if !strings.Contains(pathBlock, "Idempotency-Key") {
		t.Fatal("openapi commit must require Idempotency-Key")
	}
	if !strings.Contains(pathBlock, "ErpUpdateCommitRequest") || !strings.Contains(pathBlock, "ErpUpdateCommitResponse") {
		t.Fatal("openapi commit must declare update commit schemas")
	}
	if !strings.Contains(pathBlock, "'200':") {
		t.Fatal("openapi commit must declare 200 success")
	}
	for _, code := range []string{"'400'", "'401'", "'403'", "'404'", "'409'", "'422'", "'429'", "'500'"} {
		if !strings.Contains(pathBlock, code+":") {
			t.Fatalf("openapi path %s missing %s response", route.OpenAPIPath, code)
		}
	}
	for _, machine := range []string{
		"stale_target",
		"proposal_revalidation_failed",
		"stale_mapping_context",
		"actor_binding_denied",
		"analysis_expired",
		"analysis_already_consumed",
		"canonical_hash_mismatch",
		"idempotency_conflict",
		"external_id_conflict",
	} {
		if !strings.Contains(pathBlock, machine) {
			t.Fatalf("openapi commit must document machine code %s", machine)
		}
	}
}
