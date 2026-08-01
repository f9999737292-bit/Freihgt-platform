package controltowerreadmodel

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	requestsTotal    *prometheus.CounterVec
	requestDuration  *prometheus.HistogramVec
	fallbackTotal    *prometheus.CounterVec
	shadowComparison *prometheus.CounterVec
	partialResponse  *prometheus.CounterVec
}

var registerOnce sync.Once

func NewMetrics() *Metrics {
	m := &Metrics{
		requestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "control_tower_read_model_requests_total",
			Help: "Read-model status-summary requests from API Gateway.",
		}, []string{"mode", "result", "reason"}),
		requestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "control_tower_read_model_request_duration_seconds",
			Help:    "Read-model status-summary request duration.",
			Buckets: prometheus.DefBuckets,
		}, []string{"mode", "result", "reason"}),
		fallbackTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "control_tower_read_model_fallback_total",
			Help: "Primary-mode fallbacks to legacy status summary.",
		}, []string{"mode", "reason"}),
		shadowComparison: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "control_tower_read_model_shadow_comparison_total",
			Help: "Shadow-mode legacy vs read-model comparisons.",
		}, []string{"mode", "comparison"}),
		partialResponse: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "control_tower_read_model_partial_response_total",
			Help: "Read-model responses with incomplete projections.",
		}, []string{"mode"}),
	}
	registerOnce.Do(func() {
		prometheus.MustRegister(
			m.requestsTotal,
			m.requestDuration,
			m.fallbackTotal,
			m.shadowComparison,
			m.partialResponse,
		)
	})
	return m
}

func (m *Metrics) ObserveRequest(mode, result, reason string, duration time.Duration) {
	if reason == "" {
		reason = "NONE"
	}
	m.requestsTotal.WithLabelValues(mode, result, reason).Inc()
	m.requestDuration.WithLabelValues(mode, result, reason).Observe(duration.Seconds())
}

func (m *Metrics) ObserveFallback(mode, reason string) {
	if reason == "" {
		reason = "UNKNOWN"
	}
	m.fallbackTotal.WithLabelValues(mode, reason).Inc()
}

func (m *Metrics) ObserveComparison(mode string, comparison ComparisonResult) {
	m.shadowComparison.WithLabelValues(mode, string(comparison)).Inc()
}

func (m *Metrics) ObservePartial(mode string) {
	m.partialResponse.WithLabelValues(mode).Inc()
}
