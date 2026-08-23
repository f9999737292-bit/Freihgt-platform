package freightcostrbac

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	metricsOnce sync.Once

	publicRequestsTotal *prometheus.CounterVec
	authzDeniedTotal    *prometheus.CounterVec
)

func initMetrics() {
	metricsOnce.Do(func() {
		publicRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "freight_cost_public_requests_total",
			Help: "Public freight-cost gateway requests by operation and result.",
		}, []string{"operation", "result"})
		authzDeniedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "freight_cost_authz_denied_total",
			Help: "Freight-cost authorization denials by policy and reason.",
		}, []string{"policy", "reason"})
		prometheus.MustRegister(publicRequestsTotal, authzDeniedTotal)
	})
}

func recordPublicRequest(operation, result string) {
	initMetrics()
	publicRequestsTotal.WithLabelValues(operation, result).Inc()
}

func recordAuthzDenied(policy, reason string) {
	initMetrics()
	authzDeniedTotal.WithLabelValues(policy, reason).Inc()
}
