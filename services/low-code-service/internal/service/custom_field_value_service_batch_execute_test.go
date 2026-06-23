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

type batchExecuteCustomFieldValueStore struct {
	itemsByEntity map[uuid.UUID][]domain.CustomFieldValue
	writeCalls    int
	lastInput     domain.UpsertCustomFieldValuesInput
}

func (s *batchExecuteCustomFieldValueStore) ListByEntity(ctx context.Context, tenantID uuid.UUID, entityType string, entityID uuid.UUID) ([]domain.CustomFieldValue, error) {
	return s.itemsByEntity[entityID], nil
}

func (s *batchExecuteCustomFieldValueStore) UpsertBatch(ctx context.Context, input domain.UpsertCustomFieldValuesInput, values []repository.ResolvedCustomFieldValue) (int, error) {
	return 0, errors.New("unexpected upsert")
}

func (s *batchExecuteCustomFieldValueStore) ReplaceFieldCodesBatch(
	ctx context.Context,
	input domain.UpsertCustomFieldValuesInput,
	fieldCodes []string,
	values []repository.ResolvedCustomFieldValue,
) (int, error) {
	s.writeCalls++
	s.lastInput = input
	return len(values), nil
}

func batchExecuteTargetReader(tenantID, targetID uuid.UUID) *executeFormTemplateReader {
	cargoFieldID := uuid.New()
	return &executeFormTemplateReader{
		activeItems: []domain.FormTemplateSummary{{ID: targetID, Code: "transport_order_default", Version: 1}},
		target: &domain.PublishedTemplateContext{
			ID: targetID, TenantID: tenantID, EntityType: "TRANSPORT_ORDER",
			Code: "transport_order_default", Version: 1, Status: domain.PublishedStatus,
			Fields: map[string]domain.FieldDefinition{
				"cargo_class": {
					ID: cargoFieldID, Code: "cargo_class", FieldType: "SELECT",
					OptionsJSON: json.RawMessage(`{"options":[{"value":"GENERAL","label":"General"}]}`),
				},
				"amount": {Code: "amount", FieldType: "MONEY"},
			},
		},
	}
}

func TestBatchMigrateToActiveSafeEntitiesMigrated(t *testing.T) {
	tenantID := uuid.New()
	targetID := uuid.New()
	entityID := uuid.New()
	store := &batchExecuteCustomFieldValueStore{
		itemsByEntity: map[uuid.UUID][]domain.CustomFieldValue{
			entityID: {{FieldCode: "cargo_class", ValueJSON: json.RawMessage(`"GENERAL"`)}},
		},
	}
	svc := NewCustomFieldValueService(batchExecuteTargetReader(tenantID, targetID), store)

	result, err := svc.BatchMigrateToActiveTemplate(context.Background(), domain.BatchMigrateCustomFieldValuesToActiveInput{
		TenantID: tenantID, EntityType: "TRANSPORT_ORDER", EntityIDs: []uuid.UUID{entityID},
		TemplateCode: "transport_order_default", SkipBlocked: true,
	})
	if err != nil {
		t.Fatalf("batch migrate failed: %v", err)
	}
	if result.Status != domain.BatchMigrateStatusCompleted || result.Summary.Migrated != 1 || store.writeCalls != 1 {
		t.Fatalf("unexpected result: %+v writes=%d", result, store.writeCalls)
	}
	if store.lastInput.MigrationAudit == nil || store.lastInput.MigrationAudit.BatchID == uuid.Nil {
		t.Fatal("expected batch audit metadata")
	}
}

func TestBatchMigrateToActiveWarningBlockedWithoutAllowWarnings(t *testing.T) {
	tenantID := uuid.New()
	targetID := uuid.New()
	entityID := uuid.New()
	store := &batchExecuteCustomFieldValueStore{
		itemsByEntity: map[uuid.UUID][]domain.CustomFieldValue{
			entityID: {
				{FieldCode: "cargo_class", ValueJSON: json.RawMessage(`"GENERAL"`)},
				{FieldCode: "deprecated_field", ValueJSON: json.RawMessage(`"legacy"`)},
			},
		},
	}
	svc := NewCustomFieldValueService(batchExecuteTargetReader(tenantID, targetID), store)

	_, err := svc.BatchMigrateToActiveTemplate(context.Background(), domain.BatchMigrateCustomFieldValuesToActiveInput{
		TenantID: tenantID, EntityType: "TRANSPORT_ORDER", EntityIDs: []uuid.UUID{entityID},
		TemplateCode: "transport_order_default", AllowWarnings: false, SkipBlocked: true,
	})
	if err == nil {
		t.Fatal("expected batch warning confirmation error")
	}
	appErr, ok := err.(*apperrors.AppError)
	if !ok || appErr.Code != apperrors.CodeBatchMigrationWarningsRequireConfirmation {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.writeCalls != 0 {
		t.Fatal("must not write when all entities are warnings without confirmation")
	}
}

func TestBatchMigrateToActiveWarningMigratedWithAllowWarnings(t *testing.T) {
	tenantID := uuid.New()
	targetID := uuid.New()
	entityID := uuid.New()
	store := &batchExecuteCustomFieldValueStore{
		itemsByEntity: map[uuid.UUID][]domain.CustomFieldValue{
			entityID: {
				{FieldCode: "cargo_class", ValueJSON: json.RawMessage(`"GENERAL"`)},
				{FieldCode: "deprecated_field", ValueJSON: json.RawMessage(`"legacy"`)},
			},
		},
	}
	svc := NewCustomFieldValueService(batchExecuteTargetReader(tenantID, targetID), store)

	result, err := svc.BatchMigrateToActiveTemplate(context.Background(), domain.BatchMigrateCustomFieldValuesToActiveInput{
		TenantID: tenantID, EntityType: "TRANSPORT_ORDER", EntityIDs: []uuid.UUID{entityID},
		TemplateCode: "transport_order_default", AllowWarnings: true, SkipBlocked: true,
	})
	if err != nil {
		t.Fatalf("batch migrate failed: %v", err)
	}
	if result.Summary.Migrated != 1 || store.writeCalls != 1 || result.Items[0].Status != domain.BatchMigrateItemStatusMigratedWithWarnings {
		t.Fatalf("unexpected result: %+v writes=%d", result, store.writeCalls)
	}
}

func TestBatchMigrateToActiveBlockedFailsWholeBatchWhenSkipBlockedFalse(t *testing.T) {
	tenantID := uuid.New()
	targetID := uuid.New()
	safeEntityID := uuid.New()
	blockedEntityID := uuid.New()
	store := &batchExecuteCustomFieldValueStore{
		itemsByEntity: map[uuid.UUID][]domain.CustomFieldValue{
			safeEntityID:    {{FieldCode: "cargo_class", ValueJSON: json.RawMessage(`"GENERAL"`)}},
			blockedEntityID: {{FieldCode: "amount", ValueJSON: json.RawMessage(`42`)}},
		},
	}
	svc := NewCustomFieldValueService(batchExecuteTargetReader(tenantID, targetID), store)

	_, err := svc.BatchMigrateToActiveTemplate(context.Background(), domain.BatchMigrateCustomFieldValuesToActiveInput{
		TenantID: tenantID, EntityType: "TRANSPORT_ORDER",
		EntityIDs: []uuid.UUID{safeEntityID, blockedEntityID},
		TemplateCode: "transport_order_default", SkipBlocked: false,
	})
	if err == nil {
		t.Fatal("expected batch blocked error")
	}
	appErr, ok := err.(*apperrors.AppError)
	if !ok || appErr.Code != apperrors.CodeBatchMigrationBlocked {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.writeCalls != 0 {
		t.Fatal("must not write when skip_blocked=false and blocked entities exist")
	}
}

func TestBatchMigrateToActiveBlockedSkippedWhenSkipBlockedTrue(t *testing.T) {
	tenantID := uuid.New()
	targetID := uuid.New()
	safeEntityID := uuid.New()
	blockedEntityID := uuid.New()
	store := &batchExecuteCustomFieldValueStore{
		itemsByEntity: map[uuid.UUID][]domain.CustomFieldValue{
			safeEntityID:    {{FieldCode: "cargo_class", ValueJSON: json.RawMessage(`"GENERAL"`)}},
			blockedEntityID: {{FieldCode: "amount", ValueJSON: json.RawMessage(`42`)}},
		},
	}
	svc := NewCustomFieldValueService(batchExecuteTargetReader(tenantID, targetID), store)

	result, err := svc.BatchMigrateToActiveTemplate(context.Background(), domain.BatchMigrateCustomFieldValuesToActiveInput{
		TenantID: tenantID, EntityType: "TRANSPORT_ORDER",
		EntityIDs: []uuid.UUID{safeEntityID, blockedEntityID},
		TemplateCode: "transport_order_default", SkipBlocked: true,
	})
	if err != nil {
		t.Fatalf("batch migrate failed: %v", err)
	}
	if result.Status != domain.BatchMigrateStatusPartiallyCompleted || result.Summary.Migrated != 1 || result.Summary.Blocked != 1 {
		t.Fatalf("unexpected summary: %+v", result.Summary)
	}
	if result.Items[1].Status != domain.BatchMigrateItemStatusSkipped || result.Items[1].Reason != domain.BatchMigrateSkipReasonBlocked {
		t.Fatalf("unexpected blocked item: %+v", result.Items[1])
	}
}

func TestBatchMigrateToActivePartialSuccessSummary(t *testing.T) {
	tenantID := uuid.New()
	targetID := uuid.New()
	safeEntityID := uuid.New()
	warningEntityID := uuid.New()
	blockedEntityID := uuid.New()
	store := &batchExecuteCustomFieldValueStore{
		itemsByEntity: map[uuid.UUID][]domain.CustomFieldValue{
			safeEntityID: {{FieldCode: "cargo_class", ValueJSON: json.RawMessage(`"GENERAL"`)}},
			warningEntityID: {
				{FieldCode: "cargo_class", ValueJSON: json.RawMessage(`"GENERAL"`)},
				{FieldCode: "deprecated_field", ValueJSON: json.RawMessage(`"legacy"`)},
			},
			blockedEntityID: {{FieldCode: "amount", ValueJSON: json.RawMessage(`42`)}},
		},
	}
	svc := NewCustomFieldValueService(batchExecuteTargetReader(tenantID, targetID), store)

	result, err := svc.BatchMigrateToActiveTemplate(context.Background(), domain.BatchMigrateCustomFieldValuesToActiveInput{
		TenantID: tenantID, EntityType: "TRANSPORT_ORDER",
		EntityIDs: []uuid.UUID{safeEntityID, warningEntityID, blockedEntityID},
		TemplateCode: "transport_order_default", AllowWarnings: true, SkipBlocked: true,
	})
	if err != nil {
		t.Fatalf("batch migrate failed: %v", err)
	}
	if result.Summary.Total != 3 || result.Summary.Migrated != 2 || result.Summary.Skipped != 1 || result.Summary.Blocked != 1 {
		t.Fatalf("unexpected summary: %+v", result.Summary)
	}
	if result.Status != domain.BatchMigrateStatusPartiallyCompleted {
		t.Fatalf("unexpected status: %s", result.Status)
	}
}

func TestBatchMigrateToActivePartialSuccessSkipsWarningsWithoutAllowWarnings(t *testing.T) {
	tenantID := uuid.New()
	targetID := uuid.New()
	safeEntityID := uuid.New()
	warningEntityID := uuid.New()
	store := &batchExecuteCustomFieldValueStore{
		itemsByEntity: map[uuid.UUID][]domain.CustomFieldValue{
			safeEntityID: {{FieldCode: "cargo_class", ValueJSON: json.RawMessage(`"GENERAL"`)}},
			warningEntityID: {
				{FieldCode: "cargo_class", ValueJSON: json.RawMessage(`"GENERAL"`)},
				{FieldCode: "deprecated_field", ValueJSON: json.RawMessage(`"legacy"`)},
			},
		},
	}
	svc := NewCustomFieldValueService(batchExecuteTargetReader(tenantID, targetID), store)

	result, err := svc.BatchMigrateToActiveTemplate(context.Background(), domain.BatchMigrateCustomFieldValuesToActiveInput{
		TenantID: tenantID, EntityType: "TRANSPORT_ORDER",
		EntityIDs: []uuid.UUID{safeEntityID, warningEntityID},
		TemplateCode: "transport_order_default", AllowWarnings: false, SkipBlocked: true,
	})
	if err != nil {
		t.Fatalf("batch migrate failed: %v", err)
	}
	if result.Summary.Migrated != 1 || result.Summary.Skipped != 1 || result.Summary.Warnings != 1 {
		t.Fatalf("unexpected summary: %+v", result.Summary)
	}
	if result.Items[1].Reason != domain.BatchMigrateSkipReasonWarningsRequireConfirmation {
		t.Fatalf("unexpected skip reason: %s", result.Items[1].Reason)
	}
}

func TestBatchMigrateToActiveRepeatedExecuteIsStable(t *testing.T) {
	tenantID := uuid.New()
	targetID := uuid.New()
	entityID := uuid.New()
	store := &batchExecuteCustomFieldValueStore{
		itemsByEntity: map[uuid.UUID][]domain.CustomFieldValue{
			entityID: {{FieldCode: "cargo_class", ValueJSON: json.RawMessage(`"GENERAL"`)}},
		},
	}
	svc := NewCustomFieldValueService(batchExecuteTargetReader(tenantID, targetID), store)
	input := domain.BatchMigrateCustomFieldValuesToActiveInput{
		TenantID: tenantID, EntityType: "TRANSPORT_ORDER", EntityIDs: []uuid.UUID{entityID},
		TemplateCode: "transport_order_default", SkipBlocked: true,
	}

	first, err := svc.BatchMigrateToActiveTemplate(context.Background(), input)
	if err != nil {
		t.Fatalf("first batch migrate failed: %v", err)
	}
	second, err := svc.BatchMigrateToActiveTemplate(context.Background(), input)
	if err != nil {
		t.Fatalf("second batch migrate failed: %v", err)
	}
	if first.Summary.Migrated != 1 || second.Summary.Migrated != 1 {
		t.Fatalf("expected stable migrated counts: first=%+v second=%+v", first.Summary, second.Summary)
	}
}

func TestBatchMigrateToActiveTenantIsolation(t *testing.T) {
	tenantID := uuid.MustParse("74519f22-ff9b-4a8b-8fff-a958c689682f")
	otherTenant := uuid.New()
	targetID := uuid.New()
	entityID := uuid.New()
	store := &batchExecuteCustomFieldValueStore{
		itemsByEntity: map[uuid.UUID][]domain.CustomFieldValue{
			entityID: {{FieldCode: "cargo_class", ValueJSON: json.RawMessage(`"GENERAL"`)}},
		},
	}
	svc := NewCustomFieldValueService(batchExecuteTargetReader(tenantID, targetID), store)

	_, err := svc.BatchMigrateToActiveTemplate(context.Background(), domain.BatchMigrateCustomFieldValuesToActiveInput{
		TenantID: otherTenant, EntityType: "TRANSPORT_ORDER", EntityIDs: []uuid.UUID{entityID},
		TemplateCode: "transport_order_default", SkipBlocked: true,
	})
	if err == nil {
		t.Fatal("expected tenant mismatch/not found error")
	}
}

func TestBatchMigrateToActiveEmptyEntityIDsRejected(t *testing.T) {
	svc := NewCustomFieldValueService(&executeFormTemplateReader{}, &batchExecuteCustomFieldValueStore{})
	_, err := svc.BatchMigrateToActiveTemplate(context.Background(), domain.BatchMigrateCustomFieldValuesToActiveInput{
		TenantID: uuid.New(), EntityType: "TRANSPORT_ORDER", TemplateCode: "transport_order_default",
	})
	if err == nil {
		t.Fatal("expected empty entity_ids validation error")
	}
}

func TestBatchMigrateToActiveTooManyEntityIDsRejected(t *testing.T) {
	svc := NewCustomFieldValueService(&executeFormTemplateReader{}, &batchExecuteCustomFieldValueStore{})
	ids := make([]uuid.UUID, domain.MaxMigrationPreviewEntityCount+1)
	for i := range ids {
		ids[i] = uuid.New()
	}
	_, err := svc.BatchMigrateToActiveTemplate(context.Background(), domain.BatchMigrateCustomFieldValuesToActiveInput{
		TenantID: uuid.New(), EntityType: "TRANSPORT_ORDER", EntityIDs: ids,
		TemplateCode: "transport_order_default",
	})
	if err == nil {
		t.Fatal("expected max entity_ids validation error")
	}
}

func TestBatchMigrateToActiveInvalidEntityType(t *testing.T) {
	svc := NewCustomFieldValueService(&executeFormTemplateReader{}, &batchExecuteCustomFieldValueStore{})
	_, err := svc.BatchMigrateToActiveTemplate(context.Background(), domain.BatchMigrateCustomFieldValuesToActiveInput{
		TenantID: uuid.New(), EntityType: "INVALID", EntityIDs: []uuid.UUID{uuid.New()},
		TemplateCode: "transport_order_default",
	})
	if err == nil {
		t.Fatal("expected entity type validation error")
	}
}

func TestBatchMigrateToActiveNoAuditForSkippedEntities(t *testing.T) {
	tenantID := uuid.New()
	targetID := uuid.New()
	blockedEntityID := uuid.New()
	store := &batchExecuteCustomFieldValueStore{
		itemsByEntity: map[uuid.UUID][]domain.CustomFieldValue{
			blockedEntityID: {{FieldCode: "amount", ValueJSON: json.RawMessage(`42`)}},
		},
	}
	svc := NewCustomFieldValueService(batchExecuteTargetReader(tenantID, targetID), store)

	result, err := svc.BatchMigrateToActiveTemplate(context.Background(), domain.BatchMigrateCustomFieldValuesToActiveInput{
		TenantID: tenantID, EntityType: "TRANSPORT_ORDER", EntityIDs: []uuid.UUID{blockedEntityID},
		TemplateCode: "transport_order_default", SkipBlocked: true,
	})
	if err != nil {
		t.Fatalf("batch migrate failed: %v", err)
	}
	if store.writeCalls != 0 || result.Summary.Migrated != 0 {
		t.Fatalf("expected no writes for skipped blocked entity: writes=%d summary=%+v", store.writeCalls, result.Summary)
	}
}
