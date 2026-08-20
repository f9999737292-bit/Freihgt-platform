package observability

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	resolutionTotal      *prometheus.CounterVec
	resolutionFailed     *prometheus.CounterVec
	resolutionAmbiguous  prometheus.Counter
	resolutionDuration   prometheus.Histogram
	versionActivation    *prometheus.CounterVec
}

func NewMetrics(serviceName string) *Metrics {
	m := &Metrics{
		resolutionTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: serviceName,
			Name:      "rate_resolution_total",
			Help:      "Rate resolution outcomes by status and pricing source",
		}, []string{"status", "source_type"}),
		resolutionFailed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: serviceName,
			Name:      "rate_resolution_failed_total",
			Help:      "Rate resolution failures by reason code",
		}, []string{"reason"}),
		resolutionAmbiguous: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: serviceName,
			Name:      "rate_resolution_ambiguous_total",
			Help:      "Ambiguous rate resolution outcomes",
		}),
		resolutionDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: serviceName,
			Name:      "rate_resolution_duration_seconds",
			Help:      "Rate resolution latency",
			Buckets:   prometheus.DefBuckets,
		}),
		versionActivation: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: serviceName,
			Name:      "rate_version_activation_total",
			Help:      "Rate version activation outcomes",
		}, []string{"result"}),
	}
	prometheus.MustRegister(
		m.resolutionTotal,
		m.resolutionFailed,
		m.resolutionAmbiguous,
		m.resolutionDuration,
		m.versionActivation,
	)
	return m
}

func (m *Metrics) ObserveResolution(start time.Time, status, sourceType, reason string) {
	m.resolutionDuration.Observe(time.Since(start).Seconds())
	if status == "" {
		status = "UNKNOWN"
	}
	source := sourceType
	if source == "" {
		source = "NONE"
	}
	m.resolutionTotal.WithLabelValues(status, source).Inc()
	if status == "AMBIGUOUS" {
		m.resolutionAmbiguous.Inc()
	}
	if reason != "" {
		m.resolutionFailed.WithLabelValues(reason).Inc()
	}
}

func (m *Metrics) IncVersionActivation(result string) {
	if result == "" {
		result = "UNKNOWN"
	}
	m.versionActivation.WithLabelValues(result).Inc()
}
