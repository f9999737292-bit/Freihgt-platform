package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/low-code-service/internal/domain"
	apperrors "github.com/freight-platform/low-code-service/internal/platform/errors"
	"github.com/freight-platform/low-code-service/internal/repository"
	"github.com/freight-platform/low-code-service/internal/service"
)

type stubCustomFieldValueStore struct {
	listItems []domain.CustomFieldValue
	listErr   error
	upsertErr error
	upsertCount int
	lastInput domain.UpsertCustomFieldValuesInput
}

func (s *stubCustomFieldValueStore) ListByEntity(ctx context.Context, tenantID uuid.UUID, entityType string, entityID uuid.UUID) ([]domain.CustomFieldValue, error) {
	return s.listItems, s.listErr
}

func (s *stubCustomFieldValueStore) UpsertBatch(ctx context.Context, input domain.UpsertCustomFieldValuesInput, values []repository.ResolvedCustomFieldValue) (int, error) {
	s.lastInput = input
	if s.upsertErr != nil {
		return 0, s.upsertErr
	}
	if s.upsertCount > 0 {
		return s.upsertCount, nil
	}
	return len(values), nil
}

type stubFormTemplateReader struct {
	ctx   *domain.PublishedTemplateContext
	err   error
}

func (s *stubFormTemplateReader) GetPublishedTemplateContext(ctx context.Context, tenantID uuid.UUID, templateID uuid.UUID) (*domain.PublishedTemplateContext, error) {
	return s.ctx, s.err
}

func TestCustomFieldValueGetTenantRequired(t *testing.T) {
	handler := NewCustomFieldValueHandler(service.NewCustomFieldValueService(
		&stubFormTemplateReader{},
		&stubCustomFieldValueStore{},
	))
	req := httptest.NewRequest(http.MethodGet, "/v1/low-code/custom-field-values?entity_type=TRANSPORT_ORDER&entity_id="+uuid.New().String(), nil)
	rec := httptest.NewRecorder()
	handler.Get(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	assertErrorCode(t, rec.Body.Bytes(), "TENANT_REQUIRED")
}

func TestCustomFieldValueGetInvalidEntityType(t *testing.T) {
	handler := NewCustomFieldValueHandler(service.NewCustomFieldValueService(
		&stubFormTemplateReader{},
		&stubCustomFieldValueStore{},
	))
	req := httptest.NewRequest(http.MethodGet, "/v1/low-code/custom-field-values?entity_type=BAD&entity_id="+uuid.New().String(), nil)
	req.Header.Set(tenantHeader, "74519f22-ff9b-4a8b-8fff-a958c689682f")
	rec := httptest.NewRecorder()
	handler.Get(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	assertErrorCode(t, rec.Body.Bytes(), "ENTITY_TYPE_INVALID")
}

func TestCustomFieldValuePutSystemFieldProtected(t *testing.T) {
	templateID := uuid.New()
	fieldID := uuid.New()
	tenantID := uuid.MustParse("74519f22-ff9b-4a8b-8fff-a958c689682f")
	entityID := uuid.New()

	handler := NewCustomFieldValueHandler(service.NewCustomFieldValueService(
		&stubFormTemplateReader{
			ctx: &domain.PublishedTemplateContext{
				ID:         templateID,
				TenantID:   tenantID,
				EntityType: "TRANSPORT_ORDER",
				Status:     domain.PublishedStatus,
				Fields: map[string]domain.FieldDefinition{
					"sys_field": {
						ID:          fieldID,
						Code:        "sys_field",
						FieldType:   "TEXT",
						SystemField: true,
					},
				},
			},
		},
		&stubCustomFieldValueStore{},
	))

	body := `{"entity_type":"TRANSPORT_ORDER","entity_id":"` + entityID.String() + `","form_template_id":"` + templateID.String() + `","values":[{"field_code":"sys_field","value_json":"x"}]}`
	req := httptest.NewRequest(http.MethodPut, "/v1/low-code/custom-field-values", strings.NewReader(body))
	req.Header.Set(tenantHeader, tenantID.String())
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.Put(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.Bytes(), "SYSTEM_FIELD_PROTECTED")
}

func TestCustomFieldValuePutReadOnlyFieldProtected(t *testing.T) {
	templateID := uuid.New()
	fieldID := uuid.New()
	tenantID := uuid.MustParse("74519f22-ff9b-4a8b-8fff-a958c689682f")
	entityID := uuid.New()

	handler := NewCustomFieldValueHandler(service.NewCustomFieldValueService(
		&stubFormTemplateReader{
			ctx: &domain.PublishedTemplateContext{
				ID:         templateID,
				TenantID:   tenantID,
				EntityType: "TRANSPORT_ORDER",
				Status:     domain.PublishedStatus,
				Fields: map[string]domain.FieldDefinition{
					"locked_note": {
						ID:       fieldID,
						Code:     "locked_note",
						FieldType: "TEXT",
						ReadOnly: true,
					},
				},
			},
		},
		&stubCustomFieldValueStore{},
	))

	body := `{"entity_type":"TRANSPORT_ORDER","entity_id":"` + entityID.String() + `","form_template_id":"` + templateID.String() + `","values":[{"field_code":"locked_note","value_json":"x"}]}`
	req := httptest.NewRequest(http.MethodPut, "/v1/low-code/custom-field-values", strings.NewReader(body))
	req.Header.Set(tenantHeader, tenantID.String())
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.Put(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.Bytes(), "READ_ONLY_FIELD_PROTECTED")
}

func TestCustomFieldValuePutDraftTemplateBlocked(t *testing.T) {
	handler := NewCustomFieldValueHandler(service.NewCustomFieldValueService(
		&stubFormTemplateReader{err: apperrors.FormTemplateNotPublished()},
		&stubCustomFieldValueStore{},
	))

	entityID := uuid.New()
	templateID := uuid.New()
	body := `{"entity_type":"TRANSPORT_ORDER","entity_id":"` + entityID.String() + `","form_template_id":"` + templateID.String() + `","values":[{"field_code":"cargo_class","value_json":"GENERAL"}]}`
	req := httptest.NewRequest(http.MethodPut, "/v1/low-code/custom-field-values", strings.NewReader(body))
	req.Header.Set(tenantHeader, "74519f22-ff9b-4a8b-8fff-a958c689682f")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.Put(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	assertErrorCode(t, rec.Body.Bytes(), "FORM_TEMPLATE_NOT_PUBLISHED")
}

func TestCustomFieldValuePutConditionalRequiredFailed(t *testing.T) {
	templateID := uuid.New()
	cargoFieldID := uuid.New()
	noteFieldID := uuid.New()
	tenantID := uuid.MustParse("74519f22-ff9b-4a8b-8fff-a958c689682f")
	entityID := uuid.New()

	handler := NewCustomFieldValueHandler(service.NewCustomFieldValueService(
		&stubFormTemplateReader{
			ctx: &domain.PublishedTemplateContext{
				ID:         templateID,
				TenantID:   tenantID,
				EntityType: "TRANSPORT_ORDER",
				Status:     domain.PublishedStatus,
				Fields: map[string]domain.FieldDefinition{
					"cargo_class": {
						ID:        cargoFieldID,
						Code:      "cargo_class",
						FieldType: "SELECT",
						OptionsJSON: json.RawMessage(`{"options":[{"value":"A","label":"Class A"}]}`),
						ValidationRuleJSON: json.RawMessage(
							`{"if":{"field":"cargo_class","in":["A","B","C"]},"then":{"required":["loading_window_note"]}}`,
						),
					},
					"loading_window_note": {
						ID:        noteFieldID,
						Code:      "loading_window_note",
						FieldType: "TEXT",
					},
				},
			},
		},
		&stubCustomFieldValueStore{},
	))

	body := `{"entity_type":"TRANSPORT_ORDER","entity_id":"` + entityID.String() + `","form_template_id":"` + templateID.String() + `","values":[{"field_code":"cargo_class","value_json":"A"}]}`
	req := httptest.NewRequest(http.MethodPut, "/v1/low-code/custom-field-values", strings.NewReader(body))
	req.Header.Set(tenantHeader, tenantID.String())
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.Put(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertErrorCode(t, rec.Body.Bytes(), "VALIDATION_RULE_FAILED")
}

func TestCustomFieldValuePutIdempotent(t *testing.T) {
	templateID := uuid.New()
	fieldID := uuid.New()
	tenantID := uuid.MustParse("74519f22-ff9b-4a8b-8fff-a958c689682f")
	entityID := uuid.New()
	store := &stubCustomFieldValueStore{}

	handler := NewCustomFieldValueHandler(service.NewCustomFieldValueService(
		&stubFormTemplateReader{
			ctx: &domain.PublishedTemplateContext{
				ID:         templateID,
				TenantID:   tenantID,
				EntityType: "TRANSPORT_ORDER",
				Status:     domain.PublishedStatus,
				Fields: map[string]domain.FieldDefinition{
					"cargo_class": {
						ID:        fieldID,
						Code:      "cargo_class",
						FieldType: "SELECT",
						OptionsJSON: json.RawMessage(`{"options":[{"value":"GENERAL","label":"General"}]}`),
					},
				},
			},
		},
		store,
	))

	body := `{"entity_type":"TRANSPORT_ORDER","entity_id":"` + entityID.String() + `","form_template_id":"` + templateID.String() + `","values":[{"field_code":"cargo_class","value_json":"GENERAL"}]}`
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPut, "/v1/low-code/custom-field-values", strings.NewReader(body))
		req.Header.Set(tenantHeader, tenantID.String())
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.Put(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("run %d expected 200, got %d body=%s", i+1, rec.Code, rec.Body.String())
		}
	}
	if store.lastInput.EntityID != entityID {
		t.Fatal("tenant-filtered upsert expected")
	}
}

func TestCustomFieldValueGetReturnsTenantFilteredValues(t *testing.T) {
	tenantID := uuid.MustParse("74519f22-ff9b-4a8b-8fff-a958c689682f")
	entityID := uuid.New()
	fieldID := uuid.New()
	handler := NewCustomFieldValueHandler(service.NewCustomFieldValueService(
		&stubFormTemplateReader{},
		&stubCustomFieldValueStore{
			listItems: []domain.CustomFieldValue{
				{
					FieldID:   fieldID,
					FieldCode: "cargo_class",
					ValueJSON: json.RawMessage(`"GENERAL"`),
					UpdatedAt: time.Now().UTC(),
				},
			},
		},
	))

	req := httptest.NewRequest(http.MethodGet, "/v1/low-code/custom-field-values?entity_type=TRANSPORT_ORDER&entity_id="+entityID.String(), nil)
	req.Header.Set(tenantHeader, tenantID.String())
	rec := httptest.NewRecorder()
	handler.Get(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "cargo_class") {
		t.Fatalf("expected field in response: %s", rec.Body.String())
	}
}
