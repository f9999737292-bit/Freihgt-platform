package observability

import (
	"github.com/freight-platform/shared-go/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

type SettlementMetrics struct {
	legacyPricingFallback prometheus.Counter
}

func NewSettlementMetrics(serviceName string) *SettlementMetrics {
	return newSettlementMetrics(prometheus.DefaultRegisterer, serviceName)
}

func newSettlementMetrics(reg prometheus.Registerer, serviceName string) *SettlementMetrics {
	namespace := metrics.PrometheusNamespace(serviceName)
	m := &SettlementMetrics{
		legacyPricingFallback: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "legacy_settlement_pricing_fallback_total",
			Help:      "Settlement principal loaded from legacy award link fallback",
		}),
	}
	reg.MustRegister(m.legacyPricingFallback)
	return m
}

func (m *SettlementMetrics) IncLegacyPricingFallback() {
	if m != nil {
		m.legacyPricingFallback.Inc()
	}
}
