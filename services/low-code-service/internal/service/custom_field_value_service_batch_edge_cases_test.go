package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/low-code-service/internal/domain"
	apperrors "github.com/freight-platform/low-code-service/internal/platform/errors"
)

func TestBatchPreviewDuplicateEntityIDsProcessedTwice(t *testing.T) {
	tenantID := uuid.New()
	targetID := uuid.New()
	entityID := uuid.New()
	store := &batchExecuteCustomFieldValueStore{
		itemsByEntity: map[uuid.UUID][]domain.CustomFieldValue{
			entityID: {{FieldCode: "cargo_class", ValueJSON: json.RawMessage(`"GENERAL"`)}},
		},
	}
	svc := NewCustomFieldValueService(batchExecuteTargetReader(tenantID, targetID), store)

	result, err := svc.PreviewMigrationToActive(context.Background(), domain.MigrationPreviewInput{
		TenantID:     tenantID,
		EntityType:   "TRANSPORT_ORDER",
		EntityIDs:    []uuid.UUID{entityID, entityID},
		TemplateCode: "transport_order_default",
	})
	if err != nil {
		t.Fatalf("preview failed: %v", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("duplicate entity_ids are not deduplicated; expected 2 preview items, got %d", len(result.Items))
	}
	if result.Summary.EntitiesChecked != 2 || result.Summary.SafeToMigrate != 2 {
		t.Fatalf("unexpected summary for duplicate ids: %+v", result.Summary)
	}
}

func TestBatchPreviewDoesNotWrite(t *testing.T) {
	tenantID := uuid.New()
	targetID := uuid.New()
	entityID := uuid.New()
	store := &batchExecuteCustomFieldValueStore{
		itemsByEntity: map[uuid.UUID][]domain.CustomFieldValue{
			entityID: {{FieldCode: "cargo_class", ValueJSON: json.RawMessage(`"GENERAL"`)}},
		},
	}
	svc := NewCustomFieldValueService(batchExecuteTargetReader(tenantID, targetID), store)

	_, err := svc.PreviewMigrationToActive(context.Background(), domain.MigrationPreviewInput{
		TenantID:     tenantID,
		EntityType:   "TRANSPORT_ORDER",
		EntityIDs:    []uuid.UUID{entityID},
		TemplateCode: "transport_order_default",
	})
	if err != nil {
		t.Fatalf("preview failed: %v", err)
	}
	if store.writeCalls != 0 {
		t.Fatal("batch preview must not write custom field values")
	}
}

func TestBatchMigrateToActiveBatchIDPropagatedToAuditMetadata(t *testing.T) {
	tenantID := uuid.New()
	targetID := uuid.New()
	entityID := uuid.New()
	batchID := uuid.New()
	store := &batchExecuteCustomFieldValueStore{
		itemsByEntity: map[uuid.UUID][]domain.CustomFieldValue{
			entityID: {{FieldCode: "cargo_class", ValueJSON: json.RawMessage(`"GENERAL"`)}},
		},
	}
	svc := NewCustomFieldValueService(batchExecuteTargetReader(tenantID, targetID), store)

	result, err := svc.BatchMigrateToActiveTemplate(context.Background(), domain.BatchMigrateCustomFieldValuesToActiveInput{
		TenantID:      tenantID,
		EntityType:    "TRANSPORT_ORDER",
		EntityIDs:     []uuid.UUID{entityID},
		TemplateCode:  "transport_order_default",
		BatchID:       batchID,
		SkipBlocked:   true,
		AllowWarnings: true,
	})
	if err != nil {
		t.Fatalf("batch migrate failed: %v", err)
	}
	if result.BatchID != batchID {
		t.Fatalf("expected batch_id in result, got %s", result.BatchID)
	}
	audit := store.lastInput.MigrationAudit
	if audit == nil {
		t.Fatal("expected migration audit metadata")
	}
	if audit.BatchID != batchID {
		t.Fatalf("expected batch_id in audit metadata, got %s", audit.BatchID)
	}
	if audit.TemplateCode != "transport_order_default" {
		t.Fatalf("expected template_code in audit metadata, got %q", audit.TemplateCode)
	}
	if !audit.SkipBlocked {
		t.Fatal("expected skip_blocked=true in audit metadata")
	}

	_, newJSON, err := domain.BuildCustomFieldValuesMigratedToActiveAuditPayload(
		audit.SourceTemplateID,
		targetID,
		audit.PreviewItem,
		audit.AllowWarnings,
		&domain.BatchMigrationAuditContext{
			BatchID:      audit.BatchID,
			EntityType:   "TRANSPORT_ORDER",
			EntityID:     entityID,
			TemplateCode: audit.TemplateCode,
			SkipBlocked:  audit.SkipBlocked,
		},
		&domain.MigrationExecutionAuditContext{
			MigrationStatus: "migrated",
			MigratedCount:   1,
			SkippedCount:    0,
		},
	)
	if err != nil {
		t.Fatalf("build audit payload: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(newJSON, &payload); err != nil {
		t.Fatalf("unmarshal audit payload: %v", err)
	}
	for _, key := range []string{"batch_id", "template_code", "skip_blocked", "preview_status", "migration_status"} {
		if _, ok := payload[key]; !ok {
			t.Fatalf("expected audit payload key %q", key)
		}
	}
	if payload["batch_id"] != batchID.String() {
		t.Fatalf("unexpected batch_id in payload: %#v", payload["batch_id"])
	}
}

func TestBatchMigrateRepeatedExecuteWriteCountStable(t *testing.T) {
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
		TenantID:     tenantID,
		EntityType:   "TRANSPORT_ORDER",
		EntityIDs:    []uuid.UUID{entityID},
		TemplateCode: "transport_order_default",
		SkipBlocked:  true,
	}

	if _, err := svc.BatchMigrateToActiveTemplate(context.Background(), input); err != nil {
		t.Fatalf("first batch migrate failed: %v", err)
	}
	if _, err := svc.BatchMigrateToActiveTemplate(context.Background(), input); err != nil {
		t.Fatalf("second batch migrate failed: %v", err)
	}
	if store.writeCalls != 2 {
		t.Fatalf("expected one write per execute, got %d writes", store.writeCalls)
	}
}

func TestBatchExecuteTargetTemplateNotPublishedRejected(t *testing.T) {
	tenantID := uuid.New()
	targetID := uuid.New()
	entityID := uuid.New()
	svc := NewCustomFieldValueService(
		&batchEdgeCasePublishedTemplateReader{
			target: &domain.PublishedTemplateContext{
				ID: targetID, TenantID: tenantID, EntityType: "TRANSPORT_ORDER",
				Code: "transport_order_default", Version: 1, Status: domain.DraftStatus,
			},
		},
		&batchExecuteCustomFieldValueStore{},
	)

	_, err := svc.BatchMigrateToActiveTemplate(context.Background(), domain.BatchMigrateCustomFieldValuesToActiveInput{
		TenantID:         tenantID,
		EntityType:       "TRANSPORT_ORDER",
		EntityIDs:        []uuid.UUID{entityID},
		TargetTemplateID: targetID,
		SkipBlocked:      true,
	})
	if err == nil {
		t.Fatal("expected not published error")
	}
	appErr, ok := err.(*apperrors.AppError)
	if !ok || appErr.Code != apperrors.CodeFormTemplateNotPublished {
		t.Fatalf("unexpected error: %v", err)
	}
}

type batchEdgeCasePublishedTemplateReader struct {
	target *domain.PublishedTemplateContext
}

func (s *batchEdgeCasePublishedTemplateReader) GetPublishedTemplateContext(ctx context.Context, tenantID uuid.UUID, templateID uuid.UUID) (*domain.PublishedTemplateContext, error) {
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

func (s *batchEdgeCasePublishedTemplateReader) ListActivePublished(ctx context.Context, tenantID uuid.UUID, entityType string, code string) ([]domain.FormTemplateSummary, error) {
	return nil, nil
}

func TestBatchExecuteRequiresTemplateCodeWhenAmbiguous(t *testing.T) {
	tenantID := uuid.New()
	entityID := uuid.New()
	svc := NewCustomFieldValueService(
		&executeFormTemplateReader{
			activeItems: []domain.FormTemplateSummary{
				{ID: uuid.New(), Code: "template_a", Version: 1},
				{ID: uuid.New(), Code: "template_b", Version: 1},
			},
		},
		&batchExecuteCustomFieldValueStore{},
	)

	_, err := svc.BatchMigrateToActiveTemplate(context.Background(), domain.BatchMigrateCustomFieldValuesToActiveInput{
		TenantID:    tenantID,
		EntityType:  "TRANSPORT_ORDER",
		EntityIDs:   []uuid.UUID{entityID},
		SkipBlocked: true,
	})
	if err == nil {
		t.Fatal("expected ambiguous template validation error")
	}
}
