package handlers

import (
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
)

func TestResolveVerifiedTenantFromHeader(t *testing.T) {
	t.Parallel()
	headerTenant := "11111111-1111-1111-1111-111111111111"
	req := httptest.NewRequest("GET", "/v1/shipments/x", nil)
	req.Header.Set("X-Tenant-ID", headerTenant)

	got, err := resolveVerifiedTenant(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.String() != headerTenant {
		t.Fatalf("tenant=%s want header tenant", got)
	}
}

func TestResolveVerifiedTenantIgnoresQueryWhenHeaderPresent(t *testing.T) {
	t.Parallel()
	headerTenant := "11111111-1111-1111-1111-111111111111"
	queryTenant := "22222222-2222-2222-2222-222222222222"
	req := httptest.NewRequest("GET", "/v1/shipments/x?tenant_id="+queryTenant, nil)
	req.Header.Set("X-Tenant-ID", headerTenant)

	got, err := resolveVerifiedTenant(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.String() != headerTenant {
		t.Fatalf("tenant=%s want header tenant", got)
	}
}

func TestResolveVerifiedTenantQueryOnlyReturnsUnauthorized(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest("GET", "/v1/shipments/x?tenant_id=11111111-1111-1111-1111-111111111111", nil)
	_, err := resolveVerifiedTenant(req)
	if err == nil {
		t.Fatal("expected error for query-only tenant")
	}
	var appErr *apperrors.AppError
	if !errorsAsAppError(err, &appErr) || appErr.Code != apperrors.CodeUnauthorized {
		t.Fatalf("expected unauthorized, got %v", err)
	}
}

func TestResolveVerifiedTenantMissingReturnsUnauthorized(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest("GET", "/v1/shipments/x", nil)
	_, err := resolveVerifiedTenant(req)
	if err == nil {
		t.Fatal("expected error for missing tenant")
	}
}

func TestResolveVerifiedTenantRejectsZeroUUID(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest("GET", "/v1/shipments/x", nil)
	req.Header.Set("X-Tenant-ID", uuid.Nil.String())
	_, err := resolveVerifiedTenant(req)
	if err == nil {
		t.Fatal("expected validation error for zero tenant")
	}
}

func TestResolveVerifiedTenantRejectsInvalidUUID(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest("GET", "/v1/shipments/x", nil)
	req.Header.Set("X-Tenant-ID", "not-a-uuid")
	_, err := resolveVerifiedTenant(req)
	if err == nil {
		t.Fatal("expected validation error for invalid tenant")
	}
}

func errorsAsAppError(err error, target **apperrors.AppError) bool {
	if err == nil {
		return false
	}
	appErr, ok := err.(*apperrors.AppError)
	if !ok {
		return false
	}
	*target = appErr
	return true
}
