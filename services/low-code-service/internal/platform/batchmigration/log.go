package batchmigration

import (
	"log/slog"

	"github.com/google/uuid"

	"github.com/freight-platform/low-code-service/internal/domain"
)

func LogPreview(batchID uuid.UUID, entityType, templateCode, requestID, status string, summary domain.MigrationPreviewSummary) {
	attrs := []any{
		slog.String("operation", "batch_migration_preview"),
		slog.String("entity_type", entityType),
		slog.String("template_code", templateCode),
		slog.String("status", status),
		slog.Int("total", summary.EntitiesChecked),
		slog.Int("safe", summary.SafeToMigrate),
		slog.Int("warnings", summary.Warnings),
		slog.Int("blocked", summary.Blocked),
	}
	if batchID != uuid.Nil {
		attrs = append(attrs, slog.String("batch_id", batchID.String()))
	}
	if requestID != "" {
		attrs = append(attrs, slog.String("request_id", requestID))
	}
	slog.Info("batch migration preview", attrs...)
}

func LogExecute(batchID uuid.UUID, entityType, templateCode, requestID, status string, summary domain.BatchMigrateSummary) {
	attrs := []any{
		slog.String("operation", "batch_migration_execute"),
		slog.String("entity_type", entityType),
		slog.String("template_code", templateCode),
		slog.String("status", status),
		slog.Int("total", summary.Total),
		slog.Int("migrated", summary.Migrated),
		slog.Int("skipped", summary.Skipped),
		slog.Int("blocked", summary.Blocked),
		slog.Int("failed", summary.Failed),
		slog.Int("warnings", summary.Warnings),
	}
	if batchID != uuid.Nil {
		attrs = append(attrs, slog.String("batch_id", batchID.String()))
	}
	if requestID != "" {
		attrs = append(attrs, slog.String("request_id", requestID))
	}
	slog.Info("batch migration execute", attrs...)
}
