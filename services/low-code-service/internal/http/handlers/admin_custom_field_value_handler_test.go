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

func TestAdminMigrateCustomFieldValuesToActive(t *testing.T) {
	templateID := uuid.New()
	cargoFieldID := uuid.New()
	tenantID := uuid.MustParse("74519f22-ff9b-4a8b-8fff-a958c689682f")
	entityID := uuid.New()

	handler := NewAdminCustomFieldValueHandler(service.NewCustomFieldValueService(
		&stubFormTemplateReader{
			activeItems: []domain.FormTemplateSummary{
				{ID: templateID, EntityType: "TRANSPORT_ORDER", Code: "transport_order_default", Status: domain.PublishedStatus, Version: 2},
			},
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
						OptionsJSON: json.RawMessage(`{"options":[{"value":"GENERAL","label":"General"}]}`),
					},
				},
			},
		},
		&migrateCustomFieldValueStore{
			stubCustomFieldValueStore: stubCustomFieldValueStore{},
			listItems: []domain.CustomFieldValue{
				{
					FieldID:   uuid.New(),
					FieldCode: "cargo_class",
					ValueJSON: json.RawMessage(`"GENERAL"`),
				},
			},
		},
	))

	body := `{"entity_type":"TRANSPORT_ORDER","entity_id":"` + entityID.String() + `","code":"transport_order_default"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/low-code/admin/custom-field-values/migrate-to-active", strings.NewReader(body))
	req.Header.Set(tenantHeader, tenantID.String())
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.MigrateToActive(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var payload migrateCustomFieldValuesToActiveResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.MigratedCount != 1 || payload.ActiveTemplateID != templateID.String() {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

type migrateCustomFieldValueStore struct {
	stubCustomFieldValueStore
	listItems []domain.CustomFieldValue
}

func (s *migrateCustomFieldValueStore) ListByEntity(ctx context.Context, tenantID uuid.UUID, entityType string, entityID uuid.UUID) ([]domain.CustomFieldValue, error) {
	return s.listItems, nil
}

func (s *migrateCustomFieldValueStore) ReplaceFieldCodesBatch(
	ctx context.Context,
	input domain.UpsertCustomFieldValuesInput,
	fieldCodes []string,
	values []repository.ResolvedCustomFieldValue,
) (int, error) {
	s.lastInput = input
	return len(values), nil
}
