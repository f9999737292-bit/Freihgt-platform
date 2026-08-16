package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
)

func TestRejectClientTenantQueryForbidden(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/v1/bids/x?tenant_id=11111111-1111-1111-1111-111111111111", nil)
	if err := rejectClientTenantQuery(req); err == nil {
		t.Fatal("expected forbidden for tenant_id query")
	}
}

func TestRejectClientTenantQueryAllowsCleanRequest(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/v1/bids/x", nil)
	if err := rejectClientTenantQuery(req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRejectBodyTenantMismatchForbidden(t *testing.T) {
	t.Parallel()
	trusted := "11111111-1111-1111-1111-111111111111"
	if err := rejectBodyTenantMismatch("22222222-2222-2222-2222-222222222222", parseTestUUID(t, trusted)); err == nil {
		t.Fatal("expected forbidden for mismatched body tenant")
	}
}

func TestRejectBodyTenantMismatchAllowsEmpty(t *testing.T) {
	t.Parallel()
	if err := rejectBodyTenantMismatch("", parseTestUUID(t, "11111111-1111-1111-1111-111111111111")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func parseTestUUID(t *testing.T, raw string) uuid.UUID {
	t.Helper()
	id, err := domain.ParseUUID(raw, "tenant_id")
	if err != nil {
		t.Fatalf("parse uuid: %v", err)
	}
	return id
}
