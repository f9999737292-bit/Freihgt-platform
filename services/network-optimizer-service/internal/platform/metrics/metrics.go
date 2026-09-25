package metrics

import "time"

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

var (
	SearchRunsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "bno_search_runs_total",
		Help: "Next-load candidate searches.",
	})
	CandidatePoolSize = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "bno_candidate_pool_size", Help: "Visible loads considered by a search.",
		Buckets: []float64{1, 5, 10, 25, 50, 100, 250},
	})
	EligibleCandidates = promauto.NewCounter(prometheus.CounterOpts{
		Name: "bno_eligible_candidates_total", Help: "Eligible next-load candidates.",
	})
	RejectedCandidates = promauto.NewCounter(prometheus.CounterOpts{
		Name: "bno_rejected_candidates_total", Help: "Rejected next-load candidates.",
	})
	RejectionsByReason = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "bno_rejections_by_reason_total", Help: "Rejected candidates by bounded reason code.",
	}, []string{"reason"})
	RoutingErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "bno_routing_errors_total", Help: "Routing provider failures during search.",
	})
	MatrixBatchCount = promauto.NewCounter(prometheus.CounterOpts{
		Name: "bno_matrix_batches_total", Help: "Synchronous distance-matrix batches.",
	})
	SearchDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "bno_search_duration_seconds", Help: "Next-load search duration.",
		Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 5},
	})
)

func SearchPool(size int) { CandidatePoolSize.Observe(float64(size)) }

func RoutingError() { RoutingErrors.Inc() }

func MatrixBatches(count int) { MatrixBatchCount.Add(float64(count)) }

func SearchRun(duration time.Duration, eligible, rejected int, reasons map[string]int) {
	SearchRunsTotal.Inc()
	EligibleCandidates.Add(float64(eligible))
	RejectedCandidates.Add(float64(rejected))
	SearchDuration.Observe(duration.Seconds())
	for reason, count := range reasons {
		RejectionsByReason.WithLabelValues(reason).Add(float64(count))
	}
}
