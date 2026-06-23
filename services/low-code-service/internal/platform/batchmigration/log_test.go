package batchmigration

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/freight-platform/low-code-service/internal/domain"
)

func TestLogExecuteDoesNotIncludeSensitiveFields(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	defer slog.SetDefault(prev)

	LogExecute(
		uuid.New(),
		"TRANSPORT_ORDER",
		"transport_order_default",
		"req-1",
		"completed",
		domain.BatchMigrateSummary{Total: 2, Migrated: 1, Skipped: 1},
	)

	output := buf.String()
	for _, forbidden := range []string{"value_json", "password", "secret", "personal_data"} {
		if strings.Contains(strings.ToLower(output), forbidden) {
			t.Fatalf("structured log must not include %q, got: %s", forbidden, output)
		}
	}
	if !strings.Contains(output, "batch_id") || !strings.Contains(output, "batch_migration_execute") {
		t.Fatalf("expected safe batch execute log fields, got: %s", output)
	}
}

func TestLogPreviewDoesNotIncludeSensitiveFields(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	defer slog.SetDefault(prev)

	LogPreview(
		uuid.Nil,
		"TRANSPORT_ORDER",
		"transport_order_default",
		"req-2",
		"success",
		domain.MigrationPreviewSummary{EntitiesChecked: 1, SafeToMigrate: 1},
	)

	output := buf.String()
	for _, forbidden := range []string{"value_json", "password", "secret"} {
		if strings.Contains(strings.ToLower(output), forbidden) {
			t.Fatalf("structured log must not include %q, got: %s", forbidden, output)
		}
	}
}
