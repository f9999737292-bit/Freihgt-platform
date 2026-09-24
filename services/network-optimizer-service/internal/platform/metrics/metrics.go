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
	PredictionResults = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "bno_capacity_predictions_total",
		Help: "Rule-based capacity prediction outcomes. Confidence is a rule score, not a calibrated probability.",
	}, []string{"result", "reason"})
)

func Prediction(result, reason string) {
	PredictionResults.WithLabelValues(result, reason).Inc()
}

func API(operation, result string) {
	APIRequests.WithLabelValues(operation, result).Inc()
}
