package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/freight-platform/low-code-service/internal/domain"
	apperrors "github.com/freight-platform/low-code-service/internal/platform/errors"
	"github.com/freight-platform/low-code-service/internal/repository"
)

type FormTemplateReader interface {
	GetPublishedTemplateContext(ctx context.Context, tenantID uuid.UUID, templateID uuid.UUID) (*domain.PublishedTemplateContext, error)
}

type CustomFieldValueStore interface {
	ListByEntity(ctx context.Context, tenantID uuid.UUID, entityType string, entityID uuid.UUID) ([]domain.CustomFieldValue, error)
	UpsertBatch(ctx context.Context, input domain.UpsertCustomFieldValuesInput, values []repository.ResolvedCustomFieldValue) (int, error)
}

type CustomFieldValueService struct {
	templates FormTemplateReader
	values    CustomFieldValueStore
}

func NewCustomFieldValueService(templates FormTemplateReader, values CustomFieldValueStore) *CustomFieldValueService {
	return &CustomFieldValueService{templates: templates, values: values}
}

func (s *CustomFieldValueService) GetByEntity(
	ctx context.Context,
	tenantID uuid.UUID,
	entityType string,
	entityID uuid.UUID,
) ([]domain.CustomFieldValue, error) {
	if err := domain.ValidateEntityType(entityType); err != nil {
		return nil, toEntityTypeInvalid(err)
	}
	if entityID == uuid.Nil {
		return nil, apperrors.EntityIDInvalid(map[string]any{"field": "entity_id"})
	}
	return s.values.ListByEntity(ctx, tenantID, entityType, entityID)
}

func (s *CustomFieldValueService) Upsert(
	ctx context.Context,
	input domain.UpsertCustomFieldValuesInput,
) (*domain.UpsertCustomFieldValuesResult, error) {
	if err := domain.ValidateEntityType(input.EntityType); err != nil {
		return nil, toEntityTypeInvalid(err)
	}
	if input.EntityID == uuid.Nil {
		return nil, apperrors.EntityIDInvalid(map[string]any{"field": "entity_id"})
	}
	if input.FormTemplateID == uuid.Nil {
		return nil, apperrors.Validation("form_template_id is required", map[string]any{"field": "form_template_id"})
	}
	if len(input.Values) == 0 {
		return nil, apperrors.Validation("values must not be empty", map[string]any{"field": "values"})
	}

	tmpl, err := s.templates.GetPublishedTemplateContext(ctx, input.TenantID, input.FormTemplateID)
	if err != nil {
		return nil, err
	}
	if tmpl.EntityType != input.EntityType {
		return nil, apperrors.Validation("entity_type does not match form template", map[string]any{
			"entity_type":          input.EntityType,
			"template_entity_type": tmpl.EntityType,
		})
	}

	resolved := make([]repository.ResolvedCustomFieldValue, 0, len(input.Values))
	seen := make(map[string]struct{}, len(input.Values))
	for _, item := range input.Values {
		if item.FieldCode == "" {
			return nil, apperrors.Validation("field_code is required", map[string]any{"field": "field_code"})
		}
		if _, dup := seen[item.FieldCode]; dup {
			return nil, apperrors.Validation("duplicate field_code in request", map[string]any{"field_code": item.FieldCode})
		}
		seen[item.FieldCode] = struct{}{}

		field, ok := tmpl.Fields[item.FieldCode]
		if !ok {
			return nil, apperrors.FieldNotFound(item.FieldCode)
		}
		if err := domain.ValidateFieldValue(field, item.ValueJSON); err != nil {
			return nil, err
		}

		var valueBytes []byte
		if domain.IsNullJSON(item.ValueJSON) {
			valueBytes = nil
		} else {
			valueBytes = append([]byte(nil), item.ValueJSON...)
		}

		resolved = append(resolved, repository.ResolvedCustomFieldValue{
			FieldID:   field.ID,
			FieldCode: field.Code,
			ValueJSON: valueBytes,
		})
	}

	saved, err := s.values.UpsertBatch(ctx, input, resolved)
	if err != nil {
		return nil, err
	}

	return &domain.UpsertCustomFieldValuesResult{
		TenantID:   input.TenantID,
		EntityType: input.EntityType,
		EntityID:   input.EntityID,
		SavedCount: saved,
	}, nil
}

func toEntityTypeInvalid(err error) error {
	if appErr, ok := err.(*apperrors.AppError); ok && appErr.Code == apperrors.CodeValidation {
		return apperrors.EntityTypeInvalid(appErr.Details["entity_type"].(string))
	}
	return err
}
