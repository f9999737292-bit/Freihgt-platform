package controltower

import (
	"net/http"
	"testing"
)

func TestApplyHeadersSkipsTenantHeaderWhenQueryPresent(t *testing.T) {
	t.Parallel()
	endpoint := "http://shipment-service:8085/v1/shipments?tenant_id=74519f22-ff9b-4a8b-8fff-a958c689682f&limit=200&offset=0"
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &DownstreamClient{}
	client.applyHeaders(req, RequestContext{TenantID: "74519f22-ff9b-4a8b-8fff-a958c689682f"})
	if got := req.Header.Get("X-Tenant-ID"); got != "" {
		t.Fatalf("expected no X-Tenant-ID header, got %q", got)
	}
}

func TestApplyHeadersSetsTenantHeaderForInternalEndpoint(t *testing.T) {
	t.Parallel()
	req, err := http.NewRequest(http.MethodGet, "http://shipment-service:8085/internal/v1/shipments/status-summary", nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &DownstreamClient{}
	client.applyHeaders(req, RequestContext{TenantID: "74519f22-ff9b-4a8b-8fff-a958c689682f"})
	if got := req.Header.Get("X-Tenant-ID"); got == "" {
		t.Fatal("expected X-Tenant-ID header for internal endpoint")
	}
}
