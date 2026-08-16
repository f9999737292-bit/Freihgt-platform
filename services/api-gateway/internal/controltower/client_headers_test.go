package controltower

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestApplyHeadersSetsTenantHeaderWhenQueryPresent(t *testing.T) {
	t.Parallel()
	tenantID := "74519f22-ff9b-4a8b-8fff-a958c689682f"
	endpoint := "http://shipment-service:8085/v1/shipments?tenant_id=" + tenantID + "&limit=200&offset=0"
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &DownstreamClient{}
	client.applyHeaders(req, RequestContext{TenantID: tenantID})
	if got := req.Header.Get("X-Tenant-ID"); got != tenantID {
		t.Fatalf("X-Tenant-ID=%q want %q", got, tenantID)
	}
}

func TestApplyHeadersOmitsTenantHeaderWhenContextEmpty(t *testing.T) {
	t.Parallel()
	req, err := http.NewRequest(http.MethodGet, "http://shipment-service:8085/v1/shipments?tenant_id=74519f22-ff9b-4a8b-8fff-a958c689682f", nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &DownstreamClient{}
	client.applyHeaders(req, RequestContext{})
	if got := req.Header.Get("X-Tenant-ID"); got != "" {
		t.Fatalf("expected no X-Tenant-ID header, got %q", got)
	}
}

func TestApplyHeadersUsesVerifiedContextTenantNotInboundHeader(t *testing.T) {
	t.Parallel()
	verifiedTenant := "11111111-1111-1111-1111-111111111111"
	spoofedTenant := "22222222-2222-2222-2222-222222222222"
	req, err := http.NewRequest(http.MethodGet, "http://shipment-service:8085/v1/shipments?tenant_id="+spoofedTenant, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Tenant-ID", spoofedTenant)
	client := &DownstreamClient{}
	client.applyHeaders(req, RequestContext{TenantID: verifiedTenant, UserID: "user-a"})
	if got := req.Header.Get("X-Tenant-ID"); got != verifiedTenant {
		t.Fatalf("X-Tenant-ID=%q want verified context tenant %q", got, verifiedTenant)
	}
	if got := req.Header.Get("X-User-ID"); got != "user-a" {
		t.Fatalf("X-User-ID=%q want user-a", got)
	}
}

func TestApplyHeadersSetsTenantHeaderForInternalEndpoint(t *testing.T) {
	t.Parallel()
	tenantID := "74519f22-ff9b-4a8b-8fff-a958c689682f"
	req, err := http.NewRequest(http.MethodGet, "http://shipment-service:8085/internal/v1/shipments/status-summary", nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &DownstreamClient{}
	client.applyHeaders(req, RequestContext{TenantID: tenantID})
	if got := req.Header.Get("X-Tenant-ID"); got != tenantID {
		t.Fatalf("X-Tenant-ID=%q want %q", got, tenantID)
	}
}

func TestFetchShipmentsForwardsQueryAndTrustedTenantHeader(t *testing.T) {
	t.Parallel()
	tenantID := "873b3fbc-3cb4-413f-81cd-6fa2c94e785e"
	userID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

	var gotQueryTenant, gotHeaderTenant, gotUser string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/shipments" {
			t.Fatalf("path=%q want /v1/shipments", r.URL.Path)
		}
		gotQueryTenant = r.URL.Query().Get("tenant_id")
		gotHeaderTenant = r.Header.Get("X-Tenant-ID")
		gotUser = r.Header.Get("X-User-ID")
		if gotHeaderTenant == "" {
			http.Error(w, "tenant context is required", http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}, "total": 0})
	}))
	defer server.Close()

	client := NewDownstreamClient(http.DefaultClient, server.URL, server.URL, server.URL, server.URL, server.URL, 200)
	_, _, err := client.FetchShipments(context.Background(), RequestContext{
		TenantID:  tenantID,
		UserID:    userID,
		AuthToken: "Bearer token",
	})
	if err != nil {
		t.Fatalf("FetchShipments: %v", err)
	}
	if gotQueryTenant != tenantID {
		t.Fatalf("query tenant_id=%q want %q", gotQueryTenant, tenantID)
	}
	if gotHeaderTenant != tenantID {
		t.Fatalf("header X-Tenant-ID=%q want %q", gotHeaderTenant, tenantID)
	}
	if gotUser != userID {
		t.Fatalf("header X-User-ID=%q want %q", gotUser, userID)
	}
}
