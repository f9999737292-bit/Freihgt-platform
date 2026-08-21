package observability

import "github.com/prometheus/client_golang/prometheus"

type SettlementMetrics struct {
	legacyPricingFallback prometheus.Counter
}

func NewSettlementMetrics(serviceName string) *SettlementMetrics {
	m := &SettlementMetrics{
		legacyPricingFallback: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: serviceName,
			Name:      "legacy_settlement_pricing_fallback_total",
			Help:      "Settlement principal loaded from legacy award link fallback",
		}),
	}
	prometheus.MustRegister(m.legacyPricingFallback)
	return m
}

func (m *SettlementMetrics) IncLegacyPricingFallback() {
	if m != nil {
		m.legacyPricingFallback.Inc()
	}
}
