package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/low-code-service/internal/domain"
	"github.com/freight-platform/low-code-service/internal/service"
)

func TestAdminBatchMigrationPreviewDuplicateEntityIDs(t *testing.T) {
	templateID := uuid.New()
	tenantID := uuid.MustParse("74519f22-ff9b-4a8b-8fff-a958c689682f")
	entityID := uuid.New()

	handler := NewAdminCustomFieldValueHandler(service.NewCustomFieldValueService(
		&previewMigrationFormTemplateReader{
			activeItems: []domain.FormTemplateSummary{{ID: templateID, Code: "transport_order_default", Version: 1}},
			target: &domain.PublishedTemplateContext{
				ID: templateID, TenantID: tenantID, EntityType: "TRANSPORT_ORDER",
				Code: "transport_order_default", Version: 1, Status: domain.PublishedStatus,
				Fields: map[string]domain.FieldDefinition{
					"cargo_class": {
						Code: "cargo_class", FieldType: "SELECT",
						OptionsJSON: json.RawMessage(`{"options":[{"value":"GENERAL","label":"General"}]}`),
					},
				},
			},
		},
		&batchPreviewCustomFieldValueStore{
			itemsByEntity: map[uuid.UUID][]domain.CustomFieldValue{
				entityID: {{FieldCode: "cargo_class", ValueJSON: json.RawMessage(`"GENERAL"`)}},
			},
		},
	))

	body := `{"entity_type":"TRANSPORT_ORDER","template_code":"transport_order_default","entity_ids":["` +
		entityID.String() + `","` + entityID.String() + `"]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/custom-field-values/batch-migration-preview", strings.NewReader(body))
	req.Header.Set(tenantHeader, tenantID.String())
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.BatchMigrationPreview(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var payload batchMigrationPreviewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Summary.Total != 1 || len(payload.Items) != 1 {
		t.Fatalf("duplicate entity_ids must be deduplicated: summary=%+v items=%d", payload.Summary, len(payload.Items))
	}
}

func TestAdminBatchMigrateToActiveWarningsRequireConfirmationReturns409(t *testing.T) {
	templateID := uuid.New()
	tenantID := uuid.MustParse("74519f22-ff9b-4a8b-8fff-a958c689682f")
	entityID := uuid.New()
	store := &batchExecuteHandlerStore{
		batchPreviewCustomFieldValueStore: batchPreviewCustomFieldValueStore{
			itemsByEntity: map[uuid.UUID][]domain.CustomFieldValue{
				entityID: {
					{FieldCode: "cargo_class", ValueJSON: json.RawMessage(`"GENERAL"`)},
					{FieldCode: "deprecated_field", ValueJSON: json.RawMessage(`"legacy"`)},
				},
			},
		},
	}

	handler := NewAdminCustomFieldValueHandler(service.NewCustomFieldValueService(
		&previewMigrationFormTemplateReader{
			activeItems: []domain.FormTemplateSummary{{ID: templateID, Code: "transport_order_default", Version: 1}},
			target: &domain.PublishedTemplateContext{
				ID: templateID, TenantID: tenantID, EntityType: "TRANSPORT_ORDER",
				Code: "transport_order_default", Version: 1, Status: domain.PublishedStatus,
				Fields: map[string]domain.FieldDefinition{
					"cargo_class": {Code: "cargo_class", FieldType: "SELECT", OptionsJSON: json.RawMessage(`{"options":[{"value":"GENERAL"}]}`)},
				},
			},
		},
		store,
	))

	body := `{"entity_type":"TRANSPORT_ORDER","template_code":"transport_order_default","entity_ids":["` +
		entityID.String() + `"],"allow_warnings":false,"skip_blocked":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/custom-field-values/batch-migrate-to-active", strings.NewReader(body))
	req.Header.Set(tenantHeader, tenantID.String())
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.BatchMigrateToActive(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.Bytes(), "BATCH_MIGRATION_WARNINGS_REQUIRE_CONFIRMATION")
	if store.writeCalled {
		t.Fatal("must not write when warning-only batch requires confirmation")
	}
}

func TestAdminBatchMigrateToActiveAuditMetadataIncludesTemplateCodeAndSkipBlocked(t *testing.T) {
	templateID := uuid.New()
	tenantID := uuid.MustParse("74519f22-ff9b-4a8b-8fff-a958c689682f")
	entityID := uuid.New()
	store := &batchExecuteHandlerStore{
		batchPreviewCustomFieldValueStore: batchPreviewCustomFieldValueStore{
			itemsByEntity: map[uuid.UUID][]domain.CustomFieldValue{
				entityID: {{FieldCode: "cargo_class", ValueJSON: json.RawMessage(`"GENERAL"`)}},
			},
		},
	}

	handler := NewAdminCustomFieldValueHandler(service.NewCustomFieldValueService(
		&previewMigrationFormTemplateReader{
			activeItems: []domain.FormTemplateSummary{{ID: templateID, Code: "transport_order_default", Version: 1}},
			target: &domain.PublishedTemplateContext{
				ID: templateID, TenantID: tenantID, EntityType: "TRANSPORT_ORDER",
				Code: "transport_order_default", Version: 1, Status: domain.PublishedStatus,
				Fields: map[string]domain.FieldDefinition{
					"cargo_class": {
						Code: "cargo_class", FieldType: "SELECT",
						OptionsJSON: json.RawMessage(`{"options":[{"value":"GENERAL","label":"General"}]}`),
					},
				},
			},
		},
		store,
	))

	body := `{"entity_type":"TRANSPORT_ORDER","template_code":"transport_order_default","entity_ids":["` +
		entityID.String() + `"],"allow_warnings":true,"skip_blocked":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/custom-field-values/batch-migrate-to-active", strings.NewReader(body))
	req.Header.Set(tenantHeader, tenantID.String())
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.BatchMigrateToActive(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	audit := store.lastInput.MigrationAudit
	if audit == nil {
		t.Fatal("expected migration audit metadata")
	}
	if audit.TemplateCode != "transport_order_default" {
		t.Fatalf("expected template_code in audit metadata, got %q", audit.TemplateCode)
	}
	if !audit.SkipBlocked {
		t.Fatal("expected skip_blocked=true in audit metadata")
	}
}

func TestAdminBatchMigrateToActiveTargetTemplateNotPublished(t *testing.T) {
	tenantID := uuid.MustParse("74519f22-ff9b-4a8b-8fff-a958c689682f")
	targetID := uuid.New()
	entityID := uuid.New()

	handler := NewAdminCustomFieldValueHandler(service.NewCustomFieldValueService(
		&batchPreviewTargetTemplateReader{
			target: &domain.PublishedTemplateContext{
				ID: targetID, TenantID: tenantID, EntityType: "TRANSPORT_ORDER",
				Code: "transport_order_default", Version: 1, Status: domain.DraftStatus,
			},
		},
		&batchPreviewCustomFieldValueStore{},
	))

	body := `{"entity_type":"TRANSPORT_ORDER","target_template_id":"` + targetID.String() +
		`","entity_ids":["` + entityID.String() + `"],"allow_warnings":true,"skip_blocked":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/custom-field-values/batch-migrate-to-active", strings.NewReader(body))
	req.Header.Set(tenantHeader, tenantID.String())
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.BatchMigrateToActive(rec, req)
	if rec.Code == http.StatusOK {
		t.Fatal("expected not published error")
	}
	assertErrorCode(t, rec.Body.Bytes(), "FORM_TEMPLATE_NOT_PUBLISHED")
}
