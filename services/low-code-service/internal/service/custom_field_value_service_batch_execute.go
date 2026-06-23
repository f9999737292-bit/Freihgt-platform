package service

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/low-code-service/internal/domain"
	apperrors "github.com/freight-platform/low-code-service/internal/platform/errors"
)

func (s *CustomFieldValueService) BatchMigrateToActiveTemplate(
	ctx context.Context,
	input domain.BatchMigrateCustomFieldValuesToActiveInput,
) (*domain.BatchMigrateCustomFieldValuesToActiveResult, error) {
	batchID := input.BatchID
	if batchID == uuid.Nil {
		batchID = uuid.New()
	}

	templateCode := strings.TrimSpace(input.TemplateCode)
	preview, err := s.PreviewMigrationToActive(ctx, domain.MigrationPreviewInput{
		TenantID:         input.TenantID,
		EntityType:       input.EntityType,
		EntityIDs:        input.EntityIDs,
		TemplateCode:     templateCode,
		TargetTemplateID: input.TargetTemplateID,
	})
	if err != nil {
		return nil, err
	}

	previewMap := domain.MigrationPreviewResultToBatchMap(preview, templateCode)

	if !input.SkipBlocked && preview.Summary.Blocked > 0 {
		return nil, apperrors.BatchMigrationBlocked("Batch contains blocked entities.", previewMap)
	}
	if !input.AllowWarnings && preview.Summary.Warnings > 0 && preview.Summary.SafeToMigrate == 0 {
		return nil, apperrors.BatchMigrationWarningsRequireConfirmation(
			"Batch contains warning entities and requires allow_warnings=true.",
			previewMap,
		)
	}

	resolvedTemplateCode := templateCode
	if resolvedTemplateCode == "" {
		resolvedTemplateCode = preview.TargetTemplate.Code
	}

	result := &domain.BatchMigrateCustomFieldValuesToActiveResult{
		BatchID:      batchID,
		TenantID:     input.TenantID,
		EntityType:   input.EntityType,
		TemplateCode: resolvedTemplateCode,
		TargetTemplate: domain.MigrationPreviewTargetTemplate{
			ID:      preview.TargetTemplate.ID,
			Code:    preview.TargetTemplate.Code,
			Version: preview.TargetTemplate.Version,
		},
		Summary: domain.BatchMigrateSummary{Total: len(preview.Items)},
		Items:   make([]domain.BatchMigrateItemResult, 0, len(preview.Items)),
	}

	for _, previewItem := range preview.Items {
		switch previewItem.Status {
		case domain.MigrationPreviewStatusBlocked:
			result.Items = append(result.Items, buildBatchSkippedItem(previewItem, domain.BatchMigrateSkipReasonBlocked))
			result.Summary.Skipped++
			result.Summary.Blocked++
			continue
		case domain.MigrationPreviewStatusWarning:
			if !input.AllowWarnings {
				result.Items = append(result.Items, buildBatchSkippedItem(previewItem, domain.BatchMigrateSkipReasonWarningsRequireConfirmation))
				result.Summary.Skipped++
				result.Summary.Warnings++
				continue
			}
		}

		migrateResult, err := s.MigrateToActiveTemplate(ctx, domain.MigrateCustomFieldValuesToActiveInput{
			TenantID:          input.TenantID,
			EntityType:        input.EntityType,
			EntityID:          previewItem.EntityID,
			TemplateCode:      templateCode,
			TargetTemplateID:  preview.TargetTemplate.ID,
			AllowWarnings:     input.AllowWarnings,
			SkipBlocked:       input.SkipBlocked,
			BatchID:           batchID,
			ValidationContext: input.ValidationContext,
			Audit:             input.Audit,
		})
		if err != nil {
			result.Items = append(result.Items, buildBatchFailedItem(previewItem, err))
			result.Summary.Failed++
			continue
		}

		result.Items = append(result.Items, buildBatchMigratedItem(previewItem, migrateResult))
		result.Summary.Migrated++
		if previewItem.Status == domain.MigrationPreviewStatusWarning {
			result.Summary.Warnings++
		}
	}

	result.Status = resolveBatchMigrateStatus(result.Summary)
	return result, nil
}

func buildBatchSkippedItem(previewItem domain.MigrationPreviewItem, reason string) domain.BatchMigrateItemResult {
	return domain.BatchMigrateItemResult{
		EntityID:              previewItem.EntityID,
		Status:                domain.BatchMigrateItemStatusSkipped,
		PreviewStatus:         previewItem.Status,
		Reason:                reason,
		CopiedFields:          previewItem.CopiedFields,
		LegacyFields:          previewItem.LegacyFields,
		MissingRequiredFields: previewItem.MissingRequiredFields,
		IncompatibleFields:    previewItem.IncompatibleFields,
		Warnings:              previewItem.Warnings,
	}
}

func buildBatchFailedItem(previewItem domain.MigrationPreviewItem, err error) domain.BatchMigrateItemResult {
	return domain.BatchMigrateItemResult{
		EntityID:              previewItem.EntityID,
		Status:                domain.BatchMigrateItemStatusFailed,
		PreviewStatus:         previewItem.Status,
		Reason:                err.Error(),
		CopiedFields:          previewItem.CopiedFields,
		LegacyFields:          previewItem.LegacyFields,
		MissingRequiredFields: previewItem.MissingRequiredFields,
		IncompatibleFields:    previewItem.IncompatibleFields,
		Warnings:              previewItem.Warnings,
	}
}

func buildBatchMigratedItem(previewItem domain.MigrationPreviewItem, migrateResult *domain.MigrateCustomFieldValuesToActiveResult) domain.BatchMigrateItemResult {
	status := domain.BatchMigrateItemStatusMigrated
	if migrateResult.Status == "migrated_with_warnings" {
		status = domain.BatchMigrateItemStatusMigratedWithWarnings
	}
	return domain.BatchMigrateItemResult{
		EntityID:              previewItem.EntityID,
		Status:                status,
		PreviewStatus:         previewItem.Status,
		MigratedCount:         migrateResult.MigratedCount,
		CopiedFields:          migrateResult.CopiedFields,
		LegacyFields:          migrateResult.LegacyFields,
		MissingRequiredFields: migrateResult.MissingRequiredFields,
		IncompatibleFields:    migrateResult.IncompatibleFields,
		Warnings:              migrateResult.Warnings,
	}
}

func resolveBatchMigrateStatus(summary domain.BatchMigrateSummary) string {
	if summary.Migrated > 0 && summary.Skipped == 0 && summary.Failed == 0 {
		return domain.BatchMigrateStatusCompleted
	}
	if summary.Migrated > 0 {
		return domain.BatchMigrateStatusPartiallyCompleted
	}
	if summary.Blocked > 0 && summary.Blocked == summary.Total {
		return domain.BatchMigrateStatusBlocked
	}
	return domain.BatchMigrateStatusFailed
}
