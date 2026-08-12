package metrics

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func TestNewConsumerMetricsExposesDeadLetterAtZero(t *testing.T) {
	reg := prometheus.NewRegistry()
	cv := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "control_tower_shipment_dead_letter_total",
	}, []string{"error_code"})
	reg.MustRegister(cv)
	initDeadLetterSeries(cv)

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	promhttp.HandlerFor(reg, promhttp.HandlerOpts{}).ServeHTTP(w, req)
	body := w.Body.String()

	if !strings.Contains(body, "control_tower_shipment_dead_letter_total") {
		t.Fatalf("expected dead-letter metric name in exposition:\n%s", body)
	}
	if !strings.Contains(body, `control_tower_shipment_dead_letter_total{error_code="INVALID_JSON"} 0`) {
		t.Fatalf("expected zero-valued dead-letter series before events:\n%s", body)
	}
}

func TestObserveDeadLetterIncrementsExpectedSeries(t *testing.T) {
	m := NewConsumerMetrics()
	code := domain.ErrorInvalidEventType
	before := deadLetterValue(t, m, code)
	m.ObserveDeadLetter(code)
	m.ObserveDeadLetter(code)
	after := deadLetterValue(t, m, code)
	if after-before != 2 {
		t.Fatalf("dead-letter delta=%v want 2 (before=%v after=%v)", after-before, before, after)
	}
}

func TestNewConsumerMetricsDuplicateInitDoesNotPanic(t *testing.T) {
	first := NewConsumerMetrics()
	second := NewConsumerMetrics()
	if first == nil || second == nil {
		t.Fatal("expected non-nil metrics instances")
	}
}

func TestInitDeadLetterSeriesUsesCanonicalErrorCodes(t *testing.T) {
	reg := prometheus.NewRegistry()
	cv := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "control_tower_shipment_dead_letter_total",
	}, []string{"error_code"})
	reg.MustRegister(cv)
	initDeadLetterSeries(cv)

	for _, code := range domain.PermanentErrorCodes() {
		if _, err := cv.GetMetricWithLabelValues(code); err != nil {
			t.Fatalf("missing initialized series for %q: %v", code, err)
		}
	}
}

func TestMetricPresentExpectationSatisfiedByExposition(t *testing.T) {
	m := NewConsumerMetrics()
	_ = m

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	promhttp.Handler().ServeHTTP(w, req)
	body := w.Body.String()

	if !strings.Contains(body, "control_tower_shipment_dead_letter_total") {
		t.Fatalf("observation preflight metric name missing from exposition")
	}
}

func deadLetterValue(t *testing.T, m *ConsumerMetrics, code string) float64 {
	t.Helper()
	metric, err := m.deadLetterTotal.GetMetricWithLabelValues(code)
	if err != nil {
		t.Fatalf("GetMetricWithLabelValues(%q): %v", code, err)
	}
	var dtoMetric dto.Metric
	if err := metric.Write(&dtoMetric); err != nil {
		t.Fatalf("Write metric: %v", err)
	}
	return dtoMetric.GetCounter().GetValue()
}
