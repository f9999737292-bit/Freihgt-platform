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

func TestAdminMigrateToActiveTenantRequired(t *testing.T) {
	handler := NewAdminCustomFieldValueHandler(service.NewCustomFieldValueService(
		&previewMigrationFormTemplateReader{},
		&previewMigrationCustomFieldValueStore{},
	))
	body := `{"entity_type":"TRANSPORT_ORDER","entity_id":"` + uuid.New().String() + `","template_code":"transport_order_default"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/custom-field-values/migrate-to-active", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.MigrateToActive(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.Bytes(), "TENANT_REQUIRED")
}

func TestAdminMigrateToActiveInvalidEntityID(t *testing.T) {
	handler := NewAdminCustomFieldValueHandler(service.NewCustomFieldValueService(
		&previewMigrationFormTemplateReader{},
		&previewMigrationCustomFieldValueStore{},
	))
	body := `{"entity_type":"TRANSPORT_ORDER","entity_id":"not-a-uuid","template_code":"transport_order_default"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/custom-field-values/migrate-to-active", strings.NewReader(body))
	req.Header.Set(tenantHeader, "74519f22-ff9b-4a8b-8fff-a958c689682f")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.MigrateToActive(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.Bytes(), "ENTITY_ID_INVALID")
}

func TestAdminMigrationPreviewInvalidEntityID(t *testing.T) {
	handler := NewAdminCustomFieldValueHandler(service.NewCustomFieldValueService(
		&previewMigrationFormTemplateReader{},
		&previewMigrationCustomFieldValueStore{},
	))
	body := `{"entity_type":"TRANSPORT_ORDER","template_code":"transport_order_default","entity_ids":["bad-id"]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/custom-field-values/migration-preview", strings.NewReader(body))
	req.Header.Set(tenantHeader, "74519f22-ff9b-4a8b-8fff-a958c689682f")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.MigrationPreview(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.Bytes(), "ENTITY_ID_INVALID")
}

func TestAdminMigrateToActiveBlockedReturnsConflictWithPreview(t *testing.T) {
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
					"amount": {Code: "amount", FieldType: "MONEY"},
				},
			},
		},
		&previewMigrationCustomFieldValueStore{
			items: []domain.CustomFieldValue{{FieldCode: "amount", ValueJSON: json.RawMessage(`42`)}},
		},
	))

	body := `{"entity_type":"TRANSPORT_ORDER","entity_id":"` + entityID.String() + `","template_code":"transport_order_default"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/custom-field-values/migrate-to-active", strings.NewReader(body))
	req.Header.Set(tenantHeader, tenantID.String())
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.MigrateToActive(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Error struct {
			Code    string                 `json:"code"`
			Preview map[string]interface{} `json:"preview"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Error.Code != "MIGRATION_BLOCKED" {
		t.Fatalf("expected MIGRATION_BLOCKED, got %s", payload.Error.Code)
	}
	if payload.Error.Preview == nil {
		t.Fatal("expected preview payload in blocked error")
	}
}

func TestAdminMigrationPreviewInvalidEntityType(t *testing.T) {
	templateID := uuid.New()
	tenantID := uuid.MustParse("74519f22-ff9b-4a8b-8fff-a958c689682f")
	handler := NewAdminCustomFieldValueHandler(service.NewCustomFieldValueService(
		&previewMigrationFormTemplateReader{
			activeItems: []domain.FormTemplateSummary{{ID: templateID, Code: "transport_order_default", Version: 1}},
			target: &domain.PublishedTemplateContext{
				ID: templateID, TenantID: tenantID, EntityType: "TRANSPORT_ORDER",
				Code: "transport_order_default", Version: 1, Status: domain.PublishedStatus,
			},
		},
		&previewMigrationCustomFieldValueStore{},
	))
	body := `{"entity_type":"INVALID","template_code":"transport_order_default","entity_ids":["` + uuid.New().String() + `"]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/custom-field-values/migration-preview", strings.NewReader(body))
	req.Header.Set(tenantHeader, tenantID.String())
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.MigrationPreview(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.Bytes(), "ENTITY_TYPE_INVALID")
}

func TestAdminMigrationPreviewInvalidTargetTemplateID(t *testing.T) {
	tenantID := uuid.MustParse("74519f22-ff9b-4a8b-8fff-a958c689682f")
	handler := NewAdminCustomFieldValueHandler(service.NewCustomFieldValueService(
		&previewMigrationFormTemplateReader{},
		&previewMigrationCustomFieldValueStore{},
	))
	body := `{"entity_type":"TRANSPORT_ORDER","target_template_id":"not-a-uuid","entity_ids":["` + uuid.New().String() + `"]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/custom-field-values/migration-preview", strings.NewReader(body))
	req.Header.Set(tenantHeader, tenantID.String())
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.MigrationPreview(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.Bytes(), "VALIDATION_ERROR")
}
