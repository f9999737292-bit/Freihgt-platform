package ratesrbac

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
			Name: "contract_rate_public_requests_total",
			Help: "Public contract-rate gateway requests by operation and result.",
		}, []string{"operation", "result"})
		authzDeniedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "contract_rate_authz_denied_total",
			Help: "Contract-rate authorization denials by policy and reason.",
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
