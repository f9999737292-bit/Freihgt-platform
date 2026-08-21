package contractrates

import (
	"strings"
	"testing"

	"github.com/freight-platform/api-gateway/internal/ratesrbac"
)

func TestPublicResolveDeniesManualSpot(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"buyer_company_id":"11111111-1111-1111-1111-111111111111","carrier_company_id":"22222222-2222-2222-2222-222222222222","origin_location_id":"33333333-3333-3333-3333-333333333333","destination_location_id":"44444444-4444-4444-4444-444444444444","equipment_type":"BOX","transport_mode":"ROAD","pricing_date":"2026-01-01","manual_spot_amount":"100.00"}`)
	vc := ratesrbac.VerifiedContext{
		CompanyID: "11111111-1111-1111-1111-111111111111",
		ActorKind: "BUYER",
	}
	_, err := validateAndRebuildBody("POST", "/api/v1/rates/resolve", raw, vc)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "not allowed") {
		t.Fatalf("E-PUB-003 expected manual_spot deny, got %v", err)
	}
}
