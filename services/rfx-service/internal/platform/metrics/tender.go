package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	registerOnce sync.Once

	bidRevisionTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "rfx_service",
			Name:      "tender_bid_revision_total",
			Help:      "Total enterprise bid revisions submitted",
		},
		[]string{"result"},
	)
	evaluationTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "rfx_service",
			Name:      "tender_evaluation_total",
			Help:      "Total tender evaluations run",
		},
		[]string{"result"},
	)
	allocationTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "rfx_service",
			Name:      "tender_allocation_total",
			Help:      "Total allocation scenarios computed",
		},
		[]string{"result"},
	)
	awardProposalTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "rfx_service",
			Name:      "tender_award_proposal_total",
			Help:      "Total award proposals created",
		},
		[]string{"result"},
	)
	awardTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "rfx_service",
			Name:      "tender_award_total",
			Help:      "Total final awards created",
		},
		[]string{"result"},
	)
	awardConversionTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "rfx_service",
			Name:      "tender_award_conversion_total",
			Help:      "Total award conversion attempts",
		},
		[]string{"result"},
	)
	allocationInfeasibleTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "rfx_service",
			Name:      "tender_allocation_infeasible_total",
			Help:      "Total infeasible allocation scenarios",
		},
	)
)

func Register() {
	registerOnce.Do(func() {
		prometheus.MustRegister(
			bidRevisionTotal,
			evaluationTotal,
			allocationTotal,
			awardProposalTotal,
			awardTotal,
			awardConversionTotal,
			allocationInfeasibleTotal,
		)
	})
}

func IncBidRevision(result string)         { Register(); bidRevisionTotal.WithLabelValues(result).Inc() }
func IncEvaluation(result string)          { Register(); evaluationTotal.WithLabelValues(result).Inc() }
func IncAllocation(result string)          { Register(); allocationTotal.WithLabelValues(result).Inc() }
func IncAwardProposal(result string)       { Register(); awardProposalTotal.WithLabelValues(result).Inc() }
func IncAward(result string)               { Register(); awardTotal.WithLabelValues(result).Inc() }
func IncAwardConversion(result string)     { Register(); awardConversionTotal.WithLabelValues(result).Inc() }
func IncAllocationInfeasible()               { Register(); allocationInfeasibleTotal.Inc() }
