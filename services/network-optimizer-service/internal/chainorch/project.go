package chainorch

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/network-optimizer-service/internal/chain"
	"github.com/freight-platform/network-optimizer-service/internal/repository"
)

// ProjectDurableCandidates maps an actual engine shortlist into frozen chain_candidates rows.
// Ordinals are dense 1..N in engine shortlist order. Pair identity is canonical
// (first_load_id < second_load_id) with matching versions. UNFINISHED is not collapsed
// into INFEASIBLE. Unknown metrics stay nil (never coerced to zero).
func ProjectDurableCandidates(run repository.ChainSearchRun, result chain.Result) ([]repository.ChainCandidate, error) {
	out := make([]repository.ChainCandidate, 0, len(result.Candidates))
	seenIDs := map[uuid.UUID]struct{}{}
	seenPairs := map[string]struct{}{}
	for i, projected := range result.Candidates {
		row, err := projectOneCandidate(run, projected, i+1)
		if err != nil {
			return nil, err
		}
		if _, ok := seenIDs[row.CandidateID]; ok {
			return nil, fmt.Errorf("duplicate candidate_id %s", row.CandidateID)
		}
		seenIDs[row.CandidateID] = struct{}{}
		pairKey := row.FirstLoadID.String() + ":" + row.SecondLoadID.String() + ":" +
			fmt.Sprintf("%d:%d", row.FirstLoadVersion, row.SecondLoadVersion)
		if _, ok := seenPairs[pairKey]; ok {
			return nil, fmt.Errorf("duplicate durable pair %s", pairKey)
		}
		seenPairs[pairKey] = struct{}{}
		out = append(out, row)
	}
	return out, nil
}

func projectOneCandidate(run repository.ChainSearchRun, projected chain.ProjectedCandidate, ordinal int) (repository.ChainCandidate, error) {
	if projected.CandidateID == uuid.Nil {
		return repository.ChainCandidate{}, fmt.Errorf("candidate_id required at ordinal %d", ordinal)
	}
	if len(projected.Loads) != 2 {
		return repository.ChainCandidate{}, fmt.Errorf("candidate %s requires exactly two loads", projected.CandidateID)
	}
	a, b := projected.Loads[0], projected.Loads[1]
	firstID, firstVer, secondID, secondVer := a.LoadID, a.Version, b.LoadID, b.Version
	if secondID.String() < firstID.String() {
		firstID, firstVer, secondID, secondVer = secondID, secondVer, firstID, firstVer
	}
	if !(firstID.String() < secondID.String()) {
		return repository.ChainCandidate{}, fmt.Errorf("candidate %s cannot form canonical pair", projected.CandidateID)
	}
	feasibility := mapFeasibility(projected)
	var selectedOrder *string
	if feasibility == repository.ChainFeasible && projected.SelectedOrder != "" {
		order := projected.SelectedOrder
		selectedOrder = &order
	}

	var empty *float64
	if projected.EmptyRoadMeters != nil {
		v := float64(*projected.EmptyRoadMeters)
		empty = &v
	}
	var target *float64
	if projected.TargetMeters != nil {
		v := float64(*projected.TargetMeters)
		target = &v
	}

	row := repository.ChainCandidate{
		CandidateID:          projected.CandidateID,
		SearchID:             run.ID,
		TenantID:             run.TenantID,
		Ordinal:              ordinal,
		FirstLoadID:          firstID,
		FirstLoadVersion:     firstVer,
		SecondLoadID:         secondID,
		SecondLoadVersion:    secondVer,
		FeasibilityStatus:    feasibility,
		SelectedOrder:        selectedOrder,
		EmptyRoadDistanceM:   empty,
		TotalDurationSeconds: cloneInt(projected.DurationSeconds),
		Utilization:          cloneFloat64(projected.Utilization),
		TargetDistanceM:      target,
		RevenueAmount:        cloneInt64(projected.RevenueAmount),
		RevenueCurrency:      cloneString(projected.RevenueCurrency),
		OmittedComponents:    append([]string(nil), projected.OmittedComponents...),
		ReasonCodes:          append([]string(nil), projected.ReasonCodes...),
		RoutePlanID:          nil, // never fabricate from in-memory evaluation
		CandidateFingerprint: ComputeCandidateFingerprint(CandidateFingerprintInput{
			RequestFingerprint: run.RequestFingerprint,
			AlgorithmVersion:   run.AlgorithmVersion,
			PolicyVersion:      run.PolicyVersion,
			CandidateID:        projected.CandidateID,
			FirstLoadID:        firstID,
			FirstLoadVersion:   firstVer,
			SecondLoadID:       secondID,
			SecondLoadVersion:  secondVer,
			FeasibilityStatus:  feasibility,
			SelectedOrder:      selectedOrder,
			OrdersEvaluated:    projected.OrdersEvaluated,
		}),
		CreatedAt: time.Time{}, // DB-owned on insert
	}
	if row.OmittedComponents == nil {
		row.OmittedComponents = []string{}
	}
	if row.ReasonCodes == nil {
		row.ReasonCodes = []string{}
	}
	if row.RevenueAmount == nil {
		row.RevenueCurrency = nil
	}
	return row, nil
}

func mapFeasibility(projected chain.ProjectedCandidate) string {
	if projected.Feasible {
		return repository.ChainFeasible
	}
	for _, code := range projected.ReasonCodes {
		if code == chain.ReasonUnfinished {
			return repository.ChainUnfinished
		}
	}
	return repository.ChainInfeasible
}

// CandidateFingerprintInput is the deterministic provenance material for candidate_fingerprint.
// It must not include private shipper/customer/contract text.
type CandidateFingerprintInput struct {
	RequestFingerprint string
	AlgorithmVersion   string
	PolicyVersion      string
	CandidateID        uuid.UUID
	FirstLoadID        uuid.UUID
	FirstLoadVersion   int
	SecondLoadID       uuid.UUID
	SecondLoadVersion  int
	FeasibilityStatus  string
	SelectedOrder      *string
	OrdersEvaluated    []string
}

// ComputeCandidateFingerprint returns a stable hex digest for the durable candidate identity.
func ComputeCandidateFingerprint(in CandidateFingerprintInput) string {
	sum := sha256.New()
	fmt.Fprintf(sum, "request_fingerprint=%s\n", in.RequestFingerprint)
	fmt.Fprintf(sum, "algorithm_version=%s\n", in.AlgorithmVersion)
	fmt.Fprintf(sum, "policy_version=%s\n", in.PolicyVersion)
	fmt.Fprintf(sum, "candidate_id=%s\n", in.CandidateID)
	fmt.Fprintf(sum, "first_load_id=%s\n", in.FirstLoadID)
	fmt.Fprintf(sum, "first_load_version=%d\n", in.FirstLoadVersion)
	fmt.Fprintf(sum, "second_load_id=%s\n", in.SecondLoadID)
	fmt.Fprintf(sum, "second_load_version=%d\n", in.SecondLoadVersion)
	fmt.Fprintf(sum, "feasibility_status=%s\n", in.FeasibilityStatus)
	order := ""
	if in.SelectedOrder != nil {
		order = *in.SelectedOrder
	}
	fmt.Fprintf(sum, "selected_order=%s\n", order)
	orders := append([]string(nil), in.OrdersEvaluated...)
	sort.Strings(orders)
	fmt.Fprintf(sum, "orders_evaluated=%s\n", strings.Join(orders, ","))
	return fmt.Sprintf("%x", sum.Sum(nil))
}

func cloneInt(v *int) *int {
	if v == nil {
		return nil
	}
	x := *v
	return &x
}

func cloneInt64(v *int64) *int64 {
	if v == nil {
		return nil
	}
	x := *v
	return &x
}

func cloneFloat64(v *float64) *float64 {
	if v == nil {
		return nil
	}
	x := *v
	return &x
}

func cloneString(v *string) *string {
	if v == nil {
		return nil
	}
	x := *v
	return &x
}

// CountersFromEngineResult derives durable run counters from the actual engine result.
// candidate_count is the shortlist length that will be persisted.
func CountersFromEngineResult(result chain.Result, durableCandidateCount int) (candidateCount, evaluatedPairCount, providerCallCount int, reasonCounts map[string]int) {
	stats := TerminalStatsFromEngine(result, durableCandidateCount, nil)
	return stats.CandidateCount, stats.EvaluatedPairCount, stats.ProviderCallCount, stats.ReasonCounts
}

// TerminalStatsFromEngine builds canonical terminal counters from an actual engine result.
// durableCandidateCount must equal rows actually committed (0 when none are persisted).
// classifiedReasons, when non-nil, replaces raw engine ReasonCounts (engine copy plus
// any I2B classification keys). Evaluations/ProviderCalls always come from the engine.
func TerminalStatsFromEngine(result chain.Result, durableCandidateCount int, classifiedReasons map[string]int) TerminalStats {
	reasons := map[string]int{}
	if classifiedReasons != nil {
		for k, v := range classifiedReasons {
			reasons[k] = v
		}
	} else {
		for k, v := range result.ReasonCounts {
			reasons[k] = v
		}
	}
	return TerminalStats{
		CandidateCount:     durableCandidateCount,
		EvaluatedPairCount: result.Evaluations,
		ProviderCallCount:  result.ProviderCalls,
		ReasonCounts:       reasons,
	}
}
