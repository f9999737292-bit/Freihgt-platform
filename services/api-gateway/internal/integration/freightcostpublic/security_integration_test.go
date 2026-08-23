//go:build integration

package freightcostpublic

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestFC_D_SEC_006_CarrierDeniedBuyerAnalyticsEndpoints(t *testing.T) {
	h := newHarness(t, harnessOptions{})
	resp := h.request(h.carrierUserID, h.tenantID, h.carrierID, "GET", "/api/v1/freight-costs/transport-orders/"+h.orderID.String()+"/variance-detail", nil)
	mustStatus(t, "FC-D-SEC-006 variance-detail", resp, 403)

	resp = h.request(h.carrierUserID, h.tenantID, h.carrierID, "GET", "/api/v1/freight-costs/accessorials/summary", nil)
	mustStatus(t, "FC-D-SEC-006 accessorial summary", resp, 403)
}

func TestFC_D_SEC_007_BuyerReceivesAnalyticsFields(t *testing.T) {
	h := newHarness(t, harnessOptions{})
	resp := h.request(h.userID, h.tenantID, h.buyerID, "GET", "/api/v1/freight-costs", nil)
	mustStatus(t, "FC-D-SEC-007 buyer list", resp, 200)
	if !strings.Contains(string(resp.Body), `"accrued_amount"`) {
		t.Fatalf("buyer list must include accrued_amount")
	}
}

func TestFC_D_SEC_008_GatewayRBACMembershipAndRole(t *testing.T) {
	h := newHarness(t, harnessOptions{})
	resp := h.request(h.userID, h.tenantID, uuid.New(), "GET", "/api/v1/freight-costs/summary", nil)
	if resp.Status != 403 {
		t.Fatalf("unknown company membership expected 403 got %d", resp.Status)
	}

	unknownRoleCompany := uuid.New()
	noRoleHarness := newHarness(t, harnessOptions{
		BuyerID: unknownRoleCompany,
		BuyerMemberships: []identityMembership{{
			CompanyID: unknownRoleCompany, CompanyType: "SHIPPER", Roles: []string{"UNKNOWN_ROLE"},
		}},
	})
	resp = noRoleHarness.request(noRoleHarness.userID, noRoleHarness.tenantID, unknownRoleCompany, "GET", "/api/v1/freight-costs/summary", nil)
	if resp.Status != 403 {
		t.Fatalf("unknown role expected 403 got %d", resp.Status)
	}
}

func TestFC_D_SEC_009_CrossTenantDeny(t *testing.T) {
	tenantA := uuid.New()
	tenantB := uuid.New()
	h := newHarness(t, harnessOptions{
		TenantID:         tenantB,
		ResourceTenantID: tenantA,
	})
	resp := h.request(h.userID, tenantB, h.buyerID, "GET", "/api/v1/freight-costs/summary", nil)
	if resp.Status != 404 {
		t.Fatalf("cross-tenant summary expected 404 got %d body=%s", resp.Status, string(resp.Body))
	}
	resp = h.request(h.userID, tenantB, h.buyerID, "GET", "/api/v1/freight-costs/transport-orders/"+h.orderID.String(), nil)
	if resp.Status != 404 {
		t.Fatalf("cross-tenant detail expected 404 got %d", resp.Status)
	}
}

func TestFC_D_SEC_010_CarrierJSONOmitsBuyerOnlyFields(t *testing.T) {
	h := newHarness(t, harnessOptions{})
	resp := h.request(h.carrierUserID, h.tenantID, h.carrierID, "GET", "/api/v1/freight-costs", nil)
	mustStatus(t, "FC-D-SEC-010 carrier list", resp, 200)
	if jsonHasKey(resp.Body, "accrued_amount") || nestedJSONHasKey(resp.Body, "items", "accrued_amount") {
		t.Fatalf("carrier list must not expose accrued_amount: %s", string(resp.Body))
	}
	if strings.Contains(string(resp.Body), `"accrued_amount":null`) {
		t.Fatalf("carrier list must not include null accrued_amount")
	}
}

func TestPublicAuthRequiresJWT(t *testing.T) {
	h := newHarness(t, harnessOptions{})
	resp := requestWithoutAuth(h, h.buyerID, "/api/v1/freight-costs/summary")
	if resp.Status != 401 {
		t.Fatalf("missing JWT expected 401 got %d", resp.Status)
	}
}

func TestPublicSecurityHeaderSpoofingIgnored(t *testing.T) {
	h := newHarness(t, harnessOptions{})
	foreignTenant := uuid.New().String()
	foreignUser := uuid.New().String()
	resp := h.request(h.userID, h.tenantID, h.buyerID, "GET", "/api/v1/freight-costs/summary", map[string]string{
		"X-Tenant-ID":              foreignTenant,
		"X-User-ID":                foreignUser,
		"X-Actor-Kind":             "CARRIER",
		"X-Platform-Admin":         "true",
		"X-Internal-Service-Token": "spoof-token",
	})
	mustStatus(t, "summary with spoof headers", resp, 200)

	capture := h.lastDownstream()
	if capture.TenantID != h.tenantID.String() {
		t.Fatalf("downstream tenant=%q want trusted %q", capture.TenantID, h.tenantID)
	}
	if capture.UserID != h.userID.String() {
		t.Fatalf("downstream user=%q want trusted %q", capture.UserID, h.userID)
	}
	if capture.ActorKind != "BUYER" {
		t.Fatalf("downstream actor kind=%q want BUYER", capture.ActorKind)
	}
	if capture.ServiceToken != internalToken {
		t.Fatalf("downstream must use gateway S2S token")
	}
}

func TestE2E001BuyerSummaryChain(t *testing.T) {
	h := newHarness(t, harnessOptions{})
	resp := h.request(h.userID, h.tenantID, h.buyerID, "GET", "/api/v1/freight-costs/summary", nil)
	mustStatus(t, "E2E-001 summary", resp, 200)
	if !strings.Contains(string(resp.Body), `"planned_total"`) {
		t.Fatalf("summary KPI missing")
	}
	capture := h.lastDownstream()
	if capture.CompanyID != h.buyerID.String() {
		t.Fatalf("trusted company not forwarded")
	}
}

func TestE2E002BuyerLedgerPagination(t *testing.T) {
	h := newHarness(t, harnessOptions{})
	resp := h.request(h.userID, h.tenantID, h.buyerID, "GET", "/api/v1/freight-costs?limit=100&offset=0", nil)
	mustStatus(t, "E2E-002 ledger", resp, 200)
	var payload map[string]any
	if err := json.Unmarshal(resp.Body, &payload); err != nil {
		t.Fatalf("parse list: %v", err)
	}
	if int(payload["limit"].(float64)) != 100 {
		t.Fatalf("expected limit 100 got %v", payload["limit"])
	}
}

func TestE2E003CarrierMaskHTTP(t *testing.T) {
	h := newHarness(t, harnessOptions{})
	resp := h.request(h.carrierUserID, h.tenantID, h.carrierID, "GET", "/api/v1/freight-costs/summary", nil)
	mustStatus(t, "E2E-003 carrier summary", resp, 200)
	if strings.Contains(string(resp.Body), "accrued_total") {
		t.Fatalf("carrier summary leaked buyer KPI")
	}
}

func TestE2E004CrossTenantIsolation(t *testing.T) {
	TestFC_D_SEC_009_CrossTenantDeny(t)
}

func TestE2E005ServiceUnavailable(t *testing.T) {
	down := closedFreightCostServer(t)
	h := newHarness(t, harnessOptions{FreightCostDown: down})
	resp := h.request(h.userID, h.tenantID, h.buyerID, "GET", "/api/v1/freight-costs/summary", nil)
	if resp.Status != 503 && resp.Status != 502 {
		t.Fatalf("service unavailable expected 503/502 got %d body=%s", resp.Status, string(resp.Body))
	}
	mustNotContainSQLLeak(t, resp.Body)
}

func TestErrorSanitization(t *testing.T) {
	h := newHarness(t, harnessOptions{})
	resp := h.request(h.userID, h.tenantID, h.buyerID, "GET", "/api/v1/freight-costs/summary", nil)
	mustStatus(t, "summary", resp, 200)
	mustNotContainSQLLeak(t, resp.Body)
}

func TestLaneAndAccessorialNotAvailableSemantics(t *testing.T) {
	h := newHarness(t, harnessOptions{})
	resp := h.request(h.userID, h.tenantID, h.buyerID, "GET", "/api/v1/freight-costs/lanes/performance", nil)
	mustStatus(t, "lanes", resp, 200)
	if !strings.Contains(string(resp.Body), `"data_capability":"NOT_AVAILABLE"`) {
		t.Fatalf("lanes must declare NOT_AVAILABLE, body=%s", string(resp.Body))
	}
	resp = h.request(h.userID, h.tenantID, h.buyerID, "GET", "/api/v1/freight-costs/accessorials/summary", nil)
	mustStatus(t, "accessorials", resp, 200)
	if !strings.Contains(string(resp.Body), `"data_capability":"NOT_AVAILABLE"`) {
		t.Fatalf("accessorials must declare NOT_AVAILABLE, body=%s", string(resp.Body))
	}
}

var analyticsBuyerRoutes = []string{
	"/api/v1/freight-costs/analytics/overview",
	"/api/v1/freight-costs/analytics/lanes",
	"/api/v1/freight-costs/analytics/carriers",
	"/api/v1/freight-costs/analytics/accessorials",
	"/api/v1/freight-costs/opportunities",
}

func TestFC22F_SEC_003_CarrierDeniedAnalyticsOverview(t *testing.T) {
	h := newHarness(t, harnessOptions{})
	resp := h.request(h.carrierUserID, h.tenantID, h.carrierID, "GET", "/api/v1/freight-costs/analytics/overview", nil)
	mustStatus(t, "FC22F-SEC-003 carrier overview", resp, 403)
}

func TestFC22F_SEC_CarrierDeniedAllBuyerAnalyticsRoutes(t *testing.T) {
	h := newHarness(t, harnessOptions{})
	for _, route := range analyticsBuyerRoutes {
		resp := h.request(h.carrierUserID, h.tenantID, h.carrierID, "GET", route, nil)
		mustStatus(t, "carrier "+route, resp, 403)
	}
}

func TestFC22F_SEC_002_ValidBuyerAnalyticsRoutes(t *testing.T) {
	h := newHarness(t, harnessOptions{})
	for _, route := range analyticsBuyerRoutes {
		resp := h.request(h.userID, h.tenantID, h.buyerID, "GET", route, nil)
		mustStatus(t, "buyer "+route, resp, 200)
	}
	down := h.lastDownstream()
	if down.ActorKind != "BUYER" {
		t.Fatalf("expected BUYER actor kind downstream, got %q", down.ActorKind)
	}
	if down.ServiceToken != internalToken {
		t.Fatalf("expected S2S token forwarded")
	}
}

func TestFC22F_SEC_008_ForeignCompanyMembershipDenied(t *testing.T) {
	h := newHarness(t, harnessOptions{})
	foreignCompany := uuid.New()
	for _, route := range analyticsBuyerRoutes {
		resp := h.request(h.userID, h.tenantID, foreignCompany, "GET", route, nil)
		if resp.Status != 403 {
			t.Fatalf("%s foreign company expected 403 got %d", route, resp.Status)
		}
	}
}

func TestFC22F_SEC_009_CrossTenantSpoofDenied(t *testing.T) {
	tenantA := uuid.New()
	tenantB := uuid.New()
	h := newHarness(t, harnessOptions{
		TenantID:         tenantB,
		ResourceTenantID: tenantA,
	})
	resp := h.request(h.userID, tenantB, h.buyerID, "GET", "/api/v1/freight-costs/analytics/overview", nil)
	if resp.Status != 404 {
		t.Fatalf("cross-tenant analytics expected 404 got %d body=%s", resp.Status, string(resp.Body))
	}
}

func TestFC22F_SEC_012_InternalRouteNotPubliclyExposed(t *testing.T) {
	h := newHarness(t, harnessOptions{})
	resp := h.request(h.userID, h.tenantID, h.buyerID, "GET", "/internal/v1/freight-costs/analytics/overview", nil)
	if resp.Status != 404 {
		t.Fatalf("internal route must not be exposed via gateway, got %d", resp.Status)
	}
}

func TestFC22F_SEC_AnalyticsUnauthenticated(t *testing.T) {
	h := newHarness(t, harnessOptions{})
	for _, route := range analyticsBuyerRoutes {
		resp := requestWithoutAuth(h, h.buyerID, route)
		if resp.Status != 401 {
			t.Fatalf("%s without JWT expected 401 got %d", route, resp.Status)
		}
	}
}

func TestFC_D_SEC_011_CarrierDeniedAnalyticsOverview(t *testing.T) {
	TestFC22F_SEC_003_CarrierDeniedAnalyticsOverview(t)
}

func TestFC_D_SEC_012_CarrierDeniedOpportunities(t *testing.T) {
	h := newHarness(t, harnessOptions{})
	resp := h.request(h.carrierUserID, h.tenantID, h.carrierID, "GET", "/api/v1/freight-costs/opportunities", nil)
	mustStatus(t, "FC-D-SEC-012 carrier opportunities", resp, 403)
}

func TestFC_D_SEC_013_ForeignCompanyDeniedAnalytics(t *testing.T) {
	TestFC22F_SEC_008_ForeignCompanyMembershipDenied(t)
}

func TestFC_D_SEC_014_CrossTenantAnalyticsDenied(t *testing.T) {
	TestFC22F_SEC_009_CrossTenantSpoofDenied(t)
}

func TestFC_D_SEC_015_CarrierAnalyticsBodyOmitsBenchmarkFields(t *testing.T) {
	h := newHarness(t, harnessOptions{})
	for _, route := range analyticsBuyerRoutes {
		resp := h.request(h.carrierUserID, h.tenantID, h.carrierID, "GET", route, nil)
		mustStatus(t, "carrier "+route, resp, 403)
		if strings.Contains(string(resp.Body), `"median"`) ||
			strings.Contains(string(resp.Body), `"p25"`) ||
			strings.Contains(string(resp.Body), `"p75"`) ||
			strings.Contains(string(resp.Body), `"p90"`) ||
			strings.Contains(string(resp.Body), `"estimated_delta"`) ||
			strings.Contains(string(resp.Body), `"benchmark"`) {
			t.Fatalf("carrier must not receive benchmark/opportunity payload on %s: %s", route, string(resp.Body))
		}
	}
}

func TestFC22G_CarrierDeniedInternalAnalyticsRoutes(t *testing.T) {
	h := newHarness(t, harnessOptions{})
	internalRoutes := []string{
		"/internal/v1/freight-cost/analytics/tenants/" + h.tenantID.String() + "/benchmarks",
		"/internal/v1/freight-cost/analytics/tenants/" + h.tenantID.String() + "/opportunities",
		"/internal/v1/freight-cost/analytics/tenants/" + h.tenantID.String() + "/rebuild",
	}
	for _, route := range internalRoutes {
		resp := h.request(h.userID, h.tenantID, h.buyerID, "GET", route, nil)
		if resp.Status != 404 {
			t.Fatalf("internal route %s must not be public, got %d", route, resp.Status)
		}
	}
}
