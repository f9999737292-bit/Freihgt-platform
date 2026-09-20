package http

import (
	"strings"
	"testing"
)

func TestCarrierInvitedEventRouteParity(t *testing.T) {
	t.Parallel()
	serviceRouter := embeddedServiceRouter
	if embeddedServiceRouter == "" {
		t.Fatal("service router embed is empty")
	}
	rfxOpenAPI, err := readRepoFileFromModuleRoot(t, "packages/openapi/rfx-service.yaml")
	if err != nil {
		t.Fatalf("read rfx openapi: %v", err)
	}
	unifiedOpenAPI, err := readRepoFileFromModuleRoot(t, "packages/openapi/openapi.yaml")
	if err != nil {
		t.Fatalf("read unified openapi: %v", err)
	}
	gatewayRouter, err := readRepoFileFromModuleRoot(t, "services/api-gateway/internal/http/router.go")
	if err != nil {
		t.Fatalf("read gateway router: %v", err)
	}

	if !strings.Contains(serviceRouter, `r.Route("/v1/carrier/rfx-events"`) {
		t.Fatal("service router missing /v1/carrier/rfx-events group")
	}
	if !strings.Contains(serviceRouter, `r.Get("/{id}", rfxHandler.GetCarrierInvitedEvent)`) {
		t.Fatal("service router missing GET /{id} GetCarrierInvitedEvent")
	}
	if !strings.Contains(gatewayRouter, `r.Get("/api/v1/carrier/rfx-events/{id}", rfxGuard.WithPolicy(rfxrbac.PolicyCarrierRead))`) {
		t.Fatal("gateway missing PolicyCarrierRead for GET /api/v1/carrier/rfx-events/{id}")
	}
	if !strings.Contains(gatewayRouter, `r.Get("/api/v1/rfx-events/{id}", rfxGuard.WithPolicy(rfxrbac.PolicyBuyerRead))`) {
		t.Fatal("buyer GET /api/v1/rfx-events/{id} must remain PolicyBuyerRead")
	}

	for _, spec := range []struct {
		name string
		body string
	}{
		{name: "rfx-service", body: rfxOpenAPI},
		{name: "unified", body: unifiedOpenAPI},
	} {
		t.Run(spec.name, func(t *testing.T) {
			if !strings.Contains(spec.body, "/api/v1/carrier/rfx-events/{id}:") {
				t.Fatal("openapi missing GET /api/v1/carrier/rfx-events/{id}")
			}
			if !strings.Contains(spec.body, "operationId: get_carrier_invited_rfx_event") {
				t.Fatal("openapi missing operationId get_carrier_invited_rfx_event")
			}
			if !strings.Contains(spec.body, "CarrierRead") {
				t.Fatal("openapi must document CarrierRead authorization")
			}
			if !strings.Contains(spec.body, "CarrierInvitedEventResponse") {
				t.Fatal("openapi must declare CarrierInvitedEventResponse")
			}
			if !strings.Contains(spec.body, "does not expose participants") {
				t.Fatal("openapi must document that participants and competitor data stay hidden")
			}
		})
	}
}
