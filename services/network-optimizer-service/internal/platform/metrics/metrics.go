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
	ConsolidationSearches = promauto.NewCounter(prometheus.CounterOpts{
		Name: "bno_consolidation_searches_total",
		Help: "Pairwise consolidation searches.",
	})
	ConsolidationSets = promauto.NewCounter(prometheus.CounterOpts{
		Name: "bno_consolidation_sets_evaluated_total",
		Help: "Pairwise consolidation sets evaluated.",
	})
	ConsolidationOutcomes = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "bno_consolidation_outcomes_total",
		Help: "Pairwise consolidation outcomes by bounded status.",
	}, []string{"status"})
	ConsolidationDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "bno_consolidation_duration_seconds",
		Help:    "Pairwise consolidation search duration.",
		Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2, 5},
	})
)

var ConsolidationBudgetExhausted = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "bno_consolidation_budget_exhausted_total",
	Help: "N-member consolidation searches rejected by a bounded budget reason.",
}, []string{"reason"})

func ConsolidationBudget(reason string) {
	switch reason {
	case "POOL_LIMIT_EXCEEDED", "SEARCH_BUDGET_EXCEEDED":
		ConsolidationBudgetExhausted.WithLabelValues(reason).Inc()
	}
}

func ConsolidationSearch(elapsed time.Duration, evaluated, feasible, indeterminate, hard int) {
	ConsolidationSearches.Inc()
	ConsolidationSets.Add(float64(evaluated))
	ConsolidationDuration.Observe(elapsed.Seconds())
	ConsolidationOutcomes.WithLabelValues("FEASIBLE").Add(float64(feasible))
	ConsolidationOutcomes.WithLabelValues("INDETERMINATE").Add(float64(indeterminate))
	ConsolidationOutcomes.WithLabelValues("HARD_REJECT").Add(float64(hard))
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
	ScoreRuns = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "bno_score_runs_total", Help: "Scored next-load searches by objective profile.",
	}, []string{"profile"})
	RankedCandidates = promauto.NewCounter(prometheus.CounterOpts{
		Name: "bno_ranked_candidates_total", Help: "Eligible candidates that received a rank.",
	})
	UnrankedCandidates = promauto.NewCounter(prometheus.CounterOpts{
		Name: "bno_unranked_candidates_total", Help: "Eligible candidates that could not be ranked.",
	})
	ScoreDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "bno_score_duration_seconds", Help: "Deterministic match scoring duration.",
		Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.25, 1},
	})
)

func SearchPool(size int) { CandidatePoolSize.Observe(float64(size)) }

var (
	CandidateDiscovered = promauto.NewCounter(prometheus.CounterOpts{
		Name: "bno_candidate_discovered_total",
		Help: "Marketplace loads matching the visibility predicate before the discovery cap. Aggregate only.",
	})
	CandidateVisibilityPass = promauto.NewCounter(prometheus.CounterOpts{
		Name: "bno_candidate_visibility_pass_total",
		Help: "Marketplace loads that passed tenant and visibility rules before the discovery cap. Aggregate only.",
	})
	CandidatePrefilterPass = promauto.NewCounter(prometheus.CounterOpts{
		Name: "bno_candidate_prefilter_pass_total",
		Help: "Bounded candidates that passed cheap geometry prefilters before any road call. Aggregate only.",
	})
	CandidatePruned = promauto.NewCounter(prometheus.CounterOpts{
		Name: "bno_candidate_pruned_total",
		Help: "Visible marketplace loads dropped by the server discovery cap. Aggregate only.",
	})
	CandidateReturned = promauto.NewCounter(prometheus.CounterOpts{
		Name: "bno_candidate_returned_total",
		Help: "Visible marketplace loads kept by the server discovery cap. Aggregate only.",
	})
)

// RecordCandidateDiscovery adds aggregate discovery counters.
// The arguments are counts. They must not carry coordinates, addresses,
// location identifiers, customer labels, or provider secrets.
func RecordCandidateDiscovery(discovered, visibilityPass, pruned, returned int) {
	CandidateDiscovered.Add(float64(discovered))
	CandidateVisibilityPass.Add(float64(visibilityPass))
	CandidatePruned.Add(float64(pruned))
	CandidateReturned.Add(float64(returned))
}

// RecordCandidatePrefilterPass adds the cheap-prefilter survivor count.
func RecordCandidatePrefilterPass(passed int) {
	CandidatePrefilterPass.Add(float64(passed))
}

func RoutingError() { RoutingErrors.Inc() }

func MatrixBatches(count int) { MatrixBatchCount.Add(float64(count)) }

func ScoreOutcome(profile string, duration time.Duration, ranked, unranked int) {
	ScoreRuns.WithLabelValues(profile).Inc()
	RankedCandidates.Add(float64(ranked))
	UnrankedCandidates.Add(float64(unranked))
	ScoreDuration.Observe(duration.Seconds())
}

var (
	CurrentTripContextBuilds = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "bno_current_trip_context_build_total",
		Help: "Server-built current trip context results.",
	}, []string{"result"})
	ResidualCapacityDimensions = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "bno_residual_capacity_dimension_total",
		Help: "Residual capacity dimension statuses.",
	}, []string{"dimension", "status"})
)

func CurrentTripContext(result string) {
	CurrentTripContextBuilds.WithLabelValues(result).Inc()
}

func ResidualDimension(dimension, status string) {
	ResidualCapacityDimensions.WithLabelValues(dimension, status).Inc()
}

func SearchRun(duration time.Duration, eligible, rejected int, reasons map[string]int) {
	SearchRunsTotal.Inc()
	EligibleCandidates.Add(float64(eligible))
	RejectedCandidates.Add(float64(rejected))
	SearchDuration.Observe(duration.Seconds())
	for reason, count := range reasons {
		RejectionsByReason.WithLabelValues(reason).Add(float64(count))
	}
}
