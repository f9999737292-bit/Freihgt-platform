package legacyaggregate

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	FallbackLevelNone          = "NONE"
	FallbackLevelFullAggregate = "FULL_AGGREGATE"
	FallbackLevelPageLimited   = "PAGE_LIMITED"
)

type Metrics struct {
	requestsTotal   *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	errorsTotal     *prometheus.CounterVec
	fallbackTotal   *prometheus.CounterVec
}

var registerOnce sync.Once

func NewMetrics() *Metrics {
	m := &Metrics{
		requestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "control_tower_legacy_status_aggregate_requests_total",
			Help: "Legacy full status aggregate requests from API Gateway.",
		}, []string{"result", "reason", "mode"}),
		requestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "control_tower_legacy_status_aggregate_duration_seconds",
			Help:    "Legacy full status aggregate request duration.",
			Buckets: prometheus.DefBuckets,
		}, []string{"result", "reason", "mode"}),
		errorsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "control_tower_legacy_status_aggregate_errors_total",
			Help: "Legacy full status aggregate dependency errors.",
		}, []string{"result", "error_code"}),
		fallbackTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "control_tower_legacy_status_fallback_total",
			Help: "Control Tower legacy status fallback levels.",
		}, []string{"mode", "fallback_level", "reason"}),
	}
	registerOnce.Do(func() {
		prometheus.MustRegister(
			m.requestsTotal,
			m.requestDuration,
			m.errorsTotal,
			m.fallbackTotal,
		)
	})
	return m
}

func (m *Metrics) ObserveRequest(mode, result, reason, fallbackLevel string, duration time.Duration) {
	if reason == "" {
		reason = "NONE"
	}
	if fallbackLevel == "" {
		fallbackLevel = FallbackLevelNone
	}
	m.requestsTotal.WithLabelValues(result, reason, mode).Inc()
	m.requestDuration.WithLabelValues(result, reason, mode).Observe(duration.Seconds())
}

func (m *Metrics) ObserveError(result, errorCode string) {
	if errorCode == "" {
		errorCode = "UNKNOWN"
	}
	m.errorsTotal.WithLabelValues(result, errorCode).Inc()
}

func (m *Metrics) ObserveFallback(mode, fallbackLevel, reason string) {
	if reason == "" {
		reason = "UNKNOWN"
	}
	m.fallbackTotal.WithLabelValues(mode, fallbackLevel, reason).Inc()
}
