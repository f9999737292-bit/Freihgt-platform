package metrics

import (
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/client_golang/prometheus"
)

func TestBatchMigrationMetricsRegistered(t *testing.T) {
	ObserveBatchMigrationPreview("TRANSPORT_ORDER", "success")
	ObserveBatchMigrationPreviewDuration("TRANSPORT_ORDER", "success", 10*time.Millisecond)
	ObserveBatchMigrationExecute("TRANSPORT_ORDER", "completed", BatchMigrationExecuteSummary{
		Migrated: 2,
		Skipped:  1,
		Blocked:  1,
		Failed:   0,
	}, 25*time.Millisecond)

	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}

	names := map[string]bool{}
	for _, family := range families {
		names[family.GetName()] = true
	}

	for _, expected := range []string{
		"lowcode_batch_migration_preview_total",
		"lowcode_batch_migration_execute_total",
		"lowcode_batch_migration_entities_total",
		"lowcode_batch_migration_blocked_total",
		"lowcode_batch_migration_duration_seconds",
	} {
		if !names[expected] {
			t.Fatalf("expected metric %s to be registered", expected)
		}
	}

	var previewCount float64
	for _, family := range families {
		if family.GetName() != "lowcode_batch_migration_preview_total" {
			continue
		}
		for _, metric := range family.GetMetric() {
			previewCount += metric.GetCounter().GetValue()
		}
	}
	if previewCount < 1 {
		t.Fatalf("expected preview counter increment, got %v", previewCount)
	}

	_ = dto.Metric{}
}
