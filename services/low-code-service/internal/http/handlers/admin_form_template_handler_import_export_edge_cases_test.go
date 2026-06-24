package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/low-code-service/internal/domain"
	"github.com/freight-platform/low-code-service/internal/service"
)

func importPreviewRequest(body []byte, tenantID string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/form-templates/import-preview", bytes.NewReader(body))
	if tenantID != "" {
		req.Header.Set(tenantHeader, tenantID)
	}
	return req
}

func importExecuteRequest(body []byte, tenantID string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/form-templates/import", bytes.NewReader(body))
	if tenantID != "" {
		req.Header.Set(tenantHeader, tenantID)
	}
	return req
}

func TestAdminExportArchivedAllowed(t *testing.T) {
	detail := exportTemplateDetail(domain.ArchivedStatus)
	stub := &stubAdminFormTemplateRepo{getDetail: detail}
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(stub))
	rec := httptest.NewRecorder()
	handler.Export(rec, exportRequest(detail.ID, detail.TenantID))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload domain.TemplateExportEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.Template.Status != domain.ArchivedStatus {
		t.Fatalf("status = %q", payload.Template.Status)
	}
}

func TestAdminExportPreservesSectionFieldOrder(t *testing.T) {
	detail := exportTemplateDetail(domain.PublishedStatus)
	detail.Sections[0].SortOrder = 200
	detail.Sections[0].Fields[0].SortOrder = 300
	stub := &stubAdminFormTemplateRepo{getDetail: detail}
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(stub))
	rec := httptest.NewRecorder()
	handler.Export(rec, exportRequest(detail.ID, detail.TenantID))

	var payload domain.TemplateExportEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.Template.Sections[0].SortOrder != 200 {
		t.Fatalf("section sort_order = %d", payload.Template.Sections[0].SortOrder)
	}
	if payload.Template.Sections[0].Fields[0].SortOrder != 300 {
		t.Fatalf("field sort_order = %d", payload.Template.Sections[0].Fields[0].SortOrder)
	}
}

func TestAdminImportPreviewInvalidJSON(t *testing.T) {
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(&stubAdminFormTemplateRepo{}))
	rec := httptest.NewRecorder()
	handler.ImportPreview(rec, importPreviewRequest([]byte(`{not-json`), "74519f22-ff9b-4a8b-8fff-a958c689682f"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.Bytes(), "VALIDATION_ERROR")
}

func TestAdminImportPreviewMissingSchemaVersion(t *testing.T) {
	body := []byte(`{"template":{"entity_type":"TRANSPORT_ORDER","code":"x","name":"X","sections":[{"code":"s","title":"S","fields":[{"code":"f","label":"F","field_type":"TEXT"}]}]}}`)
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(&stubAdminFormTemplateRepo{}))
	rec := httptest.NewRecorder()
	handler.ImportPreview(rec, importPreviewRequest(body, "74519f22-ff9b-4a8b-8fff-a958c689682f"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	assertErrorCode(t, rec.Body.Bytes(), "VALIDATION_ERROR")
}

func TestAdminImportPreviewMissingTemplate(t *testing.T) {
	body := []byte(`{"schema_version":"lowcode.template.export.v1"}`)
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(&stubAdminFormTemplateRepo{}))
	rec := httptest.NewRecorder()
	handler.ImportPreview(rec, importPreviewRequest(body, "74519f22-ff9b-4a8b-8fff-a958c689682f"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	assertErrorCode(t, rec.Body.Bytes(), "VALIDATION_ERROR")
}

func TestAdminImportPreviewMissingEntityType(t *testing.T) {
	body := []byte(`{
		"schema_version":"lowcode.template.export.v1",
		"template":{"code":"x","name":"X","sections":[{"code":"s","title":"S","fields":[{"code":"f","label":"F","field_type":"TEXT"}]}]}
	}`)
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(&stubAdminFormTemplateRepo{}))
	rec := httptest.NewRecorder()
	handler.ImportPreview(rec, importPreviewRequest(body, "74519f22-ff9b-4a8b-8fff-a958c689682f"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.Bytes(), "VALIDATION_ERROR")
}

func TestAdminImportPreviewMissingCode(t *testing.T) {
	body := []byte(`{
		"schema_version":"lowcode.template.export.v1",
		"template":{"entity_type":"TRANSPORT_ORDER","name":"X","sections":[{"code":"s","title":"S","fields":[{"code":"f","label":"F","field_type":"TEXT"}]}]}
	}`)
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(&stubAdminFormTemplateRepo{}))
	rec := httptest.NewRecorder()
	handler.ImportPreview(rec, importPreviewRequest(body, "74519f22-ff9b-4a8b-8fff-a958c689682f"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	assertErrorCode(t, rec.Body.Bytes(), "VALIDATION_ERROR")
}

func TestAdminImportPreviewDuplicateFieldCode(t *testing.T) {
	body := []byte(`{
		"schema_version":"lowcode.template.export.v1",
		"template":{
			"entity_type":"TRANSPORT_ORDER",
			"code":"transport_order_default",
			"name":"Default",
			"sections":[{"code":"cargo","title":"Cargo","fields":[
				{"code":"dup","label":"A","field_type":"TEXT"},
				{"code":"dup","label":"B","field_type":"TEXT"}
			]}]
		}
	}`)
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(&stubAdminFormTemplateRepo{}))
	rec := httptest.NewRecorder()
	handler.ImportPreview(rec, importPreviewRequest(body, "74519f22-ff9b-4a8b-8fff-a958c689682f"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.Bytes(), "VALIDATION_ERROR")
}

func TestAdminImportPreviewDuplicateSectionCode(t *testing.T) {
	body := []byte(`{
		"schema_version":"lowcode.template.export.v1",
		"template":{
			"entity_type":"TRANSPORT_ORDER",
			"code":"transport_order_default",
			"name":"Default",
			"sections":[
				{"code":"dup","title":"A","fields":[{"code":"f1","label":"F1","field_type":"TEXT"}]},
				{"code":"dup","title":"B","fields":[{"code":"f2","label":"F2","field_type":"TEXT"}]}
			]
		}
	}`)
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(&stubAdminFormTemplateRepo{}))
	rec := httptest.NewRecorder()
	handler.ImportPreview(rec, importPreviewRequest(body, "74519f22-ff9b-4a8b-8fff-a958c689682f"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.Bytes(), "VALIDATION_ERROR")
}

func TestAdminImportPreviewSQLFragmentRejected(t *testing.T) {
	body := []byte(`{
		"schema_version":"lowcode.template.export.v1",
		"template":{
			"entity_type":"TRANSPORT_ORDER",
			"code":"transport_order_default",
			"name":"Default",
			"sections":[{"code":"cargo","title":"Cargo","fields":[{
				"code":"cargo_class","label":"Class","field_type":"TEXT",
				"validation_rule_json":{"rule":"SELECT * FROM users"}
			}]}]
		}
	}`)
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(&stubAdminFormTemplateRepo{}))
	rec := httptest.NewRecorder()
	handler.ImportPreview(rec, importPreviewRequest(body, "74519f22-ff9b-4a8b-8fff-a958c689682f"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.Bytes(), "VALIDATION_ERROR")
}

func TestAdminImportPreviewPayloadTooLarge(t *testing.T) {
	padding := strings.Repeat("x", domain.MaxImportPayloadBytes)
	body := []byte(`{"schema_version":"lowcode.template.export.v1","template":{"entity_type":"TRANSPORT_ORDER","code":"x","name":"` + padding + `","sections":[]}}`)
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(&stubAdminFormTemplateRepo{}))
	rec := httptest.NewRecorder()
	handler.ImportPreview(rec, importPreviewRequest(body, "74519f22-ff9b-4a8b-8fff-a958c689682f"))
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.Bytes(), "IMPORT_PAYLOAD_TOO_LARGE")
}

func TestAdminImportPreviewDoesNotCallImportExecute(t *testing.T) {
	stub := &stubAdminFormTemplateRepo{listByCodeItems: []domain.FormTemplateSummary{}}
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(stub))
	rec := httptest.NewRecorder()
	handler.ImportPreview(rec, importPreviewRequest(validImportPreviewPayload(), "74519f22-ff9b-4a8b-8fff-a958c689682f"))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if stub.importCallCount != 0 {
		t.Fatalf("import execute must not run during preview, calls=%d", stub.importCallCount)
	}
	if !stub.importPreviewRecorded {
		t.Fatal("expected import preview audit")
	}
}

func TestAdminImportPreviewSourceTenantIgnoredUsesRequestTenant(t *testing.T) {
	requestTenant := uuid.MustParse("74519f22-ff9b-4a8b-8fff-a958c689682f")
	otherTenant := uuid.New()
	stub := &stubAdminFormTemplateRepo{listByCodeItems: []domain.FormTemplateSummary{}}
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(stub))
	body := []byte(`{
		"schema_version":"lowcode.template.export.v1",
		"source":{"tenant_id":"` + otherTenant.String() + `","template_id":"b1111111-1111-4111-8111-111111111102"},
		"template":{
			"entity_type":"TRANSPORT_ORDER",
			"code":"transport_order_custom_edge",
			"name":"Edge",
			"sections":[{"code":"cargo","title":"Cargo","fields":[{"code":"f","label":"F","field_type":"TEXT"}]}]
		}
	}`)
	rec := httptest.NewRecorder()
	handler.Import(rec, importExecuteRequest(body, requestTenant.String()))
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	if stub.lastImportInput.TenantID != requestTenant {
		t.Fatalf("expected request tenant %s, got %s", requestTenant, stub.lastImportInput.TenantID)
	}
}

func TestAdminImportValidationErrorNoRepoWrite(t *testing.T) {
	stub := &stubAdminFormTemplateRepo{}
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(stub))
	body := []byte(`{"schema_version":"lowcode.template.export.v1","template":{"entity_type":"BAD","code":"x","name":"X","sections":[{"code":"s","title":"S","fields":[{"code":"f","label":"F","field_type":"TEXT"}]}]}}`)
	rec := httptest.NewRecorder()
	handler.Import(rec, importExecuteRequest(body, "74519f22-ff9b-4a8b-8fff-a958c689682f"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if stub.importCallCount != 0 {
		t.Fatal("expected no import repo write on validation error")
	}
}

func TestAdminImportFailIfExistsNoRepoWrite(t *testing.T) {
	stub := &stubAdminFormTemplateRepo{
		listByCodeItems: []domain.FormTemplateSummary{{
			Status: domain.PublishedStatus, Version: 1, Code: "transport_order_default", EntityType: "TRANSPORT_ORDER",
		}},
	}
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(stub))
	body := []byte(`{
		"schema_version":"lowcode.template.export.v1",
		"conflict_strategy":"FAIL_IF_EXISTS",
		"template":{
			"entity_type":"TRANSPORT_ORDER",
			"code":"transport_order_default",
			"name":"Default",
			"sections":[{"code":"cargo","title":"Cargo","fields":[{"code":"cargo_class","label":"Class","field_type":"SELECT"}]}]
		}
	}`)
	rec := httptest.NewRecorder()
	handler.Import(rec, importExecuteRequest(body, "74519f22-ff9b-4a8b-8fff-a958c689682f"))
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
	if stub.importCallCount != 0 {
		t.Fatal("expected no import repo write on conflict")
	}
}

func TestAdminImportReplaceExistingDraftWithoutDraftRejected(t *testing.T) {
	stub := &stubAdminFormTemplateRepo{
		listByCodeItems: []domain.FormTemplateSummary{{
			Status: domain.PublishedStatus, Version: 1, Code: "transport_order_default", EntityType: "TRANSPORT_ORDER",
		}},
	}
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(stub))
	body := []byte(`{
		"schema_version":"lowcode.template.export.v1",
		"conflict_strategy":"REPLACE_EXISTING_DRAFT",
		"template":{
			"entity_type":"TRANSPORT_ORDER",
			"code":"transport_order_default",
			"name":"Default",
			"sections":[{"code":"cargo","title":"Cargo","fields":[{"code":"cargo_class","label":"Class","field_type":"SELECT"}]}]
		}
	}`)
	rec := httptest.NewRecorder()
	handler.Import(rec, importExecuteRequest(body, "74519f22-ff9b-4a8b-8fff-a958c689682f"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.Bytes(), "FORM_TEMPLATE_NOT_DRAFT")
	if stub.importCallCount != 0 {
		t.Fatal("expected no import repo write when no draft exists")
	}
}

func TestAdminImportPreviewRejectsUnknownTopLevelKey(t *testing.T) {
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(&stubAdminFormTemplateRepo{}))
	body := []byte(`{
		"schema_version":"lowcode.template.export.v1",
		"custom_values":[],
		"template":{
			"entity_type":"TRANSPORT_ORDER",
			"code":"transport_order_default",
			"name":"Default",
			"sections":[{"code":"cargo","title":"Cargo","fields":[{"code":"f","label":"F","field_type":"TEXT"}]}]
		}
	}`)
	rec := httptest.NewRecorder()
	handler.ImportPreview(rec, importPreviewRequest(body, "74519f22-ff9b-4a8b-8fff-a958c689682f"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.Bytes(), "VALIDATION_ERROR")
}

func TestAdminImportPreviewChecksumMismatchWarning(t *testing.T) {
	stub := &stubAdminFormTemplateRepo{listByCodeItems: []domain.FormTemplateSummary{}}
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(stub))
	body := []byte(`{
		"schema_version":"lowcode.template.export.v1",
		"conflict_strategy":"NEW_VERSION",
		"metadata":{"checksum":"0000000000000000000000000000000000000000000000000000000000000000"},
		"template":{
			"entity_type":"TRANSPORT_ORDER",
			"code":"transport_order_default",
			"name":"Default",
			"sections":[{"code":"cargo","title":"Cargo","fields":[{"code":"cargo_class","label":"Class","field_type":"SELECT"}]}]
		}
	}`)
	rec := httptest.NewRecorder()
	handler.ImportPreview(rec, importPreviewRequest(body, "74519f22-ff9b-4a8b-8fff-a958c689682f"))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload domain.TemplateImportPreviewResult
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.Status != domain.ImportPreviewStatusWarning {
		t.Fatalf("status = %q", payload.Status)
	}
	found := false
	for _, w := range payload.Warnings {
		if strings.Contains(w, "checksum mismatch") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected checksum warning, got %+v", payload.Warnings)
	}
}

func TestAdminImportNewVersionDoesNotReplacePublished(t *testing.T) {
	tenantID := uuid.MustParse("74519f22-ff9b-4a8b-8fff-a958c689682f")
	publishedID := uuid.MustParse("b1111111-1111-4111-8111-111111111102")
	stub := &stubAdminFormTemplateRepo{
		listByCodeItems: []domain.FormTemplateSummary{{
			ID: publishedID, TenantID: tenantID, EntityType: "TRANSPORT_ORDER",
			Code: "transport_order_default", Status: domain.PublishedStatus, Version: 1,
		}},
		getDetail: &domain.FormTemplateDetail{
			ID: publishedID, TenantID: tenantID, EntityType: "TRANSPORT_ORDER", Code: "transport_order_default",
			Status: domain.PublishedStatus, Version: 1,
			Sections: []domain.FormSection{{
				Code: "cargo", Title: "Cargo",
				Fields: []domain.FormField{{Code: "cargo_class", Label: "Cargo class", FieldType: "SELECT"}},
			}},
		},
	}
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(stub))
	rec := httptest.NewRecorder()
	handler.Import(rec, importExecuteRequest(validImportPreviewPayload(), tenantID.String()))
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	if stub.lastImportInput.ReplaceDraftID != nil {
		t.Fatal("NEW_VERSION must not replace published template")
	}
	var payload domain.TemplateImportExecuteResult
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.Status != domain.DraftStatus {
		t.Fatalf("status = %q", payload.Status)
	}
}

func TestAdminImportNewVersionRepeatedCreatesSeparateDraftRows(t *testing.T) {
	stub := &stubAdminFormTemplateRepo{listByCodeItems: []domain.FormTemplateSummary{}}
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(stub))
	tenant := "74519f22-ff9b-4a8b-8fff-a958c689682f"
	body := validImportPreviewPayload()

	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		handler.Import(rec, importExecuteRequest(body, tenant))
		if rec.Code != http.StatusCreated {
			t.Fatalf("import %d: expected 201, got %d", i+1, rec.Code)
		}
	}
	if stub.importCallCount != 2 {
		t.Fatalf("expected 2 import calls, got %d", stub.importCallCount)
	}
}

func TestAdminImportPreservesFieldCodesInDraftInput(t *testing.T) {
	stub := &stubAdminFormTemplateRepo{listByCodeItems: []domain.FormTemplateSummary{}}
	handler := NewAdminFormTemplateHandler(service.NewAdminFormTemplateService(stub))
	rec := httptest.NewRecorder()
	handler.Import(rec, importExecuteRequest(validImportPreviewPayload(), "74519f22-ff9b-4a8b-8fff-a958c689682f"))
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	if len(stub.lastImportInput.DraftInput.Sections) != 1 {
		t.Fatalf("sections = %d", len(stub.lastImportInput.DraftInput.Sections))
	}
	if stub.lastImportInput.DraftInput.Sections[0].Fields[0].Code != "cargo_class" {
		t.Fatalf("field code = %q", stub.lastImportInput.DraftInput.Sections[0].Fields[0].Code)
	}
}
