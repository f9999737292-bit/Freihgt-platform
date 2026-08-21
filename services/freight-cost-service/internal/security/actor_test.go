package security

import (
	"net/http/httptest"
	"testing"
)

func TestParseTrustedActorRejectsPlatformAdmin(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/internal/v1/freight-cost/transport-orders/11111111-1111-1111-1111-111111111111", nil)
	req.Header.Set("X-Tenant-ID", "11111111-1111-1111-1111-111111111111")
	req.Header.Set("X-User-ID", "22222222-2222-2222-2222-222222222222")
	req.Header.Set("X-Company-ID", "33333333-3333-3333-3333-333333333333")
	req.Header.Set("X-Actor-Kind", "PLATFORM_ADMIN")

	_, err := ParseTrustedActor(req)
	if err == nil {
		t.Fatal("expected validation error for PLATFORM_ADMIN")
	}
}
