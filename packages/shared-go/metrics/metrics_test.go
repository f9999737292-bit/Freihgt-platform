package metrics_test

import (
	"errors"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/freight-platform/shared-go/metrics"
)

func findDBQueryMetricFamily(t *testing.T) *dto.MetricFamily {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	for _, family := range families {
		if family.GetName() == "db_query_duration_seconds" {
			return family
		}
	}
	t.Fatal("db_query_duration_seconds metric family not found")
	return nil
}

func hasLabel(metric *dto.Metric, key, value string) bool {
	for _, lp := range metric.Label {
		if lp.GetName() == key && lp.GetValue() == value {
			return true
		}
	}
	return false
}

func TestMeasureDBQueryRecordsSuccessAndError(t *testing.T) {
	err := metrics.MeasureDBQuery("company-service", "company_repository", "list_companies", func() error {
		time.Sleep(time.Millisecond)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = metrics.MeasureDBQuery("company-service", "company_repository", "get_company", func() error {
		return errors.New("db failure")
	})
	if err == nil {
		t.Fatal("expected error")
	}

	family := findDBQueryMetricFamily(t)
	if family.GetType() != dto.MetricType_HISTOGRAM {
		t.Fatalf("expected histogram, got %v", family.GetType())
	}

	foundSuccess := false
	foundError := false
	for _, metric := range family.Metric {
		if hasLabel(metric, "operation", "list_companies") && hasLabel(metric, "status", "success") {
			if metric.Histogram != nil && metric.Histogram.GetSampleCount() > 0 {
				foundSuccess = true
			}
		}
		if hasLabel(metric, "operation", "get_company") && hasLabel(metric, "status", "error") {
			if metric.Histogram != nil && metric.Histogram.GetSampleCount() > 0 {
				foundError = true
			}
		}
	}
	if !foundSuccess {
		t.Fatal("expected list_companies success histogram sample")
	}
	if !foundError {
		t.Fatal("expected get_company error histogram sample")
	}
}

func TestObserveDBQuerySlowQueryThreshold(t *testing.T) {
	metrics.ObserveDBQuery(
		"company-service",
		"company_repository",
		"list_companies",
		"success",
		600*time.Millisecond,
	)
}
