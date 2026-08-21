package contractrates

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/freight-platform/api-gateway/internal/config"
	"github.com/freight-platform/api-gateway/internal/ratesrbac"
)

const testBuyerCompany = "11111111-1111-1111-1111-111111111111"
const testCarrierCompany = "22222222-2222-2222-2222-222222222222"
const testOrigin = "33333333-3333-3333-3333-333333333333"
const testDest = "44444444-4444-4444-4444-444444444444"

func buyerVC() ratesrbac.VerifiedContext {
	return ratesrbac.VerifiedContext{
		TenantID:  "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		UserID:    "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
		CompanyID: testBuyerCompany,
		ActorKind: "BUYER",
	}
}

func assertValidation(t *testing.T, label string, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: expected validation error", label)
	}
}

func assertPass(t *testing.T, label string, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: unexpected error: %v", label, err)
	}
}

func TestEDTO001CreateContractUnknownField(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"buyer_company_id":"` + testBuyerCompany + `","carrier_company_id":"` + testCarrierCompany + `","contract_number":"C-1","name":"N","valid_from":"2026-01-01","currency_code":"RUB","status":"ACTIVE"}`)
	_, err := validateAndRebuildBody("POST", "/api/v1/transport-contracts", raw, buyerVC())
	assertValidation(t, "E-DTO-001", err)
}

func TestEDTO002PatchContractUnknownField(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"name":"Updated","tenant_id":"` + testBuyerCompany + `"}`)
	_, err := validateAndRebuildBody("PATCH", "/api/v1/transport-contracts/"+testBuyerCompany, raw, buyerVC())
	assertValidation(t, "E-DTO-002", err)
}

func TestEDTO003CreateRateCardValid(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"name":"Main Contract Rates","description":"Primary 2026 rates"}`)
	_, err := validateAndRebuildBody("POST", "/api/v1/transport-contracts/"+testBuyerCompany+"/rate-cards", raw, buyerVC())
	assertPass(t, "E-DTO-003", err)
}

func TestEDTO004CreateRateCardUnknown(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"name":"Main","admin_override":true}`)
	_, err := validateAndRebuildBody("POST", "/api/v1/transport-contracts/"+testBuyerCompany+"/rate-cards", raw, buyerVC())
	assertValidation(t, "E-DTO-004", err)
}

func TestEDTO005CreateRateVersionValid(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"valid_from":"2026-01-01","valid_to":null}`)
	_, err := validateAndRebuildBody("POST", "/api/v1/rate-cards/"+testBuyerCompany+"/versions", raw, buyerVC())
	assertPass(t, "E-DTO-005", err)
}

func TestEDTO006CreateRateVersionStatusField(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"valid_from":"2026-01-01","status":"ACTIVE"}`)
	_, err := validateAndRebuildBody("POST", "/api/v1/rate-cards/"+testBuyerCompany+"/versions", raw, buyerVC())
	assertValidation(t, "E-DTO-006", err)
}

func TestEDTO007CreateRateLineValid(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"origin_location_id":"` + testOrigin + `","destination_location_id":"` + testDest + `","equipment_type":"Box","transport_mode":"ROAD"}`)
	_, err := validateAndRebuildBody("POST", "/api/v1/rate-card-versions/"+testBuyerCompany+"/rate-lines", raw, buyerVC())
	assertPass(t, "E-DTO-007", err)
}

func TestEDTO008CreateRateLineUnknown(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"origin_location_id":"` + testOrigin + `","destination_location_id":"` + testDest + `","equipment_type":"Box","transport_mode":"ROAD","tenant_id":"` + testBuyerCompany + `"}`)
	_, err := validateAndRebuildBody("POST", "/api/v1/rate-card-versions/"+testBuyerCompany+"/rate-lines", raw, buyerVC())
	assertValidation(t, "E-DTO-008", err)
}

func TestEDTO009PatchRateLineValid(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"transport_mode":"ROAD"}`)
	_, err := validateAndRebuildBody("PATCH", "/api/v1/rate-lines/"+testBuyerCompany, raw, buyerVC())
	assertPass(t, "E-DTO-009", err)
}

func TestEDTO010PatchRateLineTenantID(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"tenant_id":"` + testBuyerCompany + `"}`)
	_, err := validateAndRebuildBody("PATCH", "/api/v1/rate-lines/"+testBuyerCompany, raw, buyerVC())
	assertValidation(t, "E-DTO-010", err)
}

func TestEDTO011CreateBaseComponentValid(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"component_type":"BASE_FREIGHT","calculation_method":"FLAT","amount":"100000.00"}`)
	_, err := validateAndRebuildBody("POST", "/api/v1/rate-lines/"+testBuyerCompany+"/components", raw, buyerVC())
	assertPass(t, "E-DTO-011", err)
}

func TestEDTO012CreateComponentUnknown(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"component_type":"BASE_FREIGHT","calculation_method":"FLAT","amount":"100000.00","created_by":"` + testBuyerCompany + `"}`)
	_, err := validateAndRebuildBody("POST", "/api/v1/rate-lines/"+testBuyerCompany+"/components", raw, buyerVC())
	assertValidation(t, "E-DTO-012", err)
}

func TestEDTO013PatchComponentValid(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"amount":"100000.00"}`)
	_, err := validateAndRebuildBody("PATCH", "/api/v1/rate-components/"+testBuyerCompany, raw, buyerVC())
	assertPass(t, "E-DTO-013", err)
}

func TestEDTO014PatchComponentTypeDenied(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"component_type":"WAITING"}`)
	_, err := validateAndRebuildBody("PATCH", "/api/v1/rate-components/"+testBuyerCompany, raw, buyerVC())
	assertValidation(t, "E-DTO-014", err)
}

func TestEDTO015ActivateEmptyBody(t *testing.T) {
	t.Parallel()
	_, err := validateAndRebuildBody("POST", "/api/v1/transport-contracts/"+testBuyerCompany+"/activate", []byte(`{}`), buyerVC())
	assertPass(t, "E-DTO-015", err)
}

func TestEDTO016ActivateForceDenied(t *testing.T) {
	t.Parallel()
	_, err := validateAndRebuildBody("POST", "/api/v1/transport-contracts/"+testBuyerCompany+"/activate", []byte(`{"force":true}`), buyerVC())
	assertValidation(t, "E-DTO-016", err)
}

func TestEDTO017TerminateReason(t *testing.T) {
	t.Parallel()
	_, err := validateAndRebuildBody("POST", "/api/v1/transport-contracts/"+testBuyerCompany+"/terminate", []byte(`{"termination_reason":"End of term"}`), buyerVC())
	assertPass(t, "E-DTO-017", err)
}

func TestEDTO018TerminateUnknown(t *testing.T) {
	t.Parallel()
	_, err := validateAndRebuildBody("POST", "/api/v1/transport-contracts/"+testBuyerCompany+"/terminate", []byte(`{"termination_reason":"x","force":true}`), buyerVC())
	assertValidation(t, "E-DTO-018", err)
}

func TestEDTO019ResolverManualSpot(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"buyer_company_id":"` + testBuyerCompany + `","carrier_company_id":"` + testCarrierCompany + `","origin_location_id":"` + testOrigin + `","destination_location_id":"` + testDest + `","equipment_type":"Box","transport_mode":"ROAD","pricing_date":"2026-01-01","manual_spot_amount":"100.00"}`)
	_, err := validateAndRebuildBody("POST", "/api/v1/rates/resolve", raw, buyerVC())
	assertValidation(t, "E-DTO-019", err)
}

func TestEDTO020ResolverUnknownField(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"buyer_company_id":"` + testBuyerCompany + `","carrier_company_id":"` + testCarrierCompany + `","origin_location_id":"` + testOrigin + `","destination_location_id":"` + testDest + `","equipment_type":"Box","transport_mode":"ROAD","pricing_date":"2026-01-01","extra":true}`)
	_, err := validateAndRebuildBody("POST", "/api/v1/rates/resolve", raw, buyerVC())
	assertValidation(t, "E-DTO-020", err)
}

func TestEDTO021MultipleJSONDocuments(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"name":"A"}{"name":"B"}`)
	_, err := validateAndRebuildBody("POST", "/api/v1/transport-contracts/"+testBuyerCompany+"/rate-cards", raw, buyerVC())
	assertValidation(t, "E-DTO-021", err)
}

func TestEDTO022MalformedJSON(t *testing.T) {
	t.Parallel()
	raw := []byte(`{name: broken}`)
	_, err := validateAndRebuildBody("POST", "/api/v1/transport-contracts/"+testBuyerCompany+"/rate-cards", raw, buyerVC())
	assertValidation(t, "E-DTO-022", err)
}

func TestPublicResolveDeniesManualSpot(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"buyer_company_id":"` + testBuyerCompany + `","carrier_company_id":"` + testCarrierCompany + `","origin_location_id":"` + testOrigin + `","destination_location_id":"` + testDest + `","equipment_type":"BOX","transport_mode":"ROAD","pricing_date":"2026-01-01","manual_spot_amount":"100.00"}`)
	_, err := validateAndRebuildBody("POST", "/api/v1/rates/resolve", raw, buyerVC())
	assertValidation(t, "manual spot", err)
}

func TestInvalidPublicDTODoesNotCallDownstream(t *testing.T) {
	var downstreamCalls int32
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&downstreamCalls, 1)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"` + testBuyerCompany + `"}`))
	}))
	t.Cleanup(downstream.Close)

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(log, config.Config{
		ProxyTimeoutSeconds:  5,
		InternalServiceToken: "test-token",
		Services:             config.ServiceURLs{ContractRate: downstream.URL},
	})

	body := `{"name":"Main","future_internal_field":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/transport-contracts/"+testBuyerCompany+"/rate-cards", strings.NewReader(body))
	req = req.WithContext(ratesrbac.WithVerifiedContext(req.Context(), buyerVC()))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d body=%s", rec.Code, rec.Body.String())
	}
	if atomic.LoadInt32(&downstreamCalls) != 0 {
		t.Fatalf("expected 0 downstream calls, got %d", downstreamCalls)
	}
}
