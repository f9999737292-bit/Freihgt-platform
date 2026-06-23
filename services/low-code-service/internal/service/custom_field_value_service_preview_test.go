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

type previewFormTemplateReader struct {
	activeItems []domain.FormTemplateSummary
	target      *domain.PublishedTemplateContext
	source      *domain.PublishedTemplateContext
}

func (s *previewFormTemplateReader) GetPublishedTemplateContext(ctx context.Context, tenantID uuid.UUID, templateID uuid.UUID) (*domain.PublishedTemplateContext, error) {
	if s.target != nil && s.target.ID == templateID {
		if s.target.TenantID != tenantID {
			return nil, apperrors.TenantMismatch()
		}
		return s.target, nil
	}
	if s.source != nil && s.source.ID == templateID {
		if s.source.TenantID != tenantID {
			return nil, apperrors.TenantMismatch()
		}
		return s.source, nil
	}
	return nil, errors.New("not found")
}

func (s *previewFormTemplateReader) ListActivePublished(ctx context.Context, tenantID uuid.UUID, entityType string, code string) ([]domain.FormTemplateSummary, error) {
	return s.activeItems, nil
}

type previewCustomFieldValueStore struct {
	itemsByEntity map[uuid.UUID][]domain.CustomFieldValue
	writeCalled   bool
}

func (s *previewCustomFieldValueStore) ListByEntity(ctx context.Context, tenantID uuid.UUID, entityType string, entityID uuid.UUID) ([]domain.CustomFieldValue, error) {
	return s.itemsByEntity[entityID], nil
}

func (s *previewCustomFieldValueStore) UpsertBatch(ctx context.Context, input domain.UpsertCustomFieldValuesInput, values []repository.ResolvedCustomFieldValue) (int, error) {
	s.writeCalled = true
	return 0, errors.New("writes are not allowed in preview")
}

func (s *previewCustomFieldValueStore) ReplaceFieldCodesBatch(
	ctx context.Context,
	input domain.UpsertCustomFieldValuesInput,
	fieldCodes []string,
	values []repository.ResolvedCustomFieldValue,
) (int, error) {
	s.writeCalled = true
	return 0, errors.New("writes are not allowed in preview")
}

func TestPreviewMigrationToActiveRequiresTemplateCodeWhenAmbiguous(t *testing.T) {
	tenantID := uuid.New()
	svc := NewCustomFieldValueService(
		&previewFormTemplateReader{
			activeItems: []domain.FormTemplateSummary{
				{ID: uuid.New(), Code: "template_a", Version: 1},
				{ID: uuid.New(), Code: "template_b", Version: 1},
			},
		},
		&previewCustomFieldValueStore{},
	)

	_, err := svc.PreviewMigrationToActive(context.Background(), domain.MigrationPreviewInput{
		TenantID:   tenantID,
		EntityType: "TRANSPORT_ORDER",
		EntityIDs:  []uuid.UUID{uuid.New()},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestPreviewMigrationToActiveResolvesActiveTemplateAndDoesNotWrite(t *testing.T) {
	tenantID := uuid.New()
	targetID := uuid.New()
	entityID := uuid.New()
	store := &previewCustomFieldValueStore{
		itemsByEntity: map[uuid.UUID][]domain.CustomFieldValue{
			entityID: {
				{
					FieldCode:        "cargo_class",
					ValueJSON:        json.RawMessage(`"GENERAL"`),
					FormTemplateID:   uuid.New(),
				},
			},
		},
	}
	svc := NewCustomFieldValueService(
		&previewFormTemplateReader{
			activeItems: []domain.FormTemplateSummary{
				{ID: targetID, Code: "transport_order_default", Version: 2},
			},
			target: &domain.PublishedTemplateContext{
				ID:         targetID,
				TenantID:   tenantID,
				EntityType: "TRANSPORT_ORDER",
				Code:       "transport_order_default",
				Version:    2,
				Status:     domain.PublishedStatus,
				Fields: map[string]domain.FieldDefinition{
					"cargo_class": {
						Code:        "cargo_class",
						FieldType:   "SELECT",
						OptionsJSON: json.RawMessage(`{"options":[{"value":"GENERAL","label":"General"}]}`),
					},
				},
			},
		},
		store,
	)

	result, err := svc.PreviewMigrationToActive(context.Background(), domain.MigrationPreviewInput{
		TenantID:     tenantID,
		EntityType:   "TRANSPORT_ORDER",
		TemplateCode: "transport_order_default",
		EntityIDs:    []uuid.UUID{entityID},
	})
	if err != nil {
		t.Fatalf("preview failed: %v", err)
	}
	if store.writeCalled {
		t.Fatal("preview must not write to database")
	}
	if result.Summary.EntitiesChecked != 1 || result.TargetTemplate.Code != "transport_order_default" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestPreviewMigrationToActiveEnforcesMaxEntityIDs(t *testing.T) {
	svc := NewCustomFieldValueService(&previewFormTemplateReader{}, &previewCustomFieldValueStore{})
	entityIDs := make([]uuid.UUID, domain.MaxMigrationPreviewEntityCount+1)
	for i := range entityIDs {
		entityIDs[i] = uuid.New()
	}
	_, err := svc.PreviewMigrationToActive(context.Background(), domain.MigrationPreviewInput{
		TenantID:   uuid.New(),
		EntityType: "TRANSPORT_ORDER",
		EntityIDs:  entityIDs,
	})
	if err == nil {
		t.Fatal("expected max entity_ids validation error")
	}
}

func TestPreviewMigrationToActiveTenantIsolationOnList(t *testing.T) {
	tenantID := uuid.MustParse("74519f22-ff9b-4a8b-8fff-a958c689682f")
	otherTenant := uuid.New()
	targetID := uuid.New()
	entityID := uuid.New()
	store := &previewCustomFieldValueStore{
		itemsByEntity: map[uuid.UUID][]domain.CustomFieldValue{
			entityID: {{FieldCode: "note", ValueJSON: json.RawMessage(`"x"`)}},
		},
	}
	svc := NewCustomFieldValueService(
		&previewFormTemplateReader{
			activeItems: []domain.FormTemplateSummary{{ID: targetID, Code: "transport_order_default", Version: 1}},
			target: &domain.PublishedTemplateContext{
				ID: targetID, TenantID: tenantID, EntityType: "TRANSPORT_ORDER",
				Code: "transport_order_default", Version: 1, Status: domain.PublishedStatus,
				Fields: map[string]domain.FieldDefinition{
					"note": {Code: "note", FieldType: "TEXT"},
				},
			},
		},
		store,
	)

	_, err := svc.PreviewMigrationToActive(context.Background(), domain.MigrationPreviewInput{
		TenantID:     otherTenant,
		EntityType:   "TRANSPORT_ORDER",
		TemplateCode: "transport_order_default",
		EntityIDs:    []uuid.UUID{entityID},
	})
	if err == nil {
		t.Fatal("expected tenant mismatch/not found error")
	}
}
