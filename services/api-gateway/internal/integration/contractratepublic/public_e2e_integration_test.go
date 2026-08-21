//go:build integration

package contractratepublic

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"

	crtestkit "github.com/freight-platform/contract-rate-service/testkit"
)

func TestPublicE2E001BuyerContractLifecycle(t *testing.T) {
	h := newHarness(t, harnessOptions{})
	user := h.userID

	createBody := map[string]any{
		"buyer_company_id":    h.buyerID.String(),
		"carrier_company_id":  h.carrierID.String(),
		"contract_number":     "PUB-E2E-001",
		"name":                "Public E2E Contract",
		"valid_from":          "2026-01-01",
		"currency_code":       "RUB",
	}
	resp := h.request(user, h.buyerID, "POST", "/api/v1/transport-contracts", createBody, nil)
	mustStatus(t, "E-E2E-001 create", resp, 201)
	contractID := parseJSONField(t, resp.Body, "id")

	resp = h.request(user, h.buyerID, "GET", "/api/v1/transport-contracts/"+contractID, nil, nil)
	mustStatus(t, "E-E2E-001 get", resp, 200)
	if !strings.Contains(string(resp.Body), `"status":"DRAFT"`) {
		t.Fatalf("expected DRAFT contract")
	}

	resp = h.request(user, h.buyerID, "PATCH", "/api/v1/transport-contracts/"+contractID, map[string]any{
		"description": "draft edit",
	}, nil)
	mustStatus(t, "E-E2E-001 patch draft", resp, 200)

	resp = h.request(user, h.buyerID, "POST", "/api/v1/transport-contracts/"+contractID+"/activate", map[string]any{}, nil)
	mustStatus(t, "E-E2E-001 activate", resp, 200)
	if !strings.Contains(string(resp.Body), `"status":"ACTIVE"`) {
		t.Fatalf("expected ACTIVE")
	}

	resp = h.request(user, h.buyerID, "PATCH", "/api/v1/transport-contracts/"+contractID, map[string]any{
		"description":        "active metadata",
		"external_reference": "EXT-001",
	}, nil)
	mustStatus(t, "E-E2E-001 patch active metadata", resp, 200)
	if !strings.Contains(string(resp.Body), `"external_reference":"EXT-001"`) {
		t.Fatalf("metadata patch failed")
	}

	resp = h.request(user, h.buyerID, "PATCH", "/api/v1/transport-contracts/"+contractID, map[string]any{
		"valid_to": "2027-12-31",
	}, nil)
	if resp.Status == 200 {
		t.Fatal("ACTIVE contract valid_to patch must be denied")
	}

	resp = h.request(user, h.buyerID, "POST", "/api/v1/transport-contracts/"+contractID+"/suspend", map[string]any{}, nil)
	mustStatus(t, "E-E2E-001 suspend", resp, 200)

	resp = h.request(user, h.buyerID, "POST", "/api/v1/transport-contracts/"+contractID+"/reactivate", map[string]any{}, nil)
	mustStatus(t, "E-E2E-001 reactivate", resp, 200)
}

func TestPublicE2E002CompleteRateFlow(t *testing.T) {
	h := newHarness(t, harnessOptions{})
	user := h.userID
	ctx := context.Background()

	contractID := createAndActivateContract(t, h, user, h.buyerID, "PUB-E2E-002")

	resp := h.request(user, h.buyerID, "POST", "/api/v1/transport-contracts/"+contractID+"/rate-cards", map[string]any{
		"name": "Main Card",
	}, nil)
	mustStatus(t, "create rate card", resp, 201)
	rateCardID := parseJSONField(t, resp.Body, "id")

	resp = h.request(user, h.buyerID, "POST", "/api/v1/rate-cards/"+rateCardID+"/versions", map[string]any{
		"valid_from": "2026-01-01",
	}, nil)
	mustStatus(t, "create version", resp, 201)
	versionID := parseJSONField(t, resp.Body, "id")

	resp = h.request(user, h.buyerID, "POST", "/api/v1/rate-card-versions/"+versionID+"/rate-lines", map[string]any{
		"origin_location_id":      h.originID.String(),
		"destination_location_id": h.destID.String(),
		"equipment_type":          "Box",
		"transport_mode":          "ROAD",
	}, nil)
	mustStatus(t, "create lane", resp, 201)
	lineID := parseJSONField(t, resp.Body, "id")
	if !strings.Contains(string(resp.Body), `"equipment_type":"Box"`) {
		t.Fatalf("equipment must remain case-sensitive Box")
	}

	for _, comp := range []map[string]any{
		{"component_type": "BASE_FREIGHT", "calculation_method": "FLAT", "amount": "100000.00"},
		{"component_type": "FUEL_SURCHARGE", "calculation_method": "PERCENT", "percent_value": "8.00"},
		{"component_type": "WAITING", "calculation_method": "UNIT_RATE", "amount": "500.00", "unit_code": "HOUR"},
		{"component_type": "DETENTION", "calculation_method": "UNIT_RATE", "amount": "700.00", "unit_code": "HOUR"},
	} {
		resp = h.request(user, h.buyerID, "POST", "/api/v1/rate-lines/"+lineID+"/components", comp, nil)
		mustStatus(t, "create component "+comp["component_type"].(string), resp, 201)
	}

	resp = h.request(user, h.buyerID, "PATCH", "/api/v1/rate-lines/"+lineID, map[string]any{
		"transport_mode": "ROAD",
	}, nil)
	mustStatus(t, "patch lane", resp, 200)

	resp = h.request(user, h.buyerID, "GET", "/api/v1/rate-lines/"+lineID+"/components", nil, nil)
	mustStatus(t, "list components", resp, 200)
	componentID := firstItemID(t, resp.Body)
	resp = h.request(user, h.buyerID, "PATCH", "/api/v1/rate-components/"+componentID, map[string]any{
		"amount": "100000.00",
	}, nil)
	mustStatus(t, "patch component", resp, 200)

	resp = h.request(user, h.buyerID, "GET", "/api/v1/rate-lines/"+lineID+"/components", nil, nil)
	mustStatus(t, "verify components", resp, 200)
	if !strings.Contains(string(resp.Body), `"BASE_FREIGHT"`) || !strings.Contains(string(resp.Body), `"DETENTION"`) {
		t.Fatalf("components missing")
	}

	resp = h.request(user, h.buyerID, "POST", "/api/v1/rate-card-versions/"+versionID+"/activate", map[string]any{}, nil)
	mustStatus(t, "activate version", resp, 200)
	if !strings.Contains(string(resp.Body), `"status":"ACTIVE"`) {
		t.Fatalf("expected ACTIVE version")
	}
	if !strings.Contains(string(resp.Body), `"created_at"`) || !strings.Contains(string(resp.Body), `"activated_at"`) {
		t.Fatalf("audit metadata missing")
	}
	if strings.Contains(string(resp.Body), `"activated_at":null`) {
		t.Fatalf("activated_at must be set after activation")
	}

	resp = h.request(user, h.buyerID, "GET", "/api/v1/rate-card-versions/"+versionID, nil, nil)
	mustStatus(t, "get version", resp, 200)

	// equipment case sensitivity: BOX must not match Box lane
	resolveNoMatch := map[string]any{
		"buyer_company_id":        h.buyerID.String(),
		"carrier_company_id":      h.carrierID.String(),
		"origin_location_id":      h.originID.String(),
		"destination_location_id": h.destID.String(),
		"equipment_type":          "BOX",
		"transport_mode":          "ROAD",
		"pricing_date":            "2026-08-20",
		"currency_code":           "RUB",
	}
	resp = h.request(user, h.buyerID, "POST", "/api/v1/rates/resolve", resolveNoMatch, nil)
	mustStatus(t, "resolve BOX no match", resp, 200)
	if strings.Contains(string(resp.Body), `"status":"MATCHED"`) && strings.Contains(string(resp.Body), `"equipment_type":"BOX"`) {
		t.Fatalf("BOX must not match Box lane")
	}

	// DB persistence assertions
	if n := crtestkit.CountRows(ctx, h.pool, `SELECT COUNT(*) FROM contract_rate.transport_contract WHERE tenant_id=$1 AND id=$2`, h.tenantID, contractID); n != 1 {
		t.Fatalf("contract row missing")
	}
	if n := crtestkit.CountRows(ctx, h.pool, `SELECT COUNT(*) FROM contract_rate.rate_card WHERE tenant_id=$1 AND id=$2`, h.tenantID, rateCardID); n != 1 {
		t.Fatalf("rate card row missing")
	}
	if n := crtestkit.CountRows(ctx, h.pool, `SELECT COUNT(*) FROM contract_rate.rate_card_version WHERE tenant_id=$1 AND id=$2 AND status='ACTIVE'`, h.tenantID, versionID); n != 1 {
		t.Fatalf("active version row missing")
	}
	if n := crtestkit.CountRows(ctx, h.pool, `SELECT COUNT(*) FROM contract_rate.rate_line WHERE tenant_id=$1 AND id=$2`, h.tenantID, lineID); n != 1 {
		t.Fatalf("rate line row missing")
	}
	if n := crtestkit.CountRows(ctx, h.pool, `SELECT COUNT(*) FROM contract_rate.rate_component rc JOIN contract_rate.rate_line rl ON rl.id=rc.rate_line_id WHERE rl.tenant_id=$1 AND rl.id=$2`, h.tenantID, lineID); n != 4 {
		t.Fatalf("expected 4 components, got %d", n)
	}
}

func TestPublicE2E003PublicSimulation(t *testing.T) {
	h := newHarness(t, harnessOptions{})
	user := h.userID
	contractID := createAndActivateContract(t, h, user, h.buyerID, "PUB-E2E-003")
	rateCardID, versionID, lineID := createActiveRateStack(t, h, user, h.buyerID, contractID, "Box", "100000.00", "8.00")

	resolveBody := map[string]any{
		"buyer_company_id":        h.buyerID.String(),
		"carrier_company_id":      h.carrierID.String(),
		"origin_location_id":      h.originID.String(),
		"destination_location_id": h.destID.String(),
		"equipment_type":          "Box",
		"transport_mode":          "ROAD",
		"pricing_date":            "2026-08-20",
		"currency_code":           "RUB",
	}
	resp := h.request(user, h.buyerID, "POST", "/api/v1/rates/resolve", resolveBody, nil)
	mustStatus(t, "resolve matched", resp, 200)

	var result map[string]any
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		t.Fatalf("decode resolve: %v", err)
	}
	if result["status"] != "MATCHED" {
		t.Fatalf("expected MATCHED got %v", result["status"])
	}
	if result["pricing_source"] != "CONTRACT_RATE" {
		t.Fatalf("expected CONTRACT_RATE pricing source")
	}
	if fmt.Sprint(result["contract_id"]) != contractID {
		t.Fatalf("contract id mismatch")
	}
	if fmt.Sprint(result["rate_card_id"]) != rateCardID {
		t.Fatalf("rate card id mismatch")
	}
	if fmt.Sprint(result["rate_version_id"]) != versionID {
		t.Fatalf("rate version id mismatch")
	}
	if fmt.Sprint(result["rate_line_id"]) != lineID {
		t.Fatalf("rate line id mismatch")
	}
	if result["base_amount"] != "100000.00" {
		t.Fatalf("base_amount want 100000.00 got %v", result["base_amount"])
	}
	if result["total_amount"] != "108000.00" {
		t.Fatalf("total_amount want 108000.00 got %v", result["total_amount"])
	}

	forbidden := []struct {
		field string
		body  map[string]any
	}{
		{"manual_spot_amount", withField(resolveBody, "manual_spot_amount", "1.00")},
		{"pricing_source", withField(resolveBody, "pricing_source", "MANUAL_SPOT")},
		{"award_link_id", withField(resolveBody, "award_link_id", uuid.New().String())},
		{"award_scope_event_id", withField(resolveBody, "award_scope_event_id", uuid.New().String())},
		{"bid_id", withField(resolveBody, "bid_id", uuid.New().String())},
	}
	for _, tc := range forbidden {
		resp = h.request(user, h.buyerID, "POST", "/api/v1/rates/resolve", tc.body, nil)
		if resp.Status != 400 {
			t.Fatalf("forbidden %s expected 400 got %d body=%s", tc.field, resp.Status, string(resp.Body))
		}
	}
}

func TestPublicE2E004CarrierReadOnly(t *testing.T) {
	buyerUser := uuid.New()
	carrierUser := uuid.New()
	tenantID := uuid.New()
	buyerID := uuid.New()
	carrierID := uuid.New()
	originID := uuid.New()
	destID := uuid.New()

	h := newHarness(t, harnessOptions{
		TenantID: tenantID,
		UserID:   buyerUser,
		BuyerID:  buyerID,
		CarrierID: carrierID,
		OriginID: originID,
		DestID:   destID,
		MembershipsByUser: map[uuid.UUID][]identityMembership{
			buyerUser:   {{CompanyID: buyerID, CompanyType: "SHIPPER", Roles: []string{"PROCUREMENT_MANAGER"}}},
			carrierUser: {{CompanyID: carrierID, CompanyType: "CARRIER", Roles: []string{"CARRIER_ADMIN"}}},
		},
	})
	contractID := createAndActivateContract(t, h, buyerUser, buyerID, "PUB-E2E-004")
	rateCardID, versionID, lineID := createActiveRateStack(t, h, buyerUser, buyerID, contractID, "Box", "100000.00", "8.00")

	readPaths := []string{
		"/api/v1/transport-contracts/" + contractID,
		"/api/v1/transport-contracts/" + contractID + "/rate-cards",
		"/api/v1/rate-cards/" + rateCardID + "/versions",
		"/api/v1/rate-card-versions/" + versionID + "/rate-lines",
		"/api/v1/rate-lines/" + lineID + "/components",
	}
	for _, path := range readPaths {
		resp := h.request(carrierUser, carrierID, "GET", path, nil, nil)
		mustStatus(t, "carrier read "+path, resp, 200)
	}

	resolveBody := map[string]any{
		"buyer_company_id": h.buyerID.String(), "carrier_company_id": h.carrierID.String(),
		"origin_location_id": h.originID.String(), "destination_location_id": h.destID.String(),
		"equipment_type": "Box", "transport_mode": "ROAD", "pricing_date": "2026-08-20",
	}
	resp := h.request(carrierUser, carrierID, "POST", "/api/v1/rates/resolve", resolveBody, nil)
	mustStatus(t, "carrier simulate", resp, 200)

	listResp := h.request(carrierUser, carrierID, "GET", "/api/v1/rate-lines/"+lineID+"/components", nil, nil)
	mustStatus(t, "list components for delete deny", listResp, 200)
	componentID := firstItemID(t, listResp.Body)

	mutations := []struct {
		method string
		path   string
		body   any
	}{
		{"PATCH", "/api/v1/transport-contracts/" + contractID, map[string]any{"description": "x"}},
		{"POST", "/api/v1/transport-contracts/" + contractID + "/suspend", map[string]any{}},
		{"POST", "/api/v1/transport-contracts/" + contractID + "/rate-cards", map[string]any{"name": "x"}},
		{"POST", "/api/v1/rate-cards/" + rateCardID + "/versions", map[string]any{"valid_from": "2026-01-01"}},
		{"PATCH", "/api/v1/rate-lines/" + lineID, map[string]any{"transport_mode": "ROAD"}},
		{"DELETE", "/api/v1/rate-components/" + componentID, nil},
	}
	for _, m := range mutations {
		resp = h.request(carrierUser, carrierID, m.method, m.path, m.body, nil)
		if resp.Status != 403 {
			t.Fatalf("carrier mutate %s %s expected 403 got %d body=%s", m.method, m.path, resp.Status, string(resp.Body))
		}
	}
}

func TestPublicE2E005TerminalHistoricalRead(t *testing.T) {
	h := newHarness(t, harnessOptions{})
	user := h.userID
	contractID := createAndActivateContract(t, h, user, h.buyerID, "PUB-E2E-005")
	rateCardID, versionID, lineID := createActiveRateStack(t, h, user, h.buyerID, contractID, "Box", "50000.00", "0.00")

	resp := h.request(user, h.buyerID, "POST", "/api/v1/transport-contracts/"+contractID+"/terminate", map[string]any{
		"termination_reason": "E2E terminate",
	}, nil)
	mustStatus(t, "terminate", resp, 200)

	for _, path := range []string{
		"/api/v1/transport-contracts/" + contractID,
		"/api/v1/transport-contracts/" + contractID + "/rate-cards",
		"/api/v1/rate-cards/" + rateCardID + "/versions",
		"/api/v1/rate-card-versions/" + versionID,
		"/api/v1/rate-card-versions/" + versionID + "/rate-lines",
		"/api/v1/rate-lines/" + lineID + "/components",
	} {
		resp = h.request(user, h.buyerID, "GET", path, nil, nil)
		mustStatus(t, "historical read "+path, resp, 200)
	}

	// activated_at must remain truthful on historical version
	if !strings.Contains(string(resp.Body), `"activated_at"`) {
		t.Fatalf("historical version must expose activated_at")
	}

	resolveBody := map[string]any{
		"buyer_company_id": h.buyerID.String(), "carrier_company_id": h.carrierID.String(),
		"origin_location_id": h.originID.String(), "destination_location_id": h.destID.String(),
		"equipment_type": "Box", "transport_mode": "ROAD", "pricing_date": "2026-08-20",
	}
	resp = h.request(user, h.buyerID, "POST", "/api/v1/rates/resolve", resolveBody, nil)
	mustStatus(t, "post-terminate resolve", resp, 200)
	if strings.Contains(string(resp.Body), `"status":"MATCHED"`) {
		t.Fatalf("terminated contract must not repricing-match")
	}
}

func TestPublicE2E006CrossCompanyRoleBleed(t *testing.T) {
	pool := crtestkit.SetupPool(t)
	ctx := context.Background()
	tenantID := uuid.New()
	userID := uuid.New()
	managerID := uuid.New()
	companyA := uuid.New()
	companyB := uuid.New()
	carrierID := uuid.New()
	originID := uuid.New()
	destID := uuid.New()

	crtestkit.SeedTenantAndCompanies(t, ctx, pool, tenantID, companyA, carrierID)
	crtestkit.SeedCompany(t, ctx, pool, tenantID, companyB, "SHIPPER", "Company B")
	crtestkit.SeedLocations(t, ctx, pool, tenantID, companyB, originID, destID)

	h := newHarness(t, harnessOptions{
		Pool: pool, TenantID: tenantID, UserID: userID, BuyerID: companyB, CarrierID: carrierID,
		OriginID: originID, DestID: destID,
		MembershipsByUser: map[uuid.UUID][]identityMembership{
			userID: {
				{CompanyID: companyA, CompanyType: "SHIPPER", Roles: []string{"SHIPPER_ADMIN"}},
				{CompanyID: companyB, CompanyType: "SHIPPER", Roles: []string{"SHIPPER_LOGIST"}},
			},
			managerID: {{CompanyID: companyB, CompanyType: "SHIPPER", Roles: []string{"PROCUREMENT_MANAGER"}}},
		},
	})

	createResp := h.request(managerID, companyB, "POST", "/api/v1/transport-contracts", map[string]any{
		"buyer_company_id": companyB.String(), "carrier_company_id": carrierID.String(),
		"contract_number": "BLEED-006", "name": "Company B Contract", "valid_from": "2026-01-01", "currency_code": "RUB",
	}, nil)
	mustStatus(t, "create B contract", createResp, 201)
	contractID := parseJSONField(t, createResp.Body, "id")

	readResp := h.request(userID, companyB, "GET", "/api/v1/transport-contracts/"+contractID, nil, nil)
	mustStatus(t, "logist read B", readResp, 200)

	patchResp := h.request(userID, companyB, "PATCH", "/api/v1/transport-contracts/"+contractID, map[string]any{
		"description": "bleed attempt",
	}, nil)
	if patchResp.Status != 403 {
		t.Fatalf("E-E2E-006 expected 403 got %d body=%s", patchResp.Status, string(patchResp.Body))
	}

	getAfter := h.request(userID, companyB, "GET", "/api/v1/transport-contracts/"+contractID, nil, nil)
	mustStatus(t, "re-read after deny", getAfter, 200)
	if strings.Contains(string(getAfter.Body), `"description":"bleed attempt"`) {
		t.Fatalf("mutation must not have occurred")
	}

	contractA := createAndActivateContract(t, h, userID, companyA, "BLEED-A")
	patchA := h.request(userID, companyA, "PATCH", "/api/v1/transport-contracts/"+contractA, map[string]any{
		"description": "admin on A",
	}, nil)
	mustStatus(t, "admin mutate A", patchA, 200)
}

func TestPublicSecurityHeaderSpoofing(t *testing.T) {
	h := newHarness(t, harnessOptions{})
	user := h.userID
	foreignTenant := uuid.New().String()
	foreignUser := uuid.New().String()

	resp := h.request(user, h.buyerID, "GET", "/api/v1/transport-contracts", nil, map[string]string{
		"X-Tenant-ID":              foreignTenant,
		"X-User-ID":                foreignUser,
		"X-Actor-Kind":             "CARRIER",
		"X-Internal-Service-Token": "spoof-token",
	})
	mustStatus(t, "list with spoof headers", resp, 200)

	// membership required
	resp = h.request(user, uuid.New(), "GET", "/api/v1/transport-contracts", nil, nil)
	if resp.Status != 403 {
		t.Fatalf("unknown company expected 403 got %d", resp.Status)
	}
}

func createAndActivateContract(t *testing.T, h *harness, user, buyerCompany uuid.UUID, number string) string {
	t.Helper()
	resp := h.request(user, buyerCompany, "POST", "/api/v1/transport-contracts", map[string]any{
		"buyer_company_id": buyerCompany.String(), "carrier_company_id": h.carrierID.String(),
		"contract_number": number, "name": "Contract " + number, "valid_from": "2026-01-01", "currency_code": "RUB",
	}, nil)
	mustStatus(t, "create contract", resp, 201)
	contractID := parseJSONField(t, resp.Body, "id")
	resp = h.request(user, buyerCompany, "POST", "/api/v1/transport-contracts/"+contractID+"/activate", map[string]any{}, nil)
	mustStatus(t, "activate contract", resp, 200)
	return contractID
}

func createActiveRateStack(t *testing.T, h *harness, user, buyerCompany uuid.UUID, contractID, equipment, baseAmount, fuelPercent string) (rateCardID, versionID, lineID string) {
	t.Helper()
	resp := h.request(user, buyerCompany, "POST", "/api/v1/transport-contracts/"+contractID+"/rate-cards", map[string]any{"name": "Card"}, nil)
	mustStatus(t, "create card", resp, 201)
	rateCardID = parseJSONField(t, resp.Body, "id")

	resp = h.request(user, buyerCompany, "POST", "/api/v1/rate-cards/"+rateCardID+"/versions", map[string]any{"valid_from": "2026-01-01"}, nil)
	mustStatus(t, "create version", resp, 201)
	versionID = parseJSONField(t, resp.Body, "id")

	resp = h.request(user, buyerCompany, "POST", "/api/v1/rate-card-versions/"+versionID+"/rate-lines", map[string]any{
		"origin_location_id": h.originID.String(), "destination_location_id": h.destID.String(),
		"equipment_type": equipment, "transport_mode": "ROAD",
	}, nil)
	mustStatus(t, "create line", resp, 201)
	lineID = parseJSONField(t, resp.Body, "id")

	for _, comp := range []map[string]any{
		{"component_type": "BASE_FREIGHT", "calculation_method": "FLAT", "amount": baseAmount},
		{"component_type": "FUEL_SURCHARGE", "calculation_method": "PERCENT", "percent_value": fuelPercent},
		{"component_type": "WAITING", "calculation_method": "UNIT_RATE", "amount": "500.00", "unit_code": "HOUR"},
		{"component_type": "DETENTION", "calculation_method": "UNIT_RATE", "amount": "700.00", "unit_code": "HOUR"},
	} {
		resp = h.request(user, buyerCompany, "POST", "/api/v1/rate-lines/"+lineID+"/components", comp, nil)
		mustStatus(t, "component", resp, 201)
	}

	resp = h.request(user, buyerCompany, "POST", "/api/v1/rate-card-versions/"+versionID+"/activate", map[string]any{}, nil)
	mustStatus(t, "activate version", resp, 200)
	return rateCardID, versionID, lineID
}

func withField(base map[string]any, key string, value any) map[string]any {
	out := map[string]any{}
	for k, v := range base {
		out[k] = v
	}
	out[key] = value
	return out
}

func firstItemID(t *testing.T, body []byte) string {
	t.Helper()
	var payload struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode items: %v body=%s", err, string(body))
	}
	if len(payload.Items) == 0 || payload.Items[0].ID == "" {
		t.Fatalf("expected items in %s", string(body))
	}
	return payload.Items[0].ID
}
