package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/low-code-service/internal/platform/errors"
	"github.com/freight-platform/low-code-service/internal/domain"
	"github.com/freight-platform/low-code-service/internal/repository"
)

type executeFormTemplateReader struct {
	activeItems []domain.FormTemplateSummary
	target      *domain.PublishedTemplateContext
}

func (s *executeFormTemplateReader) GetPublishedTemplateContext(ctx context.Context, tenantID uuid.UUID, templateID uuid.UUID) (*domain.PublishedTemplateContext, error) {
	if s.target != nil && s.target.ID == templateID {
		if s.target.TenantID != tenantID {
			return nil, apperrors.TenantMismatch()
		}
		return s.target, nil
	}
	return nil, errors.New("not found")
}

func (s *executeFormTemplateReader) ListActivePublished(ctx context.Context, tenantID uuid.UUID, entityType string, code string) ([]domain.FormTemplateSummary, error) {
	return s.activeItems, nil
}

type executeCustomFieldValueStore struct {
	items       []domain.CustomFieldValue
	writeCalled bool
	lastInput   domain.UpsertCustomFieldValuesInput
}

func (s *executeCustomFieldValueStore) ListByEntity(ctx context.Context, tenantID uuid.UUID, entityType string, entityID uuid.UUID) ([]domain.CustomFieldValue, error) {
	return s.items, nil
}

func (s *executeCustomFieldValueStore) UpsertBatch(ctx context.Context, input domain.UpsertCustomFieldValuesInput, values []repository.ResolvedCustomFieldValue) (int, error) {
	s.writeCalled = true
	return 0, errors.New("unexpected upsert")
}

func (s *executeCustomFieldValueStore) ReplaceFieldCodesBatch(
	ctx context.Context,
	input domain.UpsertCustomFieldValuesInput,
	fieldCodes []string,
	values []repository.ResolvedCustomFieldValue,
) (int, error) {
	s.writeCalled = true
	s.lastInput = input
	return len(values), nil
}

func TestMigrateToActiveSafeMigrationWrites(t *testing.T) {
	tenantID := uuid.New()
	targetID := uuid.New()
	entityID := uuid.New()
	cargoFieldID := uuid.New()
	store := &executeCustomFieldValueStore{
		items: []domain.CustomFieldValue{
			{FieldCode: "cargo_class", ValueJSON: json.RawMessage(`"GENERAL"`), FormTemplateID: uuid.New()},
		},
	}
	svc := NewCustomFieldValueService(
		&executeFormTemplateReader{
			activeItems: []domain.FormTemplateSummary{{ID: targetID, Code: "transport_order_default", Version: 1}},
			target: &domain.PublishedTemplateContext{
				ID: targetID, TenantID: tenantID, EntityType: "TRANSPORT_ORDER",
				Code: "transport_order_default", Version: 1, Status: domain.PublishedStatus,
				Fields: map[string]domain.FieldDefinition{
					"cargo_class": {
						ID: cargoFieldID, Code: "cargo_class", FieldType: "SELECT",
						OptionsJSON: json.RawMessage(`{"options":[{"value":"GENERAL","label":"General"}]}`),
					},
				},
			},
		},
		store,
	)

	result, err := svc.MigrateToActiveTemplate(context.Background(), domain.MigrateCustomFieldValuesToActiveInput{
		TenantID: tenantID, EntityType: "TRANSPORT_ORDER", EntityID: entityID,
		TemplateCode: "transport_order_default",
	})
	if err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	if !store.writeCalled || result.MigratedCount != 1 || result.Status != "migrated" {
		t.Fatalf("unexpected migrate result: %+v write=%v", result, store.writeCalled)
	}
	if store.lastInput.MigrationAudit == nil {
		t.Fatal("expected migration audit payload")
	}
}

func TestMigrateToActiveWarningBlockedWithoutAllowWarnings(t *testing.T) {
	tenantID := uuid.New()
	targetID := uuid.New()
	entityID := uuid.New()
	cargoFieldID := uuid.New()
	store := &executeCustomFieldValueStore{
		items: []domain.CustomFieldValue{
			{FieldCode: "cargo_class", ValueJSON: json.RawMessage(`"GENERAL"`)},
			{FieldCode: "deprecated_field", ValueJSON: json.RawMessage(`"legacy"`)},
		},
	}
	svc := NewCustomFieldValueService(
		&executeFormTemplateReader{
			activeItems: []domain.FormTemplateSummary{{ID: targetID, Code: "transport_order_default", Version: 1}},
			target: &domain.PublishedTemplateContext{
				ID: targetID, TenantID: tenantID, EntityType: "TRANSPORT_ORDER",
				Code: "transport_order_default", Version: 1, Status: domain.PublishedStatus,
				Fields: map[string]domain.FieldDefinition{
					"cargo_class": {
						ID: cargoFieldID, Code: "cargo_class", FieldType: "SELECT",
						OptionsJSON: json.RawMessage(`{"options":[{"value":"GENERAL","label":"General"}]}`),
					},
				},
			},
		},
		store,
	)

	_, err := svc.MigrateToActiveTemplate(context.Background(), domain.MigrateCustomFieldValuesToActiveInput{
		TenantID: tenantID, EntityType: "TRANSPORT_ORDER", EntityID: entityID,
		TemplateCode: "transport_order_default", AllowWarnings: false,
	})
	if err == nil {
		t.Fatal("expected warning confirmation error")
	}
	appErr, ok := err.(*apperrors.AppError)
	if !ok || appErr.Code != apperrors.CodeMigrationWarningsRequireConfirmation {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.writeCalled {
		t.Fatal("must not write when warnings blocked")
	}
}

func TestMigrateToActiveWarningMigratesWithAllowWarnings(t *testing.T) {
	tenantID := uuid.New()
	targetID := uuid.New()
	entityID := uuid.New()
	cargoFieldID := uuid.New()
	store := &executeCustomFieldValueStore{
		items: []domain.CustomFieldValue{
			{FieldCode: "cargo_class", ValueJSON: json.RawMessage(`"GENERAL"`)},
			{FieldCode: "deprecated_field", ValueJSON: json.RawMessage(`"legacy"`)},
		},
	}
	svc := NewCustomFieldValueService(
		&executeFormTemplateReader{
			activeItems: []domain.FormTemplateSummary{{ID: targetID, Code: "transport_order_default", Version: 1}},
			target: &domain.PublishedTemplateContext{
				ID: targetID, TenantID: tenantID, EntityType: "TRANSPORT_ORDER",
				Code: "transport_order_default", Version: 1, Status: domain.PublishedStatus,
				Fields: map[string]domain.FieldDefinition{
					"cargo_class": {
						ID: cargoFieldID, Code: "cargo_class", FieldType: "SELECT",
						OptionsJSON: json.RawMessage(`{"options":[{"value":"GENERAL","label":"General"}]}`),
					},
				},
			},
		},
		store,
	)

	result, err := svc.MigrateToActiveTemplate(context.Background(), domain.MigrateCustomFieldValuesToActiveInput{
		TenantID: tenantID, EntityType: "TRANSPORT_ORDER", EntityID: entityID,
		TemplateCode: "transport_order_default", AllowWarnings: true,
	})
	if err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	if !store.writeCalled || result.Status != "migrated_with_warnings" {
		t.Fatalf("unexpected result: %+v write=%v", result, store.writeCalled)
	}
}

func TestMigrateToActiveBlockedPreventsWrites(t *testing.T) {
	tenantID := uuid.New()
	targetID := uuid.New()
	entityID := uuid.New()
	store := &executeCustomFieldValueStore{
		items: []domain.CustomFieldValue{{FieldCode: "amount", ValueJSON: json.RawMessage(`42`)}},
	}
	svc := NewCustomFieldValueService(
		&executeFormTemplateReader{
			activeItems: []domain.FormTemplateSummary{{ID: targetID, Code: "transport_order_default", Version: 1}},
			target: &domain.PublishedTemplateContext{
				ID: targetID, TenantID: tenantID, EntityType: "TRANSPORT_ORDER",
				Code: "transport_order_default", Version: 1, Status: domain.PublishedStatus,
				Fields: map[string]domain.FieldDefinition{
					"amount": {Code: "amount", FieldType: "MONEY"},
				},
			},
		},
		store,
	)

	_, err := svc.MigrateToActiveTemplate(context.Background(), domain.MigrateCustomFieldValuesToActiveInput{
		TenantID: tenantID, EntityType: "TRANSPORT_ORDER", EntityID: entityID,
		TemplateCode: "transport_order_default",
	})
	if err == nil {
		t.Fatal("expected blocked error")
	}
	appErr, ok := err.(*apperrors.AppError)
	if !ok || appErr.Code != apperrors.CodeMigrationBlocked {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.writeCalled {
		t.Fatal("must not write when blocked")
	}
}
