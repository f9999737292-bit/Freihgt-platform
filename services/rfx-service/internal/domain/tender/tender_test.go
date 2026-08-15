package tender

import (
	"testing"
)

func TestValidateScoringTemplate(t *testing.T) {
	valid := []ScoringFactorWeight{
		{Factor: FactorPrice, Weight: 35},
		{Factor: FactorSLA, Weight: 20},
		{Factor: FactorCarrierKPI, Weight: 15},
		{Factor: FactorCapacity, Weight: 10},
		{Factor: FactorReliability, Weight: 10},
		{Factor: FactorTransitTime, Weight: 10},
	}
	if err := ValidateScoringTemplate(valid); err != nil {
		t.Fatalf("expected valid template: %v", err)
	}

	invalidSum := append([]ScoringFactorWeight{}, valid...)
	invalidSum[0].Weight = 25
	if err := ValidateScoringTemplate(invalidSum); err == nil {
		t.Fatal("expected sum validation error")
	}

	dup := append([]ScoringFactorWeight{}, valid...)
	dup[1].Factor = FactorPrice
	if err := ValidateScoringTemplate(dup); err == nil {
		t.Fatal("expected duplicate factor error")
	}

	if err := ValidateScoringTemplate([]ScoringFactorWeight{{Factor: "JS_INJECTION", Weight: 100}}); err == nil {
		t.Fatal("expected unsupported factor error")
	}
}

func TestScoreCandidatesDeterministic(t *testing.T) {
	template := ScoringTemplateSnapshot{
		VersionNumber: 1,
		Factors: []ScoringFactorWeight{
			{Factor: FactorPrice, Weight: 35},
			{Factor: FactorSLA, Weight: 20},
			{Factor: FactorCarrierKPI, Weight: 15},
			{Factor: FactorCapacity, Weight: 10},
			{Factor: FactorReliability, Weight: 10},
			{Factor: FactorTransitTime, Weight: 10},
		},
	}
	candidates := []BidCandidate{
		{CarrierCompanyID: "A", PriceAmount: 100, SLAScoreInput: 90, CarrierKPIInput: 85, CapacityUnits: 500, ReliabilityInput: 80, TransitHours: 24, CarrierActive: true},
		{CarrierCompanyID: "B", PriceAmount: 90, SLAScoreInput: 70, CarrierKPIInput: 88, CapacityUnits: 400, ReliabilityInput: 75, TransitHours: 20, CarrierActive: true},
		{CarrierCompanyID: "C", PriceAmount: 80, SLAScoreInput: 95, CarrierKPIInput: 92, CapacityUnits: 300, ReliabilityInput: 90, TransitHours: 18, CarrierActive: true},
		{CarrierCompanyID: "D", PriceAmount: 110, SLAScoreInput: 88, CarrierKPIInput: 80, CapacityUnits: 200, ReliabilityInput: 85, TransitHours: 22, CarrierActive: true},
	}
	results, err := ScoreCandidates(template, candidates, 400)
	if err != nil {
		t.Fatalf("score candidates: %v", err)
	}
	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}
	// Lowest price is C — should rank strongly but not guaranteed winner due to weights.
	if results[0].CarrierCompanyID == "" {
		t.Fatal("missing top carrier")
	}
	firstRun := results[0].TotalScore
	second, err := ScoreCandidates(template, candidates, 400)
	if err != nil {
		t.Fatalf("second score run: %v", err)
	}
	if second[0].TotalScore != firstRun {
		t.Fatalf("non-deterministic score: %.4f vs %.4f", firstRun, second[0].TotalScore)
	}
	if results[0].PriceScore <= 0 {
		t.Fatal("expected normalized price score")
	}
}

func TestQualificationHardGate(t *testing.T) {
	rules := QualificationRules{MinimumSLAScore: 75, MinimumCapacity: 250, RequireCarrierActive: true}
	candidates := []BidCandidate{
		{CarrierCompanyID: "B", PriceAmount: 90, SLAScoreInput: 70, CapacityUnits: 400, CarrierActive: true},
	}
	results := EvaluateQualification(rules, candidates)
	if len(results) != 1 || results[0].Result != QualificationDisqualified {
		t.Fatalf("expected B disqualified for SLA, got %+v", results)
	}
}

func TestQualificationOverridesBalance(t *testing.T) {
	rules := QualificationRules{MinimumSLAScore: 75}
	candidates := []BidCandidate{
		{CarrierCompanyID: "B", PriceAmount: 90, SLAScoreInput: 70, CapacityUnits: 400, CarrierActive: true},
	}
	qual := EvaluateQualification(rules, candidates)
	qualified := QualifiedCarrierSet(qual)
	scores, _ := ScoreCandidates(ScoringTemplateSnapshot{
		VersionNumber: 1,
		Factors: []ScoringFactorWeight{
			{Factor: FactorPrice, Weight: 100},
		},
	}, candidates, 400)
	positions, _ := ComputeQuotaPositions(QuotaBalancePolicy{
		TolerancePct: 5, CarryBalance: true, MaxCorrectionPct: 10,
	}, []QuotaTarget{{CarrierCompanyID: "B", TargetSharePct: 30}}, map[string]float64{"B": 18})
	adj := BalanceAdjustmentsForAllocation(positions, map[string]struct{}{"B": {}})
	outcome := ComputeAllocation(AllocationConfig{
		Strategy: StrategyScoreWeighted,
		Constraints: AllocationConstraints{TotalVolume: 1000, MinSuppliers: 1},
	}, scores, candidates, adj)
	lines := ApplyQualificationOverrideBalance(outcome.Lines, qualified)
	for _, l := range lines {
		if l.CarrierCompanyID == "B" && l.ProposedSharePct > 0 {
			t.Fatalf("disqualified carrier B must receive 0 allocation, got %.2f", l.ProposedSharePct)
		}
	}
}

func TestAllocationInfeasibleMinSuppliers(t *testing.T) {
	scores := []CarrierScoreResult{
		{CarrierCompanyID: "A", TotalScore: 90},
		{CarrierCompanyID: "B", TotalScore: 80},
	}
	outcome := ComputeAllocation(AllocationConfig{
		Strategy:    StrategyDiversified,
		RankShares:  []float64{50, 30, 20},
		Constraints: AllocationConstraints{MinSuppliers: 3},
	}, scores, nil, nil)
	if outcome.Status != AllocationStatusInfeasible {
		t.Fatalf("expected INFEASIBLE, got %s", outcome.Status)
	}
}

func TestConcentrationCap(t *testing.T) {
	scores := []CarrierScoreResult{
		{CarrierCompanyID: "A", TotalScore: 95},
		{CarrierCompanyID: "B", TotalScore: 85},
		{CarrierCompanyID: "C", TotalScore: 75},
	}
	candidates := []BidCandidate{
		{CarrierCompanyID: "A", PriceAmount: 100, CapacityUnits: 1000},
		{CarrierCompanyID: "B", PriceAmount: 110, CapacityUnits: 1000},
		{CarrierCompanyID: "C", PriceAmount: 120, CapacityUnits: 1000},
	}
	outcome := ComputeAllocation(AllocationConfig{
		Strategy:    StrategyWinnerTakesMost,
		RankShares:  []float64{70, 20, 10},
		Constraints: AllocationConstraints{MaxCarrierSharePct: 50, TotalVolume: 1000},
	}, scores, candidates, nil)
	if outcome.Status != AllocationStatusComputed {
		t.Fatalf("expected COMPUTED, got %s reasons=%v", outcome.Status, outcome.Reasons)
	}
	for _, l := range outcome.Lines {
		if l.ProposedSharePct > 50.01 {
			t.Fatalf("carrier %s exceeded max share: %.2f", l.CarrierCompanyID, l.ProposedSharePct)
		}
	}
}

func TestQuotaBalanceFixture(t *testing.T) {
	targets := []QuotaTarget{
		{CarrierCompanyID: "A", TargetSharePct: 40},
		{CarrierCompanyID: "B", TargetSharePct: 30},
		{CarrierCompanyID: "C", TargetSharePct: 20},
		{CarrierCompanyID: "D", TargetSharePct: 10},
	}
	actual := map[string]float64{"A": 52, "B": 18, "C": 21, "D": 9}
	positions, err := ComputeQuotaPositions(QuotaBalancePolicy{TolerancePct: 5, CarryBalance: true, MaxCorrectionPct: 10}, targets, actual)
	if err != nil {
		t.Fatalf("compute quota: %v", err)
	}
	statusByCarrier := map[string]string{}
	for _, p := range positions {
		statusByCarrier[p.CarrierCompanyID] = p.Status
	}
	if statusByCarrier["A"] != BalanceOverallocated {
		t.Fatalf("A expected OVERALLOCATED, got %s", statusByCarrier["A"])
	}
	if statusByCarrier["B"] != BalanceUnderallocated {
		t.Fatalf("B expected UNDERALLOCATED, got %s", statusByCarrier["B"])
	}
	if statusByCarrier["C"] != BalanceBalanced {
		t.Fatalf("C expected BALANCED, got %s", statusByCarrier["C"])
	}
}

func TestBalanceCorrectionBounded(t *testing.T) {
	targets := []QuotaTarget{
		{CarrierCompanyID: "B", TargetSharePct: 30},
		{CarrierCompanyID: "X", TargetSharePct: 70},
	}
	actual := map[string]float64{"B": 5, "X": 95}
	positions, err := ComputeQuotaPositions(QuotaBalancePolicy{
		TolerancePct: 5, CarryBalance: true, MaxCorrectionPct: 10,
	}, targets, actual)
	if err != nil {
		t.Fatalf("compute quota: %v", err)
	}
	if positions[0].NextAdjustment > 10.01 {
		t.Fatalf("expected bounded correction <=10, got %.2f", positions[0].NextAdjustment)
	}
}

func TestCapacityLimitsAllocation(t *testing.T) {
	scores := []CarrierScoreResult{{CarrierCompanyID: "A", TotalScore: 90}}
	candidates := []BidCandidate{{CarrierCompanyID: "A", PriceAmount: 100, CapacityUnits: 200}}
	outcome := ComputeAllocation(AllocationConfig{
		Strategy:    StrategyWinnerTakesMost,
		RankShares:  []float64{100},
		Constraints: AllocationConstraints{TotalVolume: 1000},
	}, scores, candidates, nil)
	if len(outcome.Lines) != 1 {
		t.Fatalf("expected one line")
	}
	if outcome.Lines[0].ProposedVolume > 200.01 {
		t.Fatalf("volume exceeds capacity: %.3f", outcome.Lines[0].ProposedVolume)
	}
}

func TestAwardProposalTransitions(t *testing.T) {
	if err := ValidateAwardProposalTransition(AwardProposalDraft, AwardProposalPendingApproval); err != nil {
		t.Fatalf("valid transition rejected: %v", err)
	}
	if err := ValidateAwardProposalTransition(AwardProposalDraft, AwardProposalApproved); err == nil {
		t.Fatal("expected invalid transition")
	}
	if err := ValidateFinalizeAward(AwardProposalApproved); err != nil {
		t.Fatalf("finalize from approved: %v", err)
	}
	if err := ValidateFinalizeAward(AwardProposalPendingApproval); err == nil {
		t.Fatal("expected finalize blocked before approval")
	}
}

func TestLowestPriceNotAlwaysWinner(t *testing.T) {
	template := ScoringTemplateSnapshot{
		VersionNumber: 1,
		Factors: []ScoringFactorWeight{
			{Factor: FactorPrice, Weight: 35},
			{Factor: FactorSLA, Weight: 20},
			{Factor: FactorCarrierKPI, Weight: 15},
			{Factor: FactorCapacity, Weight: 10},
			{Factor: FactorReliability, Weight: 10},
			{Factor: FactorTransitTime, Weight: 10},
		},
	}
	candidates := []BidCandidate{
		{CarrierCompanyID: "C", PriceAmount: 80, SLAScoreInput: 60, CarrierKPIInput: 70, CapacityUnits: 200, ReliabilityInput: 70, TransitHours: 30, CarrierActive: true},
		{CarrierCompanyID: "A", PriceAmount: 100, SLAScoreInput: 95, CarrierKPIInput: 95, CapacityUnits: 800, ReliabilityInput: 95, TransitHours: 18, CarrierActive: true},
	}
	scores, err := ScoreCandidates(template, candidates, 500)
	if err != nil {
		t.Fatalf("score: %v", err)
	}
	if scores[0].CarrierCompanyID == "C" {
		t.Fatal("lowest price carrier C should not automatically win total score ranking")
	}
	outcome := ComputeAllocation(AllocationConfig{
		Strategy:    StrategyWinnerTakesMost,
		RankShares:  []float64{70, 30},
		Constraints: AllocationConstraints{TotalVolume: 500, MaxCarrierSharePct: 50},
	}, scores, candidates, nil)
	if outcome.Status != AllocationStatusComputed {
		t.Fatalf("allocation failed: %v", outcome.Reasons)
	}
	topShare := 0.0
	topCarrier := ""
	for _, l := range outcome.Lines {
		if l.ProposedSharePct > topShare {
			topShare = l.ProposedSharePct
			topCarrier = l.CarrierCompanyID
		}
	}
	if topCarrier == "C" && topShare >= 70 {
		t.Fatalf("lowest price carrier should not receive winner-takes-most primary share")
	}
}
