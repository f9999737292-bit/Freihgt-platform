package observability

import (
	"time"

	"github.com/freight-platform/shared-go/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	resolutionTotal      *prometheus.CounterVec
	resolutionFailed     *prometheus.CounterVec
	pricingSourceTotal   *prometheus.CounterVec
	pricingSourceFailure *prometheus.CounterVec
	resolutionAmbiguous  prometheus.Counter
	resolutionDuration   prometheus.Histogram
	versionActivation    *prometheus.CounterVec
}

func NewMetrics(serviceName string) *Metrics {
	return newMetrics(prometheus.DefaultRegisterer, serviceName)
}

func newMetrics(reg prometheus.Registerer, serviceName string) *Metrics {
	namespace := metrics.PrometheusNamespace(serviceName)
	m := &Metrics{
		resolutionTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "rate_resolution_total",
			Help:      "Rate resolution outcomes by status and pricing source",
		}, []string{"status", "source_type"}),
		resolutionFailed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "rate_resolution_failed_total",
			Help:      "Rate resolution failures by reason code",
		}, []string{"reason"}),
		pricingSourceTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "pricing_source_total",
			Help:      "Successful pricing source resolutions by source type",
		}, []string{"source_type"}),
		pricingSourceFailure: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "pricing_source_failure_total",
			Help:      "Pricing source resolution failures by reason",
		}, []string{"reason"}),
		resolutionAmbiguous: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "rate_resolution_ambiguous_total",
			Help:      "Ambiguous rate resolution outcomes",
		}),
		resolutionDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "rate_resolution_duration_seconds",
			Help:      "Rate resolution latency",
			Buckets:   prometheus.DefBuckets,
		}),
		versionActivation: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "rate_version_activation_total",
			Help:      "Rate version activation outcomes",
		}, []string{"result"}),
	}
	reg.MustRegister(
		m.resolutionTotal,
		m.resolutionFailed,
		m.pricingSourceTotal,
		m.pricingSourceFailure,
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
	if status == "MATCHED" && source != "NONE" {
		m.pricingSourceTotal.WithLabelValues(source).Inc()
	}
	if status == "AMBIGUOUS" {
		m.resolutionAmbiguous.Inc()
	}
	if reason != "" {
		m.resolutionFailed.WithLabelValues(reason).Inc()
		m.pricingSourceFailure.WithLabelValues(reason).Inc()
	}
}

func (m *Metrics) IncVersionActivation(result string) {
	if result == "" {
		result = "UNKNOWN"
	}
	m.versionActivation.WithLabelValues(result).Inc()
}
