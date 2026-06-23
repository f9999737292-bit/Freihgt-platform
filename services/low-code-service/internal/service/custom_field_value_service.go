package service

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/low-code-service/internal/domain"
	apperrors "github.com/freight-platform/low-code-service/internal/platform/errors"
	"github.com/freight-platform/low-code-service/internal/repository"
)

type FormTemplateReader interface {
	GetPublishedTemplateContext(ctx context.Context, tenantID uuid.UUID, templateID uuid.UUID) (*domain.PublishedTemplateContext, error)
	ListActivePublished(ctx context.Context, tenantID uuid.UUID, entityType string, code string) ([]domain.FormTemplateSummary, error)
}

type CustomFieldValueStore interface {
	ListByEntity(ctx context.Context, tenantID uuid.UUID, entityType string, entityID uuid.UUID) ([]domain.CustomFieldValue, error)
	UpsertBatch(ctx context.Context, input domain.UpsertCustomFieldValuesInput, values []repository.ResolvedCustomFieldValue) (int, error)
	ReplaceFieldCodesBatch(
		ctx context.Context,
		input domain.UpsertCustomFieldValuesInput,
		fieldCodes []string,
		values []repository.ResolvedCustomFieldValue,
	) (int, error)
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

	existing, err := s.values.ListByEntity(ctx, input.TenantID, input.EntityType, input.EntityID)
	if err != nil {
		return nil, err
	}
	merged := domain.BuildValueSnapshot(existing, input.Values)
	if err := domain.ValidateConditionalRequiredFields(tmpl.Fields, merged, input.ValidationContext); err != nil {
		return nil, err
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

func (s *CustomFieldValueService) MigrateToActiveTemplate(
	ctx context.Context,
	input domain.MigrateCustomFieldValuesToActiveInput,
) (*domain.MigrateCustomFieldValuesToActiveResult, error) {
	if err := domain.ValidateEntityType(input.EntityType); err != nil {
		return nil, toEntityTypeInvalid(err)
	}
	if input.EntityID == uuid.Nil {
		return nil, apperrors.EntityIDInvalid(map[string]any{"field": "entity_id"})
	}

	templateCode := strings.TrimSpace(input.TemplateCode)
	if templateCode == "" {
		templateCode = strings.TrimSpace(input.Code)
	}

	targetTemplate, err := s.resolveMigrationPreviewTarget(ctx, domain.MigrationPreviewInput{
		TenantID:         input.TenantID,
		EntityType:       input.EntityType,
		TemplateCode:     templateCode,
		TargetTemplateID: input.TargetTemplateID,
	})
	if err != nil {
		return nil, err
	}
	if targetTemplate.EntityType != input.EntityType {
		return nil, apperrors.Validation("entity_type does not match form template", map[string]any{
			"entity_type":          input.EntityType,
			"template_entity_type": targetTemplate.EntityType,
		})
	}

	existing, err := s.values.ListByEntity(ctx, input.TenantID, input.EntityType, input.EntityID)
	if err != nil {
		return nil, err
	}

	sourceTemplateID := domain.InferSourceTemplateID(existing)
	sourceFields := map[string]domain.FieldDefinition{}
	if sourceTemplateID != uuid.Nil && sourceTemplateID != targetTemplate.ID {
		sourceCtx, err := s.templates.GetPublishedTemplateContext(ctx, input.TenantID, sourceTemplateID)
		if err == nil && sourceCtx != nil {
			sourceFields = sourceCtx.Fields
		}
	}

	previewItem := domain.BuildMigrationPreviewItem(input.EntityID, sourceTemplateID, *targetTemplate, existing, sourceFields)
	targetMeta := domain.MigrationPreviewTargetTemplate{
		ID:      targetTemplate.ID,
		Code:    targetTemplate.Code,
		Version: targetTemplate.Version,
	}
	previewMap := domain.MigrationPreviewItemToMap(previewItem, targetMeta, input.EntityType, input.TenantID)

	switch previewItem.Status {
	case domain.MigrationPreviewStatusBlocked:
		return nil, apperrors.MigrationBlocked("migration is blocked by incompatible fields", previewMap)
	case domain.MigrationPreviewStatusWarning:
		if !input.AllowWarnings {
			return nil, apperrors.MigrationWarningsRequireConfirmation("migration has warnings and requires allow_warnings=true", previewMap)
		}
	}

	if len(previewItem.CopiedFields) == 0 {
		return buildMigrateResult(previewItem, targetTemplate.ID, 0), nil
	}

	resolvedDomain := domain.BuildResolvedMigrationValues(previewItem, existing, *targetTemplate, sourceFields)
	resolved := make([]repository.ResolvedCustomFieldValue, 0, len(resolvedDomain))
	for _, item := range resolvedDomain {
		resolved = append(resolved, repository.ResolvedCustomFieldValue{
			FieldID:   item.FieldID,
			FieldCode: item.FieldCode,
			ValueJSON: item.ValueJSON,
		})
	}

	saved, err := s.values.ReplaceFieldCodesBatch(ctx, domain.UpsertCustomFieldValuesInput{
		TenantID:          input.TenantID,
		EntityType:        input.EntityType,
		EntityID:          input.EntityID,
		FormTemplateID:    targetTemplate.ID,
		ValidationContext: input.ValidationContext,
		Audit:             input.Audit,
		MigrationAudit: &domain.MigrateToActiveMigrationAudit{
			SourceTemplateID: sourceTemplateID,
			AllowWarnings:    input.AllowWarnings,
			SkipBlocked:      input.SkipBlocked,
			BatchID:          input.BatchID,
			TemplateCode:     templateCode,
			PreviewItem:      previewItem,
		},
	}, previewItem.CopiedFields, resolved)
	if err != nil {
		return nil, err
	}

	return buildMigrateResult(previewItem, targetTemplate.ID, saved), nil
}

func buildMigrateResult(previewItem domain.MigrationPreviewItem, targetTemplateID uuid.UUID, migratedCount int) *domain.MigrateCustomFieldValuesToActiveResult {
	status := "migrated"
	if previewItem.Status == domain.MigrationPreviewStatusWarning {
		status = "migrated_with_warnings"
	}

	skipped := append([]string{}, previewItem.LegacyFields...)

	return &domain.MigrateCustomFieldValuesToActiveResult{
		Status:                status,
		ActiveTemplateID:      targetTemplateID,
		TargetTemplateID:      targetTemplateID,
		SourceTemplateID:      previewItem.SourceTemplateID,
		MigratedCount:         migratedCount,
		SkippedCount:          len(skipped),
		SkippedFields:         skipped,
		CopiedFields:          previewItem.CopiedFields,
		LegacyFields:          previewItem.LegacyFields,
		MissingRequiredFields: previewItem.MissingRequiredFields,
		IncompatibleFields:    previewItem.IncompatibleFields,
		Warnings:              previewItem.Warnings,
	}
}

func (s *CustomFieldValueService) PreviewMigrationToActive(
	ctx context.Context,
	input domain.MigrationPreviewInput,
) (*domain.MigrationPreviewResult, error) {
	if err := domain.ValidateEntityType(input.EntityType); err != nil {
		return nil, toEntityTypeInvalid(err)
	}
	input.EntityIDs = domain.NormalizeBatchEntityIDs(input.EntityIDs)
	if len(input.EntityIDs) == 0 {
		return nil, apperrors.Validation("entity_ids must not be empty", map[string]any{"field": "entity_ids"})
	}
	if len(input.EntityIDs) > domain.MaxMigrationPreviewEntityCount {
		return nil, apperrors.Validation("entity_ids exceeds maximum batch size", map[string]any{
			"field": "entity_ids",
			"max":   domain.MaxMigrationPreviewEntityCount,
		})
	}

	targetTemplate, err := s.resolveMigrationPreviewTarget(ctx, input)
	if err != nil {
		return nil, err
	}
	if targetTemplate.EntityType != input.EntityType {
		return nil, apperrors.Validation("entity_type does not match form template", map[string]any{
			"entity_type":          input.EntityType,
			"template_entity_type": targetTemplate.EntityType,
		})
	}

	result := &domain.MigrationPreviewResult{
		TenantID:   input.TenantID,
		EntityType: input.EntityType,
		TargetTemplate: domain.MigrationPreviewTargetTemplate{
			ID:      targetTemplate.ID,
			Code:    targetTemplate.Code,
			Version: targetTemplate.Version,
		},
		Items: make([]domain.MigrationPreviewItem, 0, len(input.EntityIDs)),
	}

	for _, entityID := range input.EntityIDs {
		if entityID == uuid.Nil {
			return nil, apperrors.EntityIDInvalid(map[string]any{"field": "entity_id"})
		}

		existing, err := s.values.ListByEntity(ctx, input.TenantID, input.EntityType, entityID)
		if err != nil {
			return nil, err
		}

		sourceTemplateID := domain.InferSourceTemplateID(existing)
		sourceFields := map[string]domain.FieldDefinition{}
		if sourceTemplateID != uuid.Nil && sourceTemplateID != targetTemplate.ID {
			sourceCtx, err := s.templates.GetPublishedTemplateContext(ctx, input.TenantID, sourceTemplateID)
			if err == nil && sourceCtx != nil {
				sourceFields = sourceCtx.Fields
			}
		}

		item := domain.BuildMigrationPreviewItem(entityID, sourceTemplateID, *targetTemplate, existing, sourceFields)
		result.Items = append(result.Items, item)
	}

	result.Summary = summarizeMigrationPreview(result.Items)
	return result, nil
}

func (s *CustomFieldValueService) resolveMigrationPreviewTarget(
	ctx context.Context,
	input domain.MigrationPreviewInput,
) (*domain.PublishedTemplateContext, error) {
	if input.TargetTemplateID != uuid.Nil {
		return s.templates.GetPublishedTemplateContext(ctx, input.TenantID, input.TargetTemplateID)
	}

	code := strings.TrimSpace(input.TemplateCode)
	activeItems, err := s.templates.ListActivePublished(ctx, input.TenantID, input.EntityType, code)
	if err != nil {
		return nil, err
	}
	if len(activeItems) == 0 {
		return nil, apperrors.FormTemplateNotFound()
	}
	if code == "" && len(activeItems) > 1 {
		return nil, apperrors.Validation("template_code is required when multiple active templates exist", map[string]any{
			"field": "template_code",
		})
	}

	activeTemplateID := activeItems[0].ID
	return s.templates.GetPublishedTemplateContext(ctx, input.TenantID, activeTemplateID)
}

func summarizeMigrationPreview(items []domain.MigrationPreviewItem) domain.MigrationPreviewSummary {
	summary := domain.MigrationPreviewSummary{EntitiesChecked: len(items)}
	for _, item := range items {
		switch item.Status {
		case domain.MigrationPreviewStatusSafe:
			summary.SafeToMigrate++
		case domain.MigrationPreviewStatusWarning:
			summary.Warnings++
		case domain.MigrationPreviewStatusBlocked:
			summary.Blocked++
		}
	}
	return summary
}

func toEntityTypeInvalid(err error) error {
	if appErr, ok := err.(*apperrors.AppError); ok && appErr.Code == apperrors.CodeValidation {
		return apperrors.EntityTypeInvalid(appErr.Details["entity_type"].(string))
	}
	return err
}
