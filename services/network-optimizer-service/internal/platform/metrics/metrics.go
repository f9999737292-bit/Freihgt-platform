package metrics

import "github.com/prometheus/client_golang/prometheus"
import "github.com/prometheus/client_golang/prometheus/promauto"

var (
	LoadOpportunities = promauto.NewCounter(prometheus.CounterOpts{
		Name: "bno_load_opportunities_total",
		Help: "Load opportunities created.",
	})
	Capacities = promauto.NewCounter(prometheus.CounterOpts{
		Name: "bno_capacities_total",
		Help: "Capacities created.",
	})
	Publications = promauto.NewCounter(prometheus.CounterOpts{
		Name: "bno_publications_total",
		Help: "Explicit load or capacity publications.",
	})
	Withdrawals = promauto.NewCounter(prometheus.CounterOpts{
		Name: "bno_withdrawals_total",
		Help: "Load or capacity withdrawals.",
	})
	APIRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "bno_api_requests_total",
		Help: "Network optimizer API requests.",
	}, []string{"operation", "result"})
)

func API(operation, result string) {
	APIRequests.WithLabelValues(operation, result).Inc()
}
