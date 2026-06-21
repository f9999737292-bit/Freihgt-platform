package metrics_test

import (
	"database/sql"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"github.com/freight-platform/shared-go/metrics"
)

func findMetricFamily(t *testing.T, name string) *dto.MetricFamily {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	for _, family := range families {
		if family.GetName() == name {
			return family
		}
	}
	t.Fatalf("metric family %q not found", name)
	return nil
}

func TestRegisterDBPoolMetrics(t *testing.T) {
	db, err := sql.Open("pgx", "postgres://localhost:5432/nonexistent?sslmode=disable")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	metrics.RegisterDBPoolMetrics("test-service", db)

	for _, metricName := range []string{
		"db_pool_open_connections",
		"db_pool_in_use_connections",
		"db_pool_idle_connections",
		"db_pool_max_open_connections",
		"db_pool_wait_count_total",
		"db_pool_wait_duration_seconds_total",
	} {
		family := findMetricFamily(t, metricName)
		if len(family.Metric) == 0 {
			t.Fatalf("expected samples for %s", metricName)
		}
	}
}

func TestRegisterDBPoolMetricsNoDuplicatePanic(t *testing.T) {
	db, err := sql.Open("pgx", "postgres://localhost:5432/nonexistent?sslmode=disable")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	metrics.RegisterDBPoolMetrics("dup-service", db)
	metrics.RegisterDBPoolMetrics("dup-service", db)
}
