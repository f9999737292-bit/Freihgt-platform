package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/low-code-service/internal/platform/errors"
	"github.com/freight-platform/low-code-service/internal/domain"
	"github.com/freight-platform/low-code-service/internal/repository"
	"github.com/freight-platform/low-code-service/internal/service"
)

func TestAdminBatchMigrationPreviewReturnsBatchSummary(t *testing.T) {
	templateID := uuid.New()
	tenantID := uuid.MustParse("74519f22-ff9b-4a8b-8fff-a958c689682f")
	safeEntityID := uuid.New()
	warningEntityID := uuid.New()
	blockedEntityID := uuid.New()

	store := &batchPreviewCustomFieldValueStore{
		itemsByEntity: map[uuid.UUID][]domain.CustomFieldValue{
			safeEntityID: {
				{FieldCode: "cargo_class", ValueJSON: json.RawMessage(`"GENERAL"`)},
			},
			warningEntityID: {
				{FieldCode: "deprecated_field", ValueJSON: json.RawMessage(`"legacy"`)},
			},
			blockedEntityID: {
				{FieldCode: "amount", ValueJSON: json.RawMessage(`42`)},
			},
		},
	}

	handler := NewAdminCustomFieldValueHandler(service.NewCustomFieldValueService(
		&previewMigrationFormTemplateReader{
			activeItems: []domain.FormTemplateSummary{
				{ID: templateID, Code: "transport_order_default", Version: 1},
			},
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
		safeEntityID.String() + `","` + warningEntityID.String() + `","` + blockedEntityID.String() + `"]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/custom-field-values/batch-migration-preview", strings.NewReader(body))
	req.Header.Set(tenantHeader, tenantID.String())
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.BatchMigrationPreview(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if store.writeCalled {
		t.Fatal("batch preview must not write")
	}

	var payload batchMigrationPreviewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.TemplateCode != "transport_order_default" {
		t.Fatalf("unexpected template_code: %s", payload.TemplateCode)
	}
	if payload.Summary.Total != 3 || payload.Summary.Safe != 1 || payload.Summary.Warnings != 1 || payload.Summary.Blocked != 1 {
		t.Fatalf("unexpected summary: %+v", payload.Summary)
	}
	if len(payload.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(payload.Items))
	}
}

func TestAdminBatchMigrationPreviewInvalidEntityType(t *testing.T) {
	handler := NewAdminCustomFieldValueHandler(service.NewCustomFieldValueService(
		&previewMigrationFormTemplateReader{},
		&batchPreviewCustomFieldValueStore{},
	))
	body := `{"entity_type":"INVALID","template_code":"transport_order_default","entity_ids":["` + uuid.New().String() + `"]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/custom-field-values/batch-migration-preview", strings.NewReader(body))
	req.Header.Set(tenantHeader, "74519f22-ff9b-4a8b-8fff-a958c689682f")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.BatchMigrationPreview(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.Bytes(), "ENTITY_TYPE_INVALID")
}

func TestAdminBatchMigrationPreviewInvalidEntityID(t *testing.T) {
	handler := NewAdminCustomFieldValueHandler(service.NewCustomFieldValueService(
		&previewMigrationFormTemplateReader{},
		&batchPreviewCustomFieldValueStore{},
	))
	body := `{"entity_type":"TRANSPORT_ORDER","template_code":"transport_order_default","entity_ids":["bad-id"]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/custom-field-values/batch-migration-preview", strings.NewReader(body))
	req.Header.Set(tenantHeader, "74519f22-ff9b-4a8b-8fff-a958c689682f")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.BatchMigrationPreview(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.Bytes(), "ENTITY_ID_INVALID")
}

func TestAdminBatchMigrationPreviewTargetTemplateNotPublished(t *testing.T) {
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

	body := `{"entity_type":"TRANSPORT_ORDER","target_template_id":"` + targetID.String() + `","entity_ids":["` + entityID.String() + `"]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/custom-field-values/batch-migration-preview", strings.NewReader(body))
	req.Header.Set(tenantHeader, tenantID.String())
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.BatchMigrationPreview(rec, req)
	if rec.Code == http.StatusOK {
		t.Fatal("expected not published error")
	}
	assertErrorCode(t, rec.Body.Bytes(), "FORM_TEMPLATE_NOT_PUBLISHED")
}

func TestAdminBatchMigrationPreviewTenantRequired(t *testing.T) {
	handler := NewAdminCustomFieldValueHandler(service.NewCustomFieldValueService(
		&previewMigrationFormTemplateReader{},
		&batchPreviewCustomFieldValueStore{},
	))
	body := `{"entity_type":"TRANSPORT_ORDER","template_code":"transport_order_default","entity_ids":["` + uuid.New().String() + `"]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/custom-field-values/batch-migration-preview", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.BatchMigrationPreview(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	assertErrorCode(t, rec.Body.Bytes(), "TENANT_REQUIRED")
}

func TestAdminBatchMigrationPreviewEmptyEntityIDsRejected(t *testing.T) {
	handler := NewAdminCustomFieldValueHandler(service.NewCustomFieldValueService(
		&previewMigrationFormTemplateReader{},
		&batchPreviewCustomFieldValueStore{},
	))
	body := `{"entity_type":"TRANSPORT_ORDER","template_code":"transport_order_default","entity_ids":[]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/custom-field-values/batch-migration-preview", strings.NewReader(body))
	req.Header.Set(tenantHeader, "74519f22-ff9b-4a8b-8fff-a958c689682f")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.BatchMigrationPreview(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminBatchMigrationPreviewTooManyEntityIDsRejected(t *testing.T) {
	handler := NewAdminCustomFieldValueHandler(service.NewCustomFieldValueService(
		&previewMigrationFormTemplateReader{},
		&batchPreviewCustomFieldValueStore{},
	))
	ids := make([]string, domain.MaxMigrationPreviewEntityCount+1)
	for i := range ids {
		ids[i] = uuid.New().String()
	}
	bodyBytes, _ := json.Marshal(map[string]any{
		"entity_type":   "TRANSPORT_ORDER",
		"template_code": "transport_order_default",
		"entity_ids":    ids,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/custom-field-values/batch-migration-preview", strings.NewReader(string(bodyBytes)))
	req.Header.Set(tenantHeader, "74519f22-ff9b-4a8b-8fff-a958c689682f")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.BatchMigrationPreview(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminBatchMigrationPreviewTenantIsolation(t *testing.T) {
	tenantID := uuid.MustParse("74519f22-ff9b-4a8b-8fff-a958c689682f")
	otherTenant := uuid.New()
	targetID := uuid.New()
	entityID := uuid.New()

	handler := NewAdminCustomFieldValueHandler(service.NewCustomFieldValueService(
		&previewMigrationFormTemplateReader{
			activeItems: []domain.FormTemplateSummary{{ID: targetID, Code: "transport_order_default", Version: 1}},
			target: &domain.PublishedTemplateContext{
				ID: targetID, TenantID: tenantID, EntityType: "TRANSPORT_ORDER",
				Code: "transport_order_default", Version: 1, Status: domain.PublishedStatus,
				Fields: map[string]domain.FieldDefinition{
					"note": {Code: "note", FieldType: "TEXT"},
				},
			},
		},
		&batchPreviewCustomFieldValueStore{
			itemsByEntity: map[uuid.UUID][]domain.CustomFieldValue{
				entityID: {{FieldCode: "note", ValueJSON: json.RawMessage(`"x"`)}},
			},
		},
	))

	body := `{"entity_type":"TRANSPORT_ORDER","template_code":"transport_order_default","entity_ids":["` + entityID.String() + `"]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/custom-field-values/batch-migration-preview", strings.NewReader(body))
	req.Header.Set(tenantHeader, otherTenant.String())
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.BatchMigrationPreview(rec, req)
	if rec.Code == http.StatusOK {
		t.Fatal("expected tenant mismatch/not found error")
	}
}

func TestAdminBatchMigrationPreviewTargetTemplateTenantMismatch(t *testing.T) {
	tenantID := uuid.MustParse("74519f22-ff9b-4a8b-8fff-a958c689682f")
	targetID := uuid.New()
	entityID := uuid.New()

	handler := NewAdminCustomFieldValueHandler(service.NewCustomFieldValueService(
		&batchPreviewTargetTemplateReader{
			target: &domain.PublishedTemplateContext{
				ID: targetID, TenantID: uuid.New(), EntityType: "TRANSPORT_ORDER",
				Code: "transport_order_default", Version: 1, Status: domain.PublishedStatus,
			},
		},
		&batchPreviewCustomFieldValueStore{},
	))

	body := `{"entity_type":"TRANSPORT_ORDER","target_template_id":"` + targetID.String() + `","entity_ids":["` + entityID.String() + `"]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/custom-field-values/batch-migration-preview", strings.NewReader(body))
	req.Header.Set(tenantHeader, tenantID.String())
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.BatchMigrationPreview(rec, req)
	if rec.Code == http.StatusOK {
		t.Fatal("expected tenant mismatch error")
	}
}

type batchPreviewCustomFieldValueStore struct {
	itemsByEntity map[uuid.UUID][]domain.CustomFieldValue
	writeCalled   bool
}

func (s *batchPreviewCustomFieldValueStore) ListByEntity(ctx context.Context, tenantID uuid.UUID, entityType string, entityID uuid.UUID) ([]domain.CustomFieldValue, error) {
	return s.itemsByEntity[entityID], nil
}

func (s *batchPreviewCustomFieldValueStore) UpsertBatch(ctx context.Context, input domain.UpsertCustomFieldValuesInput, values []repository.ResolvedCustomFieldValue) (int, error) {
	s.writeCalled = true
	return 0, nil
}

func (s *batchPreviewCustomFieldValueStore) ReplaceFieldCodesBatch(
	ctx context.Context,
	input domain.UpsertCustomFieldValuesInput,
	fieldCodes []string,
	values []repository.ResolvedCustomFieldValue,
) (int, error) {
	s.writeCalled = true
	return 0, nil
}

type batchPreviewTargetTemplateReader struct {
	target *domain.PublishedTemplateContext
}

func (s *batchPreviewTargetTemplateReader) GetPublishedTemplateContext(ctx context.Context, tenantID uuid.UUID, templateID uuid.UUID) (*domain.PublishedTemplateContext, error) {
	if s.target != nil && s.target.ID == templateID {
		if s.target.TenantID != tenantID {
			return nil, apperrors.TenantMismatch()
		}
		if s.target.Status != domain.PublishedStatus {
			return nil, apperrors.FormTemplateNotPublished()
		}
		return s.target, nil
	}
	return nil, apperrors.FormTemplateNotFound()
}

func (s *batchPreviewTargetTemplateReader) ListActivePublished(ctx context.Context, tenantID uuid.UUID, entityType string, code string) ([]domain.FormTemplateSummary, error) {
	return nil, nil
}
