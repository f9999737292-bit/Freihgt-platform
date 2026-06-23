package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/low-code-service/internal/domain"
	apperrors "github.com/freight-platform/low-code-service/internal/platform/errors"
	"github.com/freight-platform/low-code-service/internal/repository"
)

type edgeCaseCustomFieldValueStore struct {
	items      []domain.CustomFieldValue
	writeCount int
	lastCodes  []string
}

func (s *edgeCaseCustomFieldValueStore) ListByEntity(ctx context.Context, tenantID uuid.UUID, entityType string, entityID uuid.UUID) ([]domain.CustomFieldValue, error) {
	return s.items, nil
}

func (s *edgeCaseCustomFieldValueStore) UpsertBatch(ctx context.Context, input domain.UpsertCustomFieldValuesInput, values []repository.ResolvedCustomFieldValue) (int, error) {
	return 0, errors.New("unexpected upsert")
}

func (s *edgeCaseCustomFieldValueStore) ReplaceFieldCodesBatch(
	ctx context.Context,
	input domain.UpsertCustomFieldValuesInput,
	fieldCodes []string,
	values []repository.ResolvedCustomFieldValue,
) (int, error) {
	s.writeCount++
	s.lastCodes = append([]string(nil), fieldCodes...)
	return len(values), nil
}

func newEdgeCaseService(
	tenantID uuid.UUID,
	targetID uuid.UUID,
	fields map[string]domain.FieldDefinition,
	items []domain.CustomFieldValue,
) (*CustomFieldValueService, *edgeCaseCustomFieldValueStore) {
	store := &edgeCaseCustomFieldValueStore{items: items}
	svc := NewCustomFieldValueService(
		&executeFormTemplateReader{
			activeItems: []domain.FormTemplateSummary{{ID: targetID, Code: "transport_order_default", Version: 1}},
			target: &domain.PublishedTemplateContext{
				ID: targetID, TenantID: tenantID, EntityType: "TRANSPORT_ORDER",
				Code: "transport_order_default", Version: 1, Status: domain.PublishedStatus,
				Fields: fields,
			},
		},
		store,
	)
	return svc, store
}

func TestPreviewMigrationToActiveEmptyValuesSafeOrWarning(t *testing.T) {
	tenantID := uuid.New()
	targetID := uuid.New()
	entityID := uuid.New()
	svc, store := newEdgeCaseService(tenantID, targetID, map[string]domain.FieldDefinition{
		"optional_note": {Code: "optional_note", FieldType: "TEXT"},
	}, nil)

	result, err := svc.PreviewMigrationToActive(context.Background(), domain.MigrationPreviewInput{
		TenantID: tenantID, EntityType: "TRANSPORT_ORDER", TemplateCode: "transport_order_default",
		EntityIDs: []uuid.UUID{entityID},
	})
	if err != nil {
		t.Fatalf("preview failed: %v", err)
	}
	if store.writeCount != 0 {
		t.Fatal("preview must not write")
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected one preview item, got %+v", result.Items)
	}
	if result.Items[0].Status != domain.MigrationPreviewStatusSafe {
		t.Fatalf("expected SAFE for empty optional template, got %s", result.Items[0].Status)
	}
}

func TestPreviewMigrationToActiveEmptyValuesMissingRequiredWarning(t *testing.T) {
	tenantID := uuid.New()
	targetID := uuid.New()
	entityID := uuid.New()
	svc, _ := newEdgeCaseService(tenantID, targetID, map[string]domain.FieldDefinition{
		"required_note": {Code: "required_note", FieldType: "TEXT", Required: true},
	}, nil)

	result, err := svc.PreviewMigrationToActive(context.Background(), domain.MigrationPreviewInput{
		TenantID: tenantID, EntityType: "TRANSPORT_ORDER", TemplateCode: "transport_order_default",
		EntityIDs: []uuid.UUID{entityID},
	})
	if err != nil {
		t.Fatalf("preview failed: %v", err)
	}
	if result.Items[0].Status != domain.MigrationPreviewStatusWarning {
		t.Fatalf("expected WARNING, got %s", result.Items[0].Status)
	}
}

func TestMigrateToActiveEmptyValuesNoWriteWhenNothingCopied(t *testing.T) {
	tenantID := uuid.New()
	targetID := uuid.New()
	entityID := uuid.New()
	svc, store := newEdgeCaseService(tenantID, targetID, map[string]domain.FieldDefinition{
		"optional_note": {Code: "optional_note", FieldType: "TEXT"},
	}, nil)

	result, err := svc.MigrateToActiveTemplate(context.Background(), domain.MigrateCustomFieldValuesToActiveInput{
		TenantID: tenantID, EntityType: "TRANSPORT_ORDER", EntityID: entityID,
		TemplateCode: "transport_order_default",
	})
	if err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	if store.writeCount != 0 {
		t.Fatal("expected no write when nothing copied")
	}
	if result.MigratedCount != 0 || result.Status != "migrated" {
		t.Fatalf("unexpected empty migrate result: %+v", result)
	}
}

func TestMigrateToActiveMissingRequiredRequiresAllowWarnings(t *testing.T) {
	tenantID := uuid.New()
	targetID := uuid.New()
	entityID := uuid.New()
	svc, store := newEdgeCaseService(tenantID, targetID, map[string]domain.FieldDefinition{
		"required_note": {Code: "required_note", FieldType: "TEXT", Required: true},
	}, nil)

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
	if store.writeCount != 0 {
		t.Fatal("must not write without allow_warnings")
	}
}

func TestMigrateToActiveMissingRequiredMigratesWithAllowWarnings(t *testing.T) {
	tenantID := uuid.New()
	targetID := uuid.New()
	entityID := uuid.New()
	svc, store := newEdgeCaseService(tenantID, targetID, map[string]domain.FieldDefinition{
		"required_note": {ID: uuid.New(), Code: "required_note", FieldType: "TEXT", Required: true},
	}, nil)

	result, err := svc.MigrateToActiveTemplate(context.Background(), domain.MigrateCustomFieldValuesToActiveInput{
		TenantID: tenantID, EntityType: "TRANSPORT_ORDER", EntityID: entityID,
		TemplateCode: "transport_order_default", AllowWarnings: true,
	})
	if err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	if store.writeCount != 0 {
		t.Fatal("expected no field writes when nothing copied despite allow_warnings")
	}
	if result.Status != "migrated_with_warnings" {
		t.Fatalf("expected migrated_with_warnings, got %s", result.Status)
	}
}

func TestMigrateToActiveRepeatedExecuteIsStable(t *testing.T) {
	tenantID := uuid.New()
	targetID := uuid.New()
	entityID := uuid.New()
	cargoFieldID := uuid.New()
	svc, store := newEdgeCaseService(tenantID, targetID, map[string]domain.FieldDefinition{
		"cargo_class": {
			ID: cargoFieldID, Code: "cargo_class", FieldType: "SELECT",
			OptionsJSON: json.RawMessage(`{"options":[{"value":"GENERAL","label":"General"}]}`),
		},
	}, []domain.CustomFieldValue{
		{FieldCode: "cargo_class", ValueJSON: json.RawMessage(`"GENERAL"`), FormTemplateID: targetID},
	})

	input := domain.MigrateCustomFieldValuesToActiveInput{
		TenantID: tenantID, EntityType: "TRANSPORT_ORDER", EntityID: entityID,
		TemplateCode: "transport_order_default",
	}

	first, err := svc.MigrateToActiveTemplate(context.Background(), input)
	if err != nil {
		t.Fatalf("first migrate failed: %v", err)
	}
	second, err := svc.MigrateToActiveTemplate(context.Background(), input)
	if err != nil {
		t.Fatalf("second migrate failed: %v", err)
	}
	if store.writeCount != 2 {
		t.Fatalf("expected two writes, got %d", store.writeCount)
	}
	if first.MigratedCount != second.MigratedCount || first.Status != second.Status {
		t.Fatalf("idempotent results differ: first=%+v second=%+v", first, second)
	}
	if len(store.lastCodes) != 1 || store.lastCodes[0] != "cargo_class" {
		t.Fatalf("unexpected last copied codes: %+v", store.lastCodes)
	}
}

func TestMigrateToActiveTenantIsolationOnExecute(t *testing.T) {
	tenantID := uuid.MustParse("74519f22-ff9b-4a8b-8fff-a958c689682f")
	otherTenant := uuid.New()
	targetID := uuid.New()
	entityID := uuid.New()
	svc, store := newEdgeCaseService(tenantID, targetID, map[string]domain.FieldDefinition{
		"note": {Code: "note", FieldType: "TEXT"},
	}, []domain.CustomFieldValue{{FieldCode: "note", ValueJSON: json.RawMessage(`"x"`)}})

	_, err := svc.MigrateToActiveTemplate(context.Background(), domain.MigrateCustomFieldValuesToActiveInput{
		TenantID: otherTenant, EntityType: "TRANSPORT_ORDER", EntityID: entityID,
		TemplateCode: "transport_order_default",
	})
	if err == nil {
		t.Fatal("expected tenant mismatch/not found error")
	}
	if store.writeCount != 0 {
		t.Fatal("must not write for foreign tenant")
	}
}

func TestMigrateToActiveInvalidEntityType(t *testing.T) {
	svc, store := newEdgeCaseService(uuid.New(), uuid.New(), nil, nil)
	_, err := svc.MigrateToActiveTemplate(context.Background(), domain.MigrateCustomFieldValuesToActiveInput{
		TenantID: uuid.New(), EntityType: "INVALID", EntityID: uuid.New(),
		TemplateCode: "transport_order_default",
	})
	if err == nil {
		t.Fatal("expected entity type invalid")
	}
	if store.writeCount != 0 {
		t.Fatal("must not write on invalid entity type")
	}
}

func TestMigrateToActiveInvalidTargetTemplateNotFound(t *testing.T) {
	tenantID := uuid.New()
	svc := NewCustomFieldValueService(
		&executeFormTemplateReader{activeItems: nil},
		&edgeCaseCustomFieldValueStore{},
	)
	_, err := svc.MigrateToActiveTemplate(context.Background(), domain.MigrateCustomFieldValuesToActiveInput{
		TenantID: tenantID, EntityType: "TRANSPORT_ORDER", EntityID: uuid.New(),
		TemplateCode: "missing_template",
	})
	if err == nil {
		t.Fatal("expected form template not found")
	}
}

func TestPreviewMigrationToActiveInvalidEntityType(t *testing.T) {
	svc, _ := newEdgeCaseService(uuid.New(), uuid.New(), nil, nil)
	_, err := svc.PreviewMigrationToActive(context.Background(), domain.MigrationPreviewInput{
		TenantID: uuid.New(), EntityType: "NOT_A_TYPE", EntityIDs: []uuid.UUID{uuid.New()},
		TemplateCode: "transport_order_default",
	})
	if err == nil {
		t.Fatal("expected invalid entity type")
	}
}

func TestPreviewMigrationToActiveInvalidTargetTemplateID(t *testing.T) {
	tenantID := uuid.New()
	svc := NewCustomFieldValueService(
		&executeFormTemplateReader{},
		&edgeCaseCustomFieldValueStore{},
	)
	_, err := svc.PreviewMigrationToActive(context.Background(), domain.MigrationPreviewInput{
		TenantID: tenantID, EntityType: "TRANSPORT_ORDER", EntityIDs: []uuid.UUID{uuid.New()},
		TargetTemplateID: uuid.New(),
	})
	if err == nil {
		t.Fatal("expected target template lookup error")
	}
}
