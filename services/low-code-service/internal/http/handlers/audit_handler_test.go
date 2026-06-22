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
	"github.com/freight-platform/low-code-service/internal/service"
)

type stubAuditStore struct {
	lastFilter domain.ListAuditEventsFilter
	items      []domain.ConfigurationAuditEntry
	err        error
}

func (s *stubAuditStore) List(ctx context.Context, filter domain.ListAuditEventsFilter) ([]domain.ConfigurationAuditEntry, error) {
	s.lastFilter = filter
	return s.items, s.err
}

func TestAuditListTenantRequired(t *testing.T) {
	handler := NewAuditHandler(service.NewAuditService(&stubAuditStore{}))
	req := httptest.NewRequest(http.MethodGet, "/v1/low-code/audit-events", nil)
	rec := httptest.NewRecorder()
	handler.List(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	assertErrorCode(t, rec.Body.Bytes(), "TENANT_REQUIRED")
}

func TestAuditListInvalidEntityID(t *testing.T) {
	handler := NewAuditHandler(service.NewAuditService(&stubAuditStore{}))
	req := httptest.NewRequest(http.MethodGet, "/v1/low-code/audit-events?entity_id=bad", nil)
	req.Header.Set(tenantHeader, "74519f22-ff9b-4a8b-8fff-a958c689682f")
	rec := httptest.NewRecorder()
	handler.List(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	assertErrorCode(t, rec.Body.Bytes(), "ENTITY_ID_INVALID")
}

func TestAuditListLimitMax(t *testing.T) {
	store := &stubAuditStore{items: []domain.ConfigurationAuditEntry{}}
	handler := NewAuditHandler(service.NewAuditService(store))
	req := httptest.NewRequest(http.MethodGet, "/v1/low-code/audit-events?limit=500", nil)
	req.Header.Set(tenantHeader, "74519f22-ff9b-4a8b-8fff-a958c689682f")
	rec := httptest.NewRecorder()
	handler.List(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if store.lastFilter.Limit != 100 {
		t.Fatalf("expected limit 100, got %d", store.lastFilter.Limit)
	}
}

func TestAuditListFiltersByTenantAndEntity(t *testing.T) {
	entityID := uuid.MustParse("2db04b49-665c-469f-bcb1-ffeb1274fedb")
	tenantID := uuid.MustParse("74519f22-ff9b-4a8b-8fff-a958c689682f")
	newJSON, _ := json.Marshal(map[string]any{
		"event_kind":     domain.AuditEventKindCustomFieldValuesUpdated,
		"changed_fields": []string{"internal_cost_center"},
		"values":         map[string]string{"internal_cost_center": "CC-2002"},
	})
	oldJSON, _ := json.Marshal(map[string]any{
		"values": map[string]string{"internal_cost_center": "CC-1001"},
	})

	store := &stubAuditStore{
		items: []domain.ConfigurationAuditEntry{
			{
				ID:           uuid.New(),
				TenantID:     tenantID,
				EntityType:   "TRANSPORT_ORDER",
				EntityID:     entityID,
				Action:       domain.AuditDBActionUpdate,
				OldValueJSON: oldJSON,
				NewValueJSON: newJSON,
				ChangedAt:    time.Now().UTC(),
			},
		},
	}
	handler := NewAuditHandler(service.NewAuditService(store))
	req := httptest.NewRequest(http.MethodGet, "/v1/low-code/audit-events?entity_type=TRANSPORT_ORDER&entity_id="+entityID.String()+"&action=CUSTOM_FIELD_VALUES_UPDATED", nil)
	req.Header.Set(tenantHeader, tenantID.String())
	rec := httptest.NewRecorder()
	handler.List(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if store.lastFilter.TenantID != tenantID {
		t.Fatalf("tenant filter mismatch")
	}
	if store.lastFilter.EntityID == nil || *store.lastFilter.EntityID != entityID {
		t.Fatalf("entity filter mismatch")
	}

	var response listAuditEventsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(response.Items))
	}
	if response.Items[0].Action != domain.AuditEventKindCustomFieldValuesUpdated {
		t.Fatalf("expected mapped action, got %s", response.Items[0].Action)
	}
}

func TestCustomFieldValuePutPassesAuditContext(t *testing.T) {
	store := &stubCustomFieldValueStore{}
	templateID := uuid.New()
	entityID := uuid.New()
	tenantID := uuid.MustParse("74519f22-ff9b-4a8b-8fff-a958c689682f")
	userID := uuid.New()

	handler := NewCustomFieldValueHandler(service.NewCustomFieldValueService(
		&stubFormTemplateReader{
			ctx: &domain.PublishedTemplateContext{
				ID:         templateID,
				TenantID:   tenantID,
				EntityType: "TRANSPORT_ORDER",
				Fields: map[string]domain.FieldDefinition{
					"cargo_class": {ID: uuid.New(), Code: "cargo_class", FieldType: "SELECT", OptionsJSON: json.RawMessage(`{"options":[{"value":"GENERAL","label":"General"}]}`)},
				},
			},
		},
		store,
	))

	body := `{"entity_type":"TRANSPORT_ORDER","entity_id":"` + entityID.String() + `","form_template_id":"` + templateID.String() + `","values":[{"field_code":"cargo_class","value_json":"GENERAL"}]}`
	req := httptest.NewRequest(http.MethodPut, "/v1/low-code/custom-field-values", strings.NewReader(body))
	req.Header.Set(tenantHeader, tenantID.String())
	req.Header.Set(userIDHeader, userID.String())
	req.Header.Set("X-Request-ID", "req-123")
	rec := httptest.NewRecorder()
	handler.Put(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if store.lastInput.Audit.RequestID != "req-123" {
		t.Fatalf("expected request id in audit context")
	}
	if store.lastInput.Audit.ChangedByUserID == nil || *store.lastInput.Audit.ChangedByUserID != userID {
		t.Fatalf("expected user id in audit context")
	}
}
