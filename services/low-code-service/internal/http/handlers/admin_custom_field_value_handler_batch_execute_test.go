package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/low-code-service/internal/domain"
	"github.com/freight-platform/low-code-service/internal/repository"
	"github.com/freight-platform/low-code-service/internal/service"
)

func TestAdminBatchMigrateToActiveTenantRequired(t *testing.T) {
	handler := NewAdminCustomFieldValueHandler(service.NewCustomFieldValueService(
		&previewMigrationFormTemplateReader{},
		&batchPreviewCustomFieldValueStore{},
	))
	body := `{"entity_type":"TRANSPORT_ORDER","template_code":"transport_order_default","entity_ids":["` + uuid.New().String() + `"],"allow_warnings":true,"skip_blocked":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/custom-field-values/batch-migrate-to-active", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.BatchMigrateToActive(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	assertErrorCode(t, rec.Body.Bytes(), "TENANT_REQUIRED")
}

func TestAdminBatchMigrateToActiveInvalidEntityType(t *testing.T) {
	handler := NewAdminCustomFieldValueHandler(service.NewCustomFieldValueService(
		&previewMigrationFormTemplateReader{},
		&batchPreviewCustomFieldValueStore{},
	))
	body := `{"entity_type":"INVALID","template_code":"transport_order_default","entity_ids":["` + uuid.New().String() + `"],"allow_warnings":true,"skip_blocked":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/custom-field-values/batch-migrate-to-active", strings.NewReader(body))
	req.Header.Set(tenantHeader, "74519f22-ff9b-4a8b-8fff-a958c689682f")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.BatchMigrateToActive(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.Bytes(), "ENTITY_TYPE_INVALID")
}

func TestAdminBatchMigrateToActiveInvalidEntityID(t *testing.T) {
	handler := NewAdminCustomFieldValueHandler(service.NewCustomFieldValueService(
		&previewMigrationFormTemplateReader{},
		&batchPreviewCustomFieldValueStore{},
	))
	body := `{"entity_type":"TRANSPORT_ORDER","template_code":"transport_order_default","entity_ids":["bad-id"],"allow_warnings":true,"skip_blocked":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/custom-field-values/batch-migrate-to-active", strings.NewReader(body))
	req.Header.Set(tenantHeader, "74519f22-ff9b-4a8b-8fff-a958c689682f")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.BatchMigrateToActive(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.Bytes(), "ENTITY_ID_INVALID")
}

func TestAdminBatchMigrateToActiveReturnsBatchResponse(t *testing.T) {
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

	body := `{"entity_type":"TRANSPORT_ORDER","template_code":"transport_order_default","entity_ids":["` + entityID.String() + `"],"allow_warnings":true,"skip_blocked":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/custom-field-values/batch-migrate-to-active", strings.NewReader(body))
	req.Header.Set(tenantHeader, tenantID.String())
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.BatchMigrateToActive(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var payload batchMigrateToActiveResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.BatchID == "" || payload.Status != domain.BatchMigrateStatusCompleted {
		t.Fatalf("unexpected batch response: %+v", payload)
	}
	if payload.Summary.Total != 1 || payload.Summary.Migrated != 1 || len(payload.Items) != 1 {
		t.Fatalf("unexpected summary/items: %+v len=%d", payload.Summary, len(payload.Items))
	}
	if store.lastInput.MigrationAudit == nil || store.lastInput.MigrationAudit.BatchID == uuid.Nil {
		t.Fatal("expected batch audit metadata on migrated entity")
	}
}

func TestAdminBatchMigrateToActiveBlockedBeforeWritesWhenSkipBlockedFalse(t *testing.T) {
	templateID := uuid.New()
	tenantID := uuid.MustParse("74519f22-ff9b-4a8b-8fff-a958c689682f")
	safeEntityID := uuid.New()
	blockedEntityID := uuid.New()
	store := &batchExecuteHandlerStore{
		batchPreviewCustomFieldValueStore: batchPreviewCustomFieldValueStore{
			itemsByEntity: map[uuid.UUID][]domain.CustomFieldValue{
				safeEntityID:    {{FieldCode: "cargo_class", ValueJSON: json.RawMessage(`"GENERAL"`)}},
				blockedEntityID: {{FieldCode: "amount", ValueJSON: json.RawMessage(`42`)}},
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
					"amount": {Code: "amount", FieldType: "MONEY"},
				},
			},
		},
		store,
	))

	body := `{"entity_type":"TRANSPORT_ORDER","template_code":"transport_order_default","entity_ids":["` +
		safeEntityID.String() + `","` + blockedEntityID.String() + `"],"allow_warnings":true,"skip_blocked":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/custom-field-values/batch-migrate-to-active", strings.NewReader(body))
	req.Header.Set(tenantHeader, tenantID.String())
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.BatchMigrateToActive(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.Bytes(), "BATCH_MIGRATION_BLOCKED")
	if store.writeCalled {
		t.Fatal("must not write when whole batch blocked before execution")
	}
}

type batchExecuteHandlerStore struct {
	batchPreviewCustomFieldValueStore
	lastInput domain.UpsertCustomFieldValuesInput
}

func (s *batchExecuteHandlerStore) ReplaceFieldCodesBatch(
	ctx context.Context,
	input domain.UpsertCustomFieldValuesInput,
	fieldCodes []string,
	values []repository.ResolvedCustomFieldValue,
) (int, error) {
	s.writeCalled = true
	s.lastInput = input
	return len(values), nil
}
