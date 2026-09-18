//go:build integration

package erp

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/rfx-service/internal/erpjson"
)

func TestE3_1ContentTypeEnforcement(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID := seedTenantCompany(t, env)
	seedPreviewMappings(t, env, tenantID)
	principal := seedIntegrationPrincipal(t, env, tenantID, companyID, "e31-ct-client")
	grantPreviewCreate(t, env, tenantID, principal.ID)
	router := newERPPreviewRouter(t, env, enabledERPIntegrationConfig())
	scopes := []string{domain.ScopeDraftPreview, domain.ScopeDraftCreate}
	body := []byte(validCreatePayload())

	accept := []string{"application/json", "application/json; charset=utf-8", "application/json; charset=UTF-8"}
	for _, ct := range accept {
		before := countERPAnalyses(t, env, tenantID)
		rec := postERPPreviewWithContentType(t, router, "/v1/integrations/erp/rfx/drafts/preview", tenantID, companyID, principal.ID, scopes, body, ct)
		if rec.Code != http.StatusOK {
			t.Fatalf("content-type %q status=%d body=%s", ct, rec.Code, rec.Body.String())
		}
		if countERPAnalyses(t, env, tenantID) != before+1 {
			t.Fatalf("content-type %q should persist analysis", ct)
		}
	}

	reject := []string{"", "application/json;;", "text/plain", "application/x-www-form-urlencoded", "multipart/form-data; boundary=x", "application/json; charset=utf-16"}
	for _, ct := range reject {
		before := countERPAnalyses(t, env, tenantID)
		rec := postERPPreviewWithContentType(t, router, "/v1/integrations/erp/rfx/drafts/preview", tenantID, companyID, principal.ID, scopes, body, ct)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("content-type %q status=%d body=%s", ct, rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), `"VALIDATION_ERROR"`) || !strings.Contains(rec.Body.String(), erpjson.MachineCodeUnsupportedMediaType) {
			t.Fatalf("content-type %q body=%s", ct, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), "USD_EXT") || strings.Contains(rec.Body.String(), "OBJ-135") {
			t.Fatalf("content-type %q leaked payload: %s", ct, rec.Body.String())
		}
		if countERPAnalyses(t, env, tenantID) != before {
			t.Fatalf("content-type %q must not persist analysis", ct)
		}
	}

	grantScope(t, env, tenantID, principal.ID, domain.ScopeDraftRead)
	eventID := seedDraftRfxEvent(t, env, tenantID, companyID)
	updateBody := []byte(strings.Replace(validCreatePayload(), "CREATE_DRAFT", "UPDATE_DRAFT", 1))
	rec := postERPPreviewWithContentType(t, router, "/v1/rfx-events/"+eventID.String()+"/erp-import/preview", tenantID, companyID, principal.ID,
		[]string{domain.ScopeDraftPreview, domain.ScopeDraftRead}, updateBody, "text/plain")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("UPDATE content-type reject status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestE3_1NestedUnknownFields(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID := seedTenantCompany(t, env)
	principal := seedIntegrationPrincipal(t, env, tenantID, companyID, "e31-nested-client")
	grantPreviewCreate(t, env, tenantID, principal.ID)
	router := newERPPreviewRouter(t, env, enabledERPIntegrationConfig())
	scopes := []string{domain.ScopeDraftPreview, domain.ScopeDraftCreate}

	cases := []struct {
		name string
		body string
		path string
	}{
		{"event", `{"schema_version":"BINTRANS_RFX_ERP_JSON_V1","requested_operation":"CREATE_DRAFT","external":{"system":"SAP","object_id":"OBJ-135"},"event":{"type":"SPOT_RFQ","title":"ERP Preview","currency":"USD_EXT","timezone":"UTC_EXT","owner_id":"x"}}`, "event.owner_id"},
		{"external", `{"schema_version":"BINTRANS_RFX_ERP_JSON_V1","requested_operation":"CREATE_DRAFT","external":{"system":"SAP","object_id":"OBJ-135","tenant_id":"x"},"event":{"type":"SPOT_RFQ","title":"ERP Preview","currency":"USD_EXT","timezone":"UTC_EXT"}}`, "external.tenant_id"},
		{"lots", `{"schema_version":"BINTRANS_RFX_ERP_JSON_V1","requested_operation":"CREATE_DRAFT","external":{"system":"SAP","object_id":"OBJ-135"},"event":{"type":"SPOT_RFQ","title":"ERP Preview","currency":"USD_EXT","timezone":"UTC_EXT"},"lots":[{"lot_number":"L1","name":"Lot","status":"OPEN"}]}`, "lots[0].status"},
	}
	for _, tc := range cases {
		before := countERPAnalyses(t, env, tenantID)
		rec := postERPPreview(t, router, "/v1/integrations/erp/rfx/drafts/preview", tenantID, companyID, principal.ID, scopes, []byte(tc.body))
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("%s status=%d body=%s", tc.name, rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), erpjson.MachineCodeUnknownField) || !strings.Contains(rec.Body.String(), tc.path) {
			t.Fatalf("%s expected unknown_field %s, body=%s", tc.name, tc.path, rec.Body.String())
		}
		if countERPAnalyses(t, env, tenantID) != before {
			t.Fatalf("%s persisted analysis", tc.name)
		}
	}
}

func TestE3_1TrailingAndDuplicateAndRawExtensions(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID := seedTenantCompany(t, env)
	seedPreviewMappings(t, env, tenantID)
	seedPlatformMappingSet(t, env, tenantID, "UNIT", 1, "TNE_EXT", "TNE")
	principal := seedIntegrationPrincipal(t, env, tenantID, companyID, "e31-struct-client")
	grantPreviewCreate(t, env, tenantID, principal.ID)
	router := newERPPreviewRouter(t, env, enabledERPIntegrationConfig())
	scopes := []string{domain.ScopeDraftPreview, domain.ScopeDraftCreate}

	trailing := validCreatePayload() + `{"x":1}`
	rec := postERPPreview(t, router, "/v1/integrations/erp/rfx/drafts/preview", tenantID, companyID, principal.ID, scopes, []byte(trailing))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("trailing status=%d body=%s", rec.Code, rec.Body.String())
	}

	dup := `{"schema_version":"BINTRANS_RFX_ERP_JSON_V1","requested_operation":"CREATE_DRAFT","external":{"system":"SAP","object_id":"OBJ-135"},"event":{"type":"SPOT_RFQ","title":"A","title":"B","currency":"USD_EXT","timezone":"UTC_EXT"}}`
	rec = postERPPreview(t, router, "/v1/integrations/erp/rfx/drafts/preview", tenantID, companyID, principal.ID, scopes, []byte(dup))
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), erpjson.MachineCodeDuplicateField) {
		t.Fatalf("duplicate status=%d body=%s", rec.Code, rec.Body.String())
	}

	ext := strings.TrimSuffix(validCreatePayload(), "}") + `,"extensions":{"freight":{"unit_code":"TNE_EXT","custom_note":"keep"}}}`
	before := countERPAnalyses(t, env, tenantID)
	rec = postERPPreview(t, router, "/v1/integrations/erp/rfx/drafts/preview", tenantID, companyID, principal.ID, scopes, []byte(ext))
	if rec.Code != http.StatusOK {
		t.Fatalf("raw extensions status=%d body=%s", rec.Code, rec.Body.String())
	}
	if countERPAnalyses(t, env, tenantID) != before+1 {
		t.Fatal("raw extensions should persist analysis")
	}
}

func TestE3_1MappingPinsAndStableErrorKey(t *testing.T) {
	env := setupTestEnv(t)
	tenantID, companyID := seedTenantCompany(t, env)
	currencySet := seedPlatformMappingSet(t, env, tenantID, "CURRENCY", 1, "USD_EXT", "USD")
	tzSet := seedPlatformMappingSet(t, env, tenantID, "TIMEZONE", 3, "UTC_EXT", "UTC")
	principal := seedIntegrationPrincipal(t, env, tenantID, companyID, "e31-pin-client")
	grantPreviewCreate(t, env, tenantID, principal.ID)
	router := newERPPreviewRouter(t, env, enabledERPIntegrationConfig())
	rec := postERPPreview(t, router, "/v1/integrations/erp/rfx/drafts/preview", tenantID, companyID, principal.ID,
		[]string{domain.ScopeDraftPreview, domain.ScopeDraftCreate}, []byte(validCreatePayload()))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	doc := decodeCanonicalPreviewDoc(t, fetchAnalysisCanonicalJSON(t, env, mustPreviewAnalysisID(t, rec.Body.Bytes())))
	if len(doc.MappingContext.Pins) != 2 {
		t.Fatalf("pins=%+v", doc.MappingContext.Pins)
	}
	if doc.MappingContext.MappingSetID != currencySet {
		t.Fatalf("primary pin=%s want currency %s", doc.MappingContext.MappingSetID, currencySet)
	}
	foundCurrency, foundTZ := false, false
	for _, pin := range doc.MappingContext.Pins {
		if pin.MappingType == "CURRENCY" && pin.MappingSetID == currencySet && pin.MappingSetVersion == 1 {
			foundCurrency = true
		}
		if pin.MappingType == "TIMEZONE" && pin.MappingSetID == tzSet && pin.MappingSetVersion == 3 {
			foundTZ = true
		}
	}
	if !foundCurrency || !foundTZ {
		t.Fatalf("pins=%+v", doc.MappingContext.Pins)
	}

	clientPin := strings.TrimSuffix(validCreatePayload(), "}") + `,"mapping_context":{"mapping_set_id":"` + currencySet.String() + `"}}`
	before := countERPAnalyses(t, env, tenantID)
	rec = postERPPreview(t, router, "/v1/integrations/erp/rfx/drafts/preview", tenantID, companyID, principal.ID,
		[]string{domain.ScopeDraftPreview, domain.ScopeDraftCreate}, []byte(clientPin))
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "mapping_context") || (!strings.Contains(rec.Body.String(), erpjson.MachineCodeUnknownField) && !strings.Contains(rec.Body.String(), erpjson.MachineCodeUnsupportedFieldV1)) {
		t.Fatalf("client mapping_context status=%d body=%s", rec.Code, rec.Body.String())
	}
	if countERPAnalyses(t, env, tenantID) != before {
		t.Fatal("client mapping_context persisted analysis")
	}

	missing := setupTestEnv(t)
	missingTenant, missingCompany := seedTenantCompany(t, missing)
	missingPrincipal := seedIntegrationPrincipal(t, missing, missingTenant, missingCompany, "e31-err-client")
	grantPreviewCreate(t, missing, missingTenant, missingPrincipal.ID)
	missingRouter := newERPPreviewRouter(t, missing, enabledERPIntegrationConfig())
	rec = postERPPreview(t, missingRouter, "/v1/integrations/erp/rfx/drafts/preview", missingTenant, missingCompany, missingPrincipal.ID,
		[]string{domain.ScopeDraftPreview, domain.ScopeDraftCreate}, []byte(validCreatePayload()))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("missing mapping status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"message_key":"rfx.erp.mapping_not_found"`) && !strings.Contains(rec.Body.String(), "rfx.erp.mapping_not_found") {
		t.Fatalf("expected stable mapping key, body=%s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "pq:") || strings.Contains(rec.Body.String(), "postgres://") || strings.Contains(rec.Body.String(), "JWT") {
		t.Fatalf("error leaked internals: %s", rec.Body.String())
	}
	var preview erpjson.PreviewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &preview); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, issue := range preview.Errors {
		if issue.MessageKey != "" && !strings.HasPrefix(issue.MessageKey, "rfx.erp.") {
			t.Fatalf("unstable message_key=%q", issue.MessageKey)
		}
	}
}
