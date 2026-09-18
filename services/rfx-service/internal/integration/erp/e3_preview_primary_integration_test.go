//go:build integration

package erp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/rfx-service/internal/erpjson"
)

func validUpdatePayload() string {
	return `{
		"schema_version":"BINTRANS_RFX_ERP_JSON_V1",
		"requested_operation":"UPDATE_DRAFT",
		"event":{"type":"SPOT_RFQ","title":"ERP Update","currency":"USD_EXT","timezone":"UTC_EXT"}
	}`
}

func seedPreviewMappings(t *testing.T, env *testEnv, tenantID uuid.UUID) {
	t.Helper()
	seedPlatformMappingSet(t, env, tenantID, "CURRENCY", 1, "USD_EXT", "USD")
	seedPlatformMappingSet(t, env, tenantID, "TIMEZONE", 1, "UTC_EXT", "UTC")
}

func grantPreviewCreate(t *testing.T, env *testEnv, tenantID, principalID uuid.UUID) {
	t.Helper()
	grantScope(t, env, tenantID, principalID, domain.ScopeDraftPreview)
	grantScope(t, env, tenantID, principalID, domain.ScopeDraftCreate)
}

func grantPreviewUpdate(t *testing.T, env *testEnv, tenantID, principalID uuid.UUID) {
	t.Helper()
	grantScope(t, env, tenantID, principalID, domain.ScopeDraftPreview)
	grantScope(t, env, tenantID, principalID, domain.ScopeDraftRead)
}

func TestE7P2INT142UnknownSchemaVersion(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID := seedTenantCompany(t, env)
	principal := seedIntegrationPrincipal(t, env, tenantID, companyID, "int142-client")
	grantPreviewCreate(t, env, tenantID, principal.ID)
	router := newERPPreviewRouter(t, env, enabledERPIntegrationConfig())
	payload := strings.Replace(validCreatePayload(), "BINTRANS_RFX_ERP_JSON_V1", "UNKNOWN_SCHEMA", 1)
	rec := postERPPreview(t, router, "/v1/integrations/erp/rfx/drafts/preview", tenantID, companyID, principal.ID,
		[]string{domain.ScopeDraftPreview, domain.ScopeDraftCreate}, []byte(payload))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), erpjson.MachineCodeUnsupportedSchema) {
		t.Fatalf("expected unsupported_schema, body=%s", rec.Body.String())
	}
}

func TestE7P2INT147OversizedPayload(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID := seedTenantCompany(t, env)
	principal := seedIntegrationPrincipal(t, env, tenantID, companyID, "int147-client")
	grantPreviewCreate(t, env, tenantID, principal.ID)
	router := newERPPreviewRouter(t, env, enabledERPIntegrationConfig())
	body := make([]byte, erpjson.MaxBodyBytes+1)
	rec := postERPPreview(t, router, "/v1/integrations/erp/rfx/drafts/preview", tenantID, companyID, principal.ID,
		[]string{domain.ScopeDraftPreview, domain.ScopeDraftCreate}, body)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestE7P2INT148UpdatePreviewOnDraft(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID := seedTenantCompany(t, env)
	seedPreviewMappings(t, env, tenantID)
	eventID := seedDraftRfxEvent(t, env, tenantID, companyID)
	principal := seedIntegrationPrincipal(t, env, tenantID, companyID, "int148-client")
	grantPreviewUpdate(t, env, tenantID, principal.ID)
	router := newERPPreviewRouter(t, env, enabledERPIntegrationConfig())
	before := countERPAnalyses(t, env, tenantID)
	rec := postERPUpdatePreview(t, router, eventID, tenantID, companyID, principal.ID,
		[]string{domain.ScopeDraftPreview, domain.ScopeDraftRead}, []byte(validUpdatePayload()))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if countERPAnalyses(t, env, tenantID) != before+1 {
		t.Fatal("expected analysis persisted")
	}
}

func TestE7P2INT149UpdatePreviewOnPublished409(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID := seedTenantCompany(t, env)
	eventID := seedPublishedRfxEvent(t, env, tenantID, companyID)
	principal := seedIntegrationPrincipal(t, env, tenantID, companyID, "int149-client")
	grantPreviewUpdate(t, env, tenantID, principal.ID)
	router := newERPPreviewRouter(t, env, enabledERPIntegrationConfig())
	rec := postERPUpdatePreview(t, router, eventID, tenantID, companyID, principal.ID,
		[]string{domain.ScopeDraftPreview, domain.ScopeDraftRead}, []byte(validUpdatePayload()))
	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestE7P2INT165UnknownCurrency422(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID := seedTenantCompany(t, env)
	principal := seedIntegrationPrincipal(t, env, tenantID, companyID, "int165-client")
	grantPreviewCreate(t, env, tenantID, principal.ID)
	router := newERPPreviewRouter(t, env, enabledERPIntegrationConfig())
	rec := postERPPreview(t, router, "/v1/integrations/erp/rfx/drafts/preview", tenantID, companyID, principal.ID,
		[]string{domain.ScopeDraftPreview, domain.ScopeDraftCreate}, []byte(validCreatePayload()))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "external_source") || !strings.Contains(rec.Body.String(), "USD_EXT") {
		t.Fatalf("expected external_source for currency, body=%s", rec.Body.String())
	}
}

func TestE7P2INT166UnknownUnit422(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID := seedTenantCompany(t, env)
	seedPreviewMappings(t, env, tenantID)
	principal := seedIntegrationPrincipal(t, env, tenantID, companyID, "int166-client")
	grantPreviewCreate(t, env, tenantID, principal.ID)
	router := newERPPreviewRouter(t, env, enabledERPIntegrationConfig())
	payload := strings.TrimSuffix(validCreatePayload(), "}") + `,"extensions":{"freight":{"unit_code":"UNKNOWN_UNIT"}}}`
	rec := postERPPreview(t, router, "/v1/integrations/erp/rfx/drafts/preview", tenantID, companyID, principal.ID,
		[]string{domain.ScopeDraftPreview, domain.ScopeDraftCreate}, []byte(payload))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestE7P2INT167UnknownCargoWarning200(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID := seedTenantCompany(t, env)
	seedPreviewMappings(t, env, tenantID)
	principal := seedIntegrationPrincipal(t, env, tenantID, companyID, "int167-client")
	grantPreviewCreate(t, env, tenantID, principal.ID)
	router := newERPPreviewRouter(t, env, enabledERPIntegrationConfig())
	payload := strings.TrimSuffix(validCreatePayload(), "}") + `,"extensions":{"freight":{"cargo_type":"UNKNOWN_CARGO"}}}`
	rec := postERPPreview(t, router, "/v1/integrations/erp/rfx/drafts/preview", tenantID, companyID, principal.ID,
		[]string{domain.ScopeDraftPreview, domain.ScopeDraftCreate}, []byte(payload))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), erpjson.MachineCodeMappingNotFound) {
		t.Fatalf("expected mapping warning, body=%s", rec.Body.String())
	}
}

func TestE7P2INT168TenantOverrideMapping(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID := seedTenantCompany(t, env)
	platformCurrencySetID := seedPlatformMappingSet(t, env, tenantID, "CURRENCY", 1, "USD_EXT", "USD")
	tenantCurrencySetID := seedTenantMappingSet(t, env, tenantID, "CURRENCY", 2, "USD_EXT", "EUR")
	seedPlatformMappingSet(t, env, tenantID, "TIMEZONE", 1, "UTC_EXT", "UTC")
	principal := seedIntegrationPrincipal(t, env, tenantID, companyID, "int168-client")
	grantPreviewCreate(t, env, tenantID, principal.ID)
	router := newERPPreviewRouter(t, env, enabledERPIntegrationConfig())
	rec := postERPPreview(t, router, "/v1/integrations/erp/rfx/drafts/preview", tenantID, companyID, principal.ID,
		[]string{domain.ScopeDraftPreview, domain.ScopeDraftCreate}, []byte(validCreatePayload()))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	analysisID := mustPreviewAnalysisID(t, rec.Body.Bytes())
	doc := decodeCanonicalPreviewDoc(t, fetchAnalysisCanonicalJSON(t, env, analysisID))
	if doc.Event.Currency != "EUR" {
		t.Fatalf("expected tenant override currency EUR, got %q", doc.Event.Currency)
	}
	if doc.MappingContext.MappingSetVersion != 2 {
		t.Fatalf("expected tenant mapping_set_version=2, got %d", doc.MappingContext.MappingSetVersion)
	}
	if doc.MappingContext.MappingSetID != tenantCurrencySetID {
		t.Fatalf("expected tenant mapping_set_id=%s, got %s", tenantCurrencySetID, doc.MappingContext.MappingSetID)
	}
	if doc.MappingContext.MappingSetID == platformCurrencySetID {
		t.Fatal("tenant override must not pin the platform currency mapping set")
	}
	if !hasMappingType(doc.MappingContext.MappingTypes, "CURRENCY") {
		t.Fatalf("expected CURRENCY in mapping_types_applied, got %v", doc.MappingContext.MappingTypes)
	}
}

func TestE7P2INT169RetiredMappingSet422(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID := seedTenantCompany(t, env)
	seedRetiredMappingSet(t, env, tenantID, nil, "CURRENCY", 2, "USD_EXT", "USD")
	seedPlatformMappingSet(t, env, tenantID, "TIMEZONE", 1, "UTC_EXT", "UTC")
	principal := seedIntegrationPrincipal(t, env, tenantID, companyID, "int169-client")
	grantPreviewCreate(t, env, tenantID, principal.ID)
	router := newERPPreviewRouter(t, env, enabledERPIntegrationConfig())
	rec := postERPPreview(t, router, "/v1/integrations/erp/rfx/drafts/preview", tenantID, companyID, principal.ID,
		[]string{domain.ScopeDraftPreview, domain.ScopeDraftCreate}, []byte(validCreatePayload()))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), erpjson.MachineCodeMappingRetired) {
		t.Fatalf("expected mapping_retired, body=%s", rec.Body.String())
	}
}

func TestE7P2INT170PlatformDefaultMapping(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID := seedTenantCompany(t, env)
	otherTenant, _ := seedTenantCompany(t, env)
	seedPlatformMappingSet(t, env, otherTenant, "CURRENCY", 1, "USD_EXT", "USD")
	seedPlatformMappingSet(t, env, otherTenant, "TIMEZONE", 1, "UTC_EXT", "UTC")
	principal := seedIntegrationPrincipal(t, env, tenantID, companyID, "int170-client")
	grantPreviewCreate(t, env, tenantID, principal.ID)
	router := newERPPreviewRouter(t, env, enabledERPIntegrationConfig())
	rec := postERPPreview(t, router, "/v1/integrations/erp/rfx/drafts/preview", tenantID, companyID, principal.ID,
		[]string{domain.ScopeDraftPreview, domain.ScopeDraftCreate}, []byte(validCreatePayload()))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestE7P2INT171MappingContextInCanonicalHash(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID := seedTenantCompany(t, env)
	currencySetID := seedPlatformMappingSet(t, env, tenantID, "CURRENCY", 1, "USD_EXT", "USD")
	seedPlatformMappingSet(t, env, tenantID, "TIMEZONE", 1, "UTC_EXT", "UTC")
	principal := seedIntegrationPrincipal(t, env, tenantID, companyID, "int171-client")
	grantPreviewCreate(t, env, tenantID, principal.ID)
	router := newERPPreviewRouter(t, env, enabledERPIntegrationConfig())
	rec := postERPPreview(t, router, "/v1/integrations/erp/rfx/drafts/preview", tenantID, companyID, principal.ID,
		[]string{domain.ScopeDraftPreview, domain.ScopeDraftCreate}, []byte(validCreatePayload()))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp erpjson.PreviewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode preview response: %v", err)
	}
	analysisID := mustPreviewAnalysisID(t, rec.Body.Bytes())
	payload, storedHash := fetchAnalysisCanonicalRecord(t, env, analysisID)
	doc := decodeCanonicalPreviewDoc(t, payload)
	if doc.Event.Currency != "USD" {
		t.Fatalf("expected resolved currency USD, got %q", doc.Event.Currency)
	}
	if doc.MappingContext.MappingSetID == uuid.Nil {
		t.Fatal("mapping_context.mapping_set_id must be pinned")
	}
	if doc.MappingContext.MappingSetID != currencySetID {
		t.Fatalf("expected pinned currency mapping_set_id=%s, got %s", currencySetID, doc.MappingContext.MappingSetID)
	}
	if doc.MappingContext.MappingSetVersion != 1 {
		t.Fatalf("expected pinned mapping_set_version=1, got %d", doc.MappingContext.MappingSetVersion)
	}
	if !hasMappingType(doc.MappingContext.MappingTypes, "CURRENCY") {
		t.Fatalf("expected CURRENCY in mapping_types_applied, got %v", doc.MappingContext.MappingTypes)
	}
	if strings.TrimSpace(resp.CanonicalPayloadHash) == "" {
		t.Fatal("preview response must return canonical_payload_hash")
	}
	if storedHash != resp.CanonicalPayloadHash {
		t.Fatalf("persisted hash %s != response hash %s", storedHash, resp.CanonicalPayloadHash)
	}
	semanticHash, err := erpjson.StableHash(payload)
	if err != nil {
		t.Fatalf("stable hash: %v", err)
	}
	if semanticHash != storedHash {
		t.Fatalf("canonical hash must match normalized payload including resolved codes: stored=%s computed=%s", storedHash, semanticHash)
	}
	var clone map[string]any
	if err := json.Unmarshal(payload, &clone); err != nil {
		t.Fatalf("clone canonical payload: %v", err)
	}
	event, ok := clone["event"].(map[string]any)
	if !ok {
		t.Fatalf("canonical event object missing: %#v", clone["event"])
	}
	if event["currency"] != "USD" {
		t.Fatalf("cloned event currency=%v", event["currency"])
	}
	event["currency"] = "USD_EXT"
	altBytes, err := json.Marshal(clone)
	if err != nil {
		t.Fatalf("marshal alternate payload: %v", err)
	}
	altHash, err := erpjson.StableHash(altBytes)
	if err != nil {
		t.Fatalf("alternate stable hash: %v", err)
	}
	if altHash == semanticHash {
		t.Fatal("resolved currency must participate in canonical hash semantics")
	}
}

func TestE7P2INT172DuplicateSectionCode422(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID := seedTenantCompany(t, env)
	seedPreviewMappings(t, env, tenantID)
	principal := seedIntegrationPrincipal(t, env, tenantID, companyID, "int172-client")
	grantPreviewCreate(t, env, tenantID, principal.ID)
	router := newERPPreviewRouter(t, env, enabledERPIntegrationConfig())
	payload := strings.TrimSuffix(validCreatePayload(), "}") + `,"questionnaire":{"sections":[{"section_code":"S1","title":"A"},{"section_code":"S1","title":"B"}]}}`
	rec := postERPPreview(t, router, "/v1/integrations/erp/rfx/drafts/preview", tenantID, companyID, principal.ID,
		[]string{domain.ScopeDraftPreview, domain.ScopeDraftCreate}, []byte(payload))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestE7P2INT173RuleCycle422(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID := seedTenantCompany(t, env)
	seedPreviewMappings(t, env, tenantID)
	principal := seedIntegrationPrincipal(t, env, tenantID, companyID, "int173-client")
	grantPreviewCreate(t, env, tenantID, principal.ID)
	router := newERPPreviewRouter(t, env, enabledERPIntegrationConfig())
	payload := strings.TrimSuffix(validCreatePayload(), "}") + `,"questionnaire":{"sections":[{"section_code":"S1","title":"S1"}],"rules":[
		{"rule_code":"R1","target_question_code":"B","action":"SHOW","condition":{"operator":"EQUALS","source_question_code":"A","value":true}},
		{"rule_code":"R2","target_question_code":"A","action":"SHOW","condition":{"operator":"IS_NOT_EMPTY","source_question_code":"B"}}
	]}}`
	rec := postERPPreview(t, router, "/v1/integrations/erp/rfx/drafts/preview", tenantID, companyID, principal.ID,
		[]string{domain.ScopeDraftPreview, domain.ScopeDraftCreate}, []byte(payload))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestE7P2INT174MaxLotsExceeded422(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID := seedTenantCompany(t, env)
	seedPreviewMappings(t, env, tenantID)
	principal := seedIntegrationPrincipal(t, env, tenantID, companyID, "int174-client")
	grantPreviewCreate(t, env, tenantID, principal.ID)
	router := newERPPreviewRouter(t, env, enabledERPIntegrationConfig())
	var lots []string
	for i := 0; i <= erpjson.MaxLots; i++ {
		lots = append(lots, fmt.Sprintf(`{"lot_number":"L%d","name":"Lot %d"}`, i, i))
	}
	payload := strings.TrimSuffix(validCreatePayload(), "}") + `,"lots":[` + strings.Join(lots, ",") + "]}"
	rec := postERPPreview(t, router, "/v1/integrations/erp/rfx/drafts/preview", tenantID, companyID, principal.ID,
		[]string{domain.ScopeDraftPreview, domain.ScopeDraftCreate}, []byte(payload))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestE7P2INT175JSONDepthBomb422(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID := seedTenantCompany(t, env)
	principal := seedIntegrationPrincipal(t, env, tenantID, companyID, "int175-client")
	grantPreviewCreate(t, env, tenantID, principal.ID)
	router := newERPPreviewRouter(t, env, enabledERPIntegrationConfig())
	depth := erpjson.MaxJSONDepth + 2
	payload := strings.Repeat(`{"nested":`, depth) + `"leaf":1` + strings.Repeat("}", depth)
	rec := postERPPreview(t, router, "/v1/integrations/erp/rfx/drafts/preview", tenantID, companyID, principal.ID,
		[]string{domain.ScopeDraftPreview, domain.ScopeDraftCreate}, []byte(payload))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestE7P2INT176InvalidRequestedOperation422(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID := seedTenantCompany(t, env)
	seedPreviewMappings(t, env, tenantID)
	principal := seedIntegrationPrincipal(t, env, tenantID, companyID, "int176-client")
	grantPreviewCreate(t, env, tenantID, principal.ID)
	router := newERPPreviewRouter(t, env, enabledERPIntegrationConfig())
	payload := strings.Replace(validCreatePayload(), "CREATE_DRAFT", "DELETE_EVERYTHING", 1)
	rec := postERPPreview(t, router, "/v1/integrations/erp/rfx/drafts/preview", tenantID, companyID, principal.ID,
		[]string{domain.ScopeDraftPreview, domain.ScopeDraftCreate}, []byte(payload))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

type canonicalPreviewDoc struct {
	Event          erpjson.EventPayload      `json:"event"`
	MappingContext erpjson.MappingContextPin `json:"mapping_context"`
}

func decodeCanonicalPreviewDoc(t *testing.T, payload []byte) canonicalPreviewDoc {
	t.Helper()
	var doc canonicalPreviewDoc
	if err := json.Unmarshal(payload, &doc); err != nil {
		t.Fatalf("decode canonical payload: %v", err)
	}
	if doc.MappingContext.MappingSetID == uuid.Nil && doc.MappingContext.MappingSetVersion == 0 && len(doc.MappingContext.MappingTypes) == 0 {
		t.Fatal("mapping_context must be present after JSON unmarshal")
	}
	return doc
}

func mustPreviewAnalysisID(t *testing.T, body []byte) uuid.UUID {
	t.Helper()
	var resp erpjson.PreviewResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode preview response: %v", err)
	}
	if resp.AnalysisID == nil || *resp.AnalysisID == uuid.Nil {
		t.Fatal("expected analysis_id in preview response")
	}
	return *resp.AnalysisID
}

func hasMappingType(types []string, want string) bool {
	for _, got := range types {
		if got == want {
			return true
		}
	}
	return false
}
