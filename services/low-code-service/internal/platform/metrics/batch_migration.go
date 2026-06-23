package metrics

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Batch migration metrics intentionally use bounded labels only (entity_type, operation, status).
// Never add tenant_id, entity_id, batch_id, or other high-cardinality dimensions.
var (
	batchMigrationMetricsOnce sync.Once

	batchMigrationPreviewTotal   *prometheus.CounterVec
	batchMigrationExecuteTotal   *prometheus.CounterVec
	batchMigrationEntitiesTotal  *prometheus.CounterVec
	batchMigrationBlockedTotal   *prometheus.CounterVec
	batchMigrationFailedTotal    *prometheus.CounterVec
	batchMigrationDurationSeconds *prometheus.HistogramVec
)

func initBatchMigrationMetrics() {
	batchMigrationMetricsOnce.Do(func() {
		batchMigrationPreviewTotal = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "lowcode_batch_migration_preview_total",
				Help: "Total batch migration preview operations",
			},
			[]string{"entity_type", "status"},
		)
		batchMigrationExecuteTotal = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "lowcode_batch_migration_execute_total",
				Help: "Total batch migration execute operations",
			},
			[]string{"entity_type", "status"},
		)
		batchMigrationEntitiesTotal = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "lowcode_batch_migration_entities_total",
				Help: "Total entities processed in batch migration execute",
			},
			[]string{"entity_type", "operation", "status"},
		)
		batchMigrationBlockedTotal = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "lowcode_batch_migration_blocked_total",
				Help: "Total entities blocked during batch migration execute",
			},
			[]string{"entity_type"},
		)
		batchMigrationFailedTotal = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "lowcode_batch_migration_failed_total",
				Help: "Total entities failed during batch migration execute",
			},
			[]string{"entity_type"},
		)
		batchMigrationDurationSeconds = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "lowcode_batch_migration_duration_seconds",
				Help:    "Batch migration operation duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"entity_type", "operation", "status"},
		)
		prometheus.MustRegister(
			batchMigrationPreviewTotal,
			batchMigrationExecuteTotal,
			batchMigrationEntitiesTotal,
			batchMigrationBlockedTotal,
			batchMigrationFailedTotal,
			batchMigrationDurationSeconds,
		)
	})
}

func ObserveBatchMigrationPreview(entityType, status string) {
	initBatchMigrationMetrics()
	batchMigrationPreviewTotal.WithLabelValues(entityType, status).Inc()
}

func ObserveBatchMigrationExecute(entityType, status string, summary BatchMigrationExecuteSummary, duration time.Duration) {
	initBatchMigrationMetrics()
	batchMigrationExecuteTotal.WithLabelValues(entityType, status).Inc()
	batchMigrationDurationSeconds.WithLabelValues(entityType, "execute", status).Observe(duration.Seconds())

	if summary.Migrated > 0 {
		batchMigrationEntitiesTotal.WithLabelValues(entityType, "execute", "migrated").Add(float64(summary.Migrated))
	}
	if summary.Skipped > 0 {
		batchMigrationEntitiesTotal.WithLabelValues(entityType, "execute", "skipped").Add(float64(summary.Skipped))
	}
	if summary.Blocked > 0 {
		batchMigrationBlockedTotal.WithLabelValues(entityType).Add(float64(summary.Blocked))
		batchMigrationEntitiesTotal.WithLabelValues(entityType, "execute", "blocked").Add(float64(summary.Blocked))
	}
	if summary.Failed > 0 {
		batchMigrationFailedTotal.WithLabelValues(entityType).Add(float64(summary.Failed))
		batchMigrationEntitiesTotal.WithLabelValues(entityType, "execute", "failed").Add(float64(summary.Failed))
	}
}

func ObserveBatchMigrationPreviewDuration(entityType, status string, duration time.Duration) {
	initBatchMigrationMetrics()
	batchMigrationDurationSeconds.WithLabelValues(entityType, "preview", status).Observe(duration.Seconds())
}

type BatchMigrationExecuteSummary struct {
	Migrated int
	Skipped  int
	Blocked  int
	Failed   int
}
