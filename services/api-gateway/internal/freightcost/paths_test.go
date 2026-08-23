package freightcost

import "testing"

func TestMapPublicToInternalPath(t *testing.T) {
	tests := []struct {
		public   string
		internal string
		ok       bool
	}{
		{"/api/v1/freight-costs", "/internal/v1/freight-costs/", true},
		{"/api/v1/freight-costs/summary", "/internal/v1/freight-costs/summary", true},
		{"/api/v1/freight-costs/transport-orders/abc", "/internal/v1/freight-costs/transport-orders/abc", true},
		{"/api/v1/internal/v1/freight-cost", "", false},
		{"/api/v1/billing-registers", "", false},
	}
	for _, tc := range tests {
		got, ok := mapPublicToInternalPath(tc.public)
		if ok != tc.ok || got != tc.internal {
			t.Fatalf("mapPublicToInternalPath(%q) = (%q, %v), want (%q, %v)", tc.public, got, ok, tc.internal, tc.ok)
		}
	}
}

func TestIsAllowlistedPublicPath(t *testing.T) {
	if !isAllowlistedPublicPath("GET", "/api/v1/freight-costs") {
		t.Fatal("list path must be allowlisted")
	}
	if isAllowlistedPublicPath("POST", "/api/v1/freight-costs") {
		t.Fatal("POST must not be allowlisted")
	}
	if !isAllowlistedPublicPath("GET", "/api/v1/freight-costs/transport-orders/id/variance-detail") {
		t.Fatal("variance detail must be allowlisted")
	}
	if !isAllowlistedPublicPath("GET", "/api/v1/freight-costs/analytics/overview") {
		t.Fatal("analytics overview must be allowlisted")
	}
	if !isAllowlistedPublicPath("GET", "/api/v1/freight-costs/opportunities") {
		t.Fatal("opportunities must be allowlisted")
	}
	if isAllowlistedPublicPath("GET", "/internal/v1/freight-cost/analytics/tenants/x/lanes") {
		t.Fatal("internal analytics path must not be public allowlist")
	}
}
