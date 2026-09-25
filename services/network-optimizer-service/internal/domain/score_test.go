package domain

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestBNO257_PROFILE_CODES_VALIDATED(t *testing.T) {
	for _, code := range []string{ProfileMinDeadhead, ProfileMaxCapacityUtilization, ProfileReturnHome, ProfileMaxRevenue, ProfileMinRisk, ProfileBalanced, ProfileMaxContribution} {
		if !KnownObjectiveProfile(code) {
			t.Fatalf("missing %s", code)
		}
	}
	policy := NextLoadSearchPolicy{SearchMode: SearchRadius, RadiusKm: f64p(10), ObjectiveProfile: "MIN_EMPTY"}
	if err := policy.Validate(); err == nil {
		t.Fatal("unknown profile accepted")
	}
	policy.ObjectiveProfile = ProfileMinDeadhead
	if err := policy.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestBNO259_PROFILE_WEIGHT_SUM_10000(t *testing.T) {
	for _, profile := range testScoreProfiles() {
		if profile.Status != ProfileStatusActive {
			continue
		}
		if profile.WeightSum() != 10000 {
			t.Fatalf("%s sum %d", profile.Code, profile.WeightSum())
		}
	}
}

func TestBNO260_PROFILE_FINGERPRINT_DETERMINISTIC(t *testing.T) {
	profile := profileByCode(t, ProfileBalanced)
	if profile.Fingerprint() == "" || profile.Fingerprint() != profile.Fingerprint() {
		t.Fatal(profile.Fingerprint())
	}
	changed := profile
	changed.Components = append([]ScoreProfileComponent(nil), profile.Components...)
	changed.Components[0].WeightBps = 3400
	if changed.Fingerprint() == profile.Fingerprint() {
		t.Fatal("weight change did not change fingerprint")
	}
	reversed := profile
	reversed.Components = append([]ScoreProfileComponent(nil), profile.Components...)
	for i, j := 0, len(reversed.Components)-1; i < j; i, j = i+1, j-1 {
		reversed.Components[i], reversed.Components[j] = reversed.Components[j], reversed.Components[i]
	}
	if reversed.Fingerprint() != profile.Fingerprint() {
		t.Fatal("fingerprint depended on component slice order")
	}
}

func TestBNO279_BALANCED_WEIGHTED_SCORE(t *testing.T) {
	best := richFact("a1", 20, 0.9, 0, 10, 120, 0)
	worst := richFact("b1", 80, 0.1, 100, 80, 0, 0)
	scored := ScorePool(profileByCode(t, ProfileBalanced), []ScoreFacts{best, worst}, "")
	if scored[best.LoadID].Total == nil || *scored[best.LoadID].Total != 9750 {
		t.Fatalf("best %+v", scored[best.LoadID])
	}
	if scored[worst.LoadID].Total == nil || *scored[worst.LoadID].Total != 250 {
		t.Fatalf("worst %+v", scored[worst.LoadID])
	}
}

func TestBNO280_BALANCED_OPTIONAL_MISSING_RENORMALIZED(t *testing.T) {
	near := ScoreFacts{LoadID: uuid.MustParse("00000000-0000-4000-8000-0000000000a1"), DeadheadKm: f64p(20)}
	far := ScoreFacts{LoadID: uuid.MustParse("00000000-0000-4000-8000-0000000000a2"), DeadheadKm: f64p(80)}
	scored := ScorePool(profileByCode(t, ProfileBalanced), []ScoreFacts{near, far}, "")
	if *scored[near.LoadID].Total != 9375 || *scored[far.LoadID].Total != 625 {
		t.Fatalf("near %+v far %+v", scored[near.LoadID], scored[far.LoadID])
	}
	if *scored[near.LoadID].Total == 3750 {
		t.Fatal("missing optional components were treated as zero")
	}
}

func TestBNO281_SCORE_EVIDENCE_BPS_CORRECT(t *testing.T) {
	near := ScoreFacts{LoadID: uuid.New(), DeadheadKm: f64p(20)}
	far := ScoreFacts{LoadID: uuid.New(), DeadheadKm: f64p(80)}
	scored := ScorePool(profileByCode(t, ProfileBalanced), []ScoreFacts{near, far}, "")
	if *scored[near.LoadID].EvidenceBps != 4000 {
		t.Fatalf("evidence %d", *scored[near.LoadID].EvidenceBps)
	}
	full := ScorePool(profileByCode(t, ProfileMinDeadhead), []ScoreFacts{near, far}, "")
	if *full[near.LoadID].EvidenceBps != 10000 {
		t.Fatalf("full %d", *full[near.LoadID].EvidenceBps)
	}
}

func TestBNO282_NETWORK_VALUE_DETERMINISTIC_FALLBACK(t *testing.T) {
	fact := ScoreFacts{LoadID: uuid.New(), DeadheadKm: f64p(10)}
	scored := ScorePool(profileByCode(t, ProfileBalanced), []ScoreFacts{fact}, "")
	var found bool
	for _, component := range scored[fact.LoadID].Components {
		if component.Code != ComponentNetworkValue {
			continue
		}
		found = true
		if component.Status != ComponentFallback || component.RawValue == nil || *component.RawValue != 0 || component.Explanation != "Network value is not yet observed" {
			t.Fatalf("%+v", component)
		}
	}
	if !found {
		t.Fatal("network component missing")
	}
}

func TestBNO283_HIGHER_IS_BETTER_NORMALIZATION(t *testing.T) {
	points := normalize([]float64{1, 2, 4}, true)
	if points[0] != 0 || points[1] != 3333 || points[2] != 10000 {
		t.Fatalf("%v", points)
	}
}

func TestBNO284_LOWER_IS_BETTER_NORMALIZATION(t *testing.T) {
	points := normalize([]float64{1, 2, 4}, false)
	if points[0] != 10000 || points[1] != 6667 || points[2] != 0 {
		t.Fatalf("%v", points)
	}
}

func TestBNO285_EQUAL_VALUES_NORMALIZE_TO_5000(t *testing.T) {
	points := normalize([]float64{40, 40, 40}, true)
	for _, point := range points {
		if point != 5000 {
			t.Fatalf("%v", points)
		}
	}
}

func TestBNO286_SCORE_RANGE_0_10000(t *testing.T) {
	low := ScoreFacts{LoadID: uuid.New(), DeadheadKm: f64p(80)}
	high := ScoreFacts{LoadID: uuid.New(), DeadheadKm: f64p(20)}
	scored := ScorePool(profileByCode(t, ProfileMinDeadhead), []ScoreFacts{low, high}, "")
	for _, item := range scored {
		if item.Total == nil || *item.Total < 0 || *item.Total > 10000 {
			t.Fatalf("%+v", item)
		}
	}
	if *scored[high.LoadID].Total != 10000 || *scored[low.LoadID].Total != 0 {
		t.Fatalf("%+v", scored)
	}
}

func TestBNO287_FIXED_POINT_REPEATABLE(t *testing.T) {
	facts := []ScoreFacts{
		{LoadID: uuid.MustParse("00000000-0000-4000-8000-0000000000b1"), DeadheadKm: f64p(20)},
		{LoadID: uuid.MustParse("00000000-0000-4000-8000-0000000000b2"), DeadheadKm: f64p(50)},
	}
	first := ScorePool(profileByCode(t, ProfileMinDeadhead), facts, "")
	second := ScorePool(profileByCode(t, ProfileMinDeadhead), facts, "")
	if first[facts[0].LoadID].Fingerprint != second[facts[0].LoadID].Fingerprint || *first[facts[0].LoadID].Total != *second[facts[0].LoadID].Total {
		t.Fatal("score drifted")
	}
}

func TestBNO274_PREDICTION_CONFIDENCE_USED(t *testing.T) {
	low := ScoreFacts{LoadID: uuid.New(), SlackKnown: true, SlackMinutes: f64p(30), Confidence: f64p(0.2), UncertaintySeconds: intp(100)}
	high := ScoreFacts{LoadID: uuid.New(), SlackKnown: true, SlackMinutes: f64p(30), Confidence: f64p(0.9), UncertaintySeconds: intp(100)}
	scored := ScorePool(profileByCode(t, ProfileMinRisk), []ScoreFacts{low, high}, "")
	if *scored[high.LoadID].Total <= *scored[low.LoadID].Total {
		t.Fatalf("confidence %+v %+v", scored[high.LoadID], scored[low.LoadID])
	}
}

func TestBNO275_ETA_UNCERTAINTY_USED(t *testing.T) {
	calm := ScoreFacts{LoadID: uuid.New(), SlackKnown: true, SlackMinutes: f64p(30), Confidence: f64p(0.5), UncertaintySeconds: intp(60)}
	wide := ScoreFacts{LoadID: uuid.New(), SlackKnown: true, SlackMinutes: f64p(30), Confidence: f64p(0.5), UncertaintySeconds: intp(600)}
	scored := ScorePool(profileByCode(t, ProfileMinRisk), []ScoreFacts{calm, wide}, "")
	if *scored[calm.LoadID].Total <= *scored[wide.LoadID].Total {
		t.Fatalf("uncertainty %+v %+v", scored[calm.LoadID], scored[wide.LoadID])
	}
}

// testScoreProfiles mirrors migration 000080 for in-package scoring tests.
// Production scoring does not call it.
func testScoreProfiles() []ScoreProfile {
	active := func(code string, components []ScoreProfileComponent) ScoreProfile {
		return ScoreProfile{
			Code: code, Scope: ProfileScopeSystem, Version: 1,
			Status: ProfileStatusActive, AlgorithmVersion: ScoringAlgorithmVersion, Components: components,
		}
	}
	comp := func(code string, weight, ordinal int, required bool) ScoreProfileComponent {
		return ScoreProfileComponent{Code: code, WeightBps: weight, Required: required, Ordinal: ordinal}
	}
	return []ScoreProfile{
		active(ProfileMinDeadhead, []ScoreProfileComponent{comp(ComponentDeadheadEfficiency, 10000, 1, true)}),
		active(ProfileMaxCapacityUtilization, []ScoreProfileComponent{comp(ComponentCapacityUtilization, 10000, 1, true)}),
		active(ProfileReturnHome, []ScoreProfileComponent{comp(ComponentTargetProximity, 10000, 1, true)}),
		active(ProfileMaxRevenue, []ScoreProfileComponent{comp(ComponentRevenue, 10000, 1, true)}),
		active(ProfileMinRisk, []ScoreProfileComponent{
			comp(ComponentPickupSlack, 5000, 1, true),
			comp(ComponentPredictionConfidence, 3000, 2, false),
			comp(ComponentETAUncertainty, 2000, 3, false),
		}),
		active(ProfileBalanced, []ScoreProfileComponent{
			comp(ComponentDeadheadEfficiency, 3500, 1, true),
			comp(ComponentCapacityUtilization, 2500, 2, false),
			comp(ComponentWaitingEfficiency, 1500, 3, false),
			comp(ComponentTargetProximity, 1000, 4, false),
			comp(ComponentPickupSlack, 1000, 5, false),
			comp(ComponentNetworkValue, 500, 6, false),
		}),
	}
}

func profileByCode(t *testing.T, code string) ScoreProfile {
	t.Helper()
	for _, profile := range testScoreProfiles() {
		if profile.Code == code {
			return profile
		}
	}
	t.Fatalf("missing %s", code)
	return ScoreProfile{}
}

func TestBNO319_PROFILE_ORDINAL_UNIQUE(t *testing.T) {
	profile := profileByCode(t, ProfileMinDeadhead)
	profile.Components = []ScoreProfileComponent{
		{Code: ComponentDeadheadEfficiency, WeightBps: 5000, Required: true, Ordinal: 1},
		{Code: ComponentNetworkValue, WeightBps: 5000, Required: false, Ordinal: 1},
	}
	if !errors.Is(ValidateActiveScoreProfile(profile), ErrScoreProfileInvalid) {
		t.Fatal("duplicate ordinal accepted")
	}
}

func TestBNO320_PROFILE_COMPONENT_VALIDATION(t *testing.T) {
	profile := profileByCode(t, ProfileMinDeadhead)
	profile.Components = []ScoreProfileComponent{{Code: "NOT_A_COMPONENT", WeightBps: 10000, Required: true, Ordinal: 1}}
	if !errors.Is(ValidateActiveScoreProfile(profile), ErrScoreProfileInvalid) {
		t.Fatal("unknown component accepted")
	}
	profile.Components = []ScoreProfileComponent{{Code: ComponentDeadheadEfficiency, WeightBps: 10001, Required: true, Ordinal: 1}}
	if !errors.Is(ValidateActiveScoreProfile(profile), ErrScoreProfileInvalid) {
		t.Fatal("weight above 10000 accepted")
	}
	unsupported := profileByCode(t, ProfileMinDeadhead)
	unsupported.AlgorithmVersion = "bno-score-unsupported"
	if !errors.Is(ValidateActiveScoreProfile(unsupported), ErrScoringAlgorithmUnsupported) {
		t.Fatal("unsupported algorithm accepted")
	}
	if scored := ScorePool(unsupported, []ScoreFacts{{LoadID: uuid.New(), DeadheadKm: f64p(10)}}, ""); len(scored) != 0 {
		t.Fatal("unsupported algorithm was scored")
	}
}

func richFact(id string, deadhead, utilization, waiting, target, slack, network float64) ScoreFacts {
	_ = network
	return ScoreFacts{
		LoadID:     uuid.MustParse("00000000-0000-4000-8000-0000000000" + id),
		DeadheadKm: f64p(deadhead), Utilization: f64p(utilization), WaitingKnown: true, WaitingMinutes: f64p(waiting),
		SlackKnown: true, SlackMinutes: f64p(slack), TargetKm: f64p(target),
	}
}

func f64p(v float64) *float64 { return &v }

func intp(v int) *int { return &v }
