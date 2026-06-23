package metrics

import (
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/client_golang/prometheus"
)

func TestBatchMigrationMetricsUseBoundedLabels(t *testing.T) {
	allowed := map[string]map[string]struct{}{
		"lowcode_batch_migration_preview_total":         {"entity_type": {}, "status": {}},
		"lowcode_batch_migration_execute_total":         {"entity_type": {}, "status": {}},
		"lowcode_batch_migration_entities_total":        {"entity_type": {}, "operation": {}, "status": {}},
		"lowcode_batch_migration_blocked_total":         {"entity_type": {}},
		"lowcode_batch_migration_failed_total":          {"entity_type": {}},
		"lowcode_batch_migration_duration_seconds":      {"entity_type": {}, "operation": {}, "status": {}},
	}
	forbidden := []string{"tenant_id", "entity_id", "batch_id", "value_json"}

	ObserveBatchMigrationPreview("TRANSPORT_ORDER", "success")
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}

	for _, family := range families {
		expectedLabels, ok := allowed[family.GetName()]
		if !ok {
			continue
		}
		for _, metric := range family.GetMetric() {
			for _, label := range metric.GetLabel() {
				name := label.GetName()
				if _, allowed := expectedLabels[name]; !allowed {
					t.Fatalf("metric %s has unexpected label %q", family.GetName(), name)
				}
				for _, bad := range forbidden {
					if name == bad {
						t.Fatalf("metric %s must not use high-cardinality label %q", family.GetName(), bad)
					}
				}
			}
		}
	}
}

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
