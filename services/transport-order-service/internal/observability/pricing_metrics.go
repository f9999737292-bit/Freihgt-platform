package observability

import (
	"github.com/freight-platform/shared-go/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

type PricingMetrics struct {
	snapshotPersistTotal      *prometheus.CounterVec
	snapshotPersistFailure    *prometheus.CounterVec
	toPricingResolutionTotal  *prometheus.CounterVec
}

func NewPricingMetrics(serviceName string) *PricingMetrics {
	return newPricingMetrics(prometheus.DefaultRegisterer, serviceName)
}

func newPricingMetrics(reg prometheus.Registerer, serviceName string) *PricingMetrics {
	namespace := metrics.PrometheusNamespace(serviceName)
	m := &PricingMetrics{
		snapshotPersistTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "snapshot_persist_total",
			Help:      "Transport order rate snapshot persistence outcomes",
		}, []string{"result"}),
		snapshotPersistFailure: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "snapshot_persist_failure_total",
			Help:      "Transport order rate snapshot persistence failures by reason",
		}, []string{"reason"}),
		toPricingResolutionTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "to_pricing_resolution_total",
			Help:      "Transport order priced create resolution outcomes",
		}, []string{"result"}),
	}
	reg.MustRegister(m.snapshotPersistTotal, m.snapshotPersistFailure, m.toPricingResolutionTotal)
	return m
}

func (m *PricingMetrics) IncSnapshotPersist(result string) {
	if m == nil {
		return
	}
	if result == "" {
		result = "UNKNOWN"
	}
	m.snapshotPersistTotal.WithLabelValues(result).Inc()
}

func (m *PricingMetrics) IncSnapshotPersistFailure(reason string) {
	if m == nil {
		return
	}
	if reason == "" {
		reason = "UNKNOWN"
	}
	m.snapshotPersistFailure.WithLabelValues(reason).Inc()
}

func (m *PricingMetrics) IncTOPricingResolution(result string) {
	if m == nil {
		return
	}
	if result == "" {
		result = "UNKNOWN"
	}
	m.toPricingResolutionTotal.WithLabelValues(result).Inc()
}
