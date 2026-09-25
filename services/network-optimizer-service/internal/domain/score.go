package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"sort"

	"github.com/google/uuid"
)

const ScoringAlgorithmVersion = "bno-score-0.1c2.1"

const (
	ProfileMinDeadhead            = "MIN_DEADHEAD"
	ProfileMaxCapacityUtilization = "MAX_CAPACITY_UTILIZATION"
	ProfileReturnHome             = "RETURN_HOME"
	ProfileMaxRevenue             = "MAX_REVENUE"
	ProfileMinRisk                = "MIN_RISK"
	ProfileBalanced               = "BALANCED"
	ProfileMaxContribution        = "MAX_CONTRIBUTION"

	ProfileScopeSystem    = "SYSTEM"
	ProfileStatusActive   = "ACTIVE"
	ProfileStatusReserved = "RESERVED"

	ComponentDeadheadEfficiency   = "DEADHEAD_EFFICIENCY"
	ComponentCapacityUtilization  = "CAPACITY_UTILIZATION"
	ComponentWaitingEfficiency    = "WAITING_EFFICIENCY"
	ComponentTargetProximity      = "TARGET_PROXIMITY"
	ComponentPickupSlack          = "PICKUP_SLACK"
	ComponentRevenue              = "REVENUE"
	ComponentPredictionConfidence = "PREDICTION_CONFIDENCE"
	ComponentETAUncertainty       = "ETA_UNCERTAINTY"
	ComponentNetworkValue         = "NETWORK_VALUE"

	ComponentAvailable   = "AVAILABLE"
	ComponentUnavailable = "UNAVAILABLE"
	ComponentFallback    = "FALLBACK"

	ScoreRanked        = "RANKED"
	ScoreUnranked      = "UNRANKED"
	ScoreNotApplicable = "NOT_APPLICABLE"

	ReasonCommercialUnavailable = "COMMERCIAL_INPUT_UNAVAILABLE"
	ReasonCurrencyNotComparable = "CURRENCY_NOT_COMPARABLE"
	ReasonUtilizationUnknown    = "CAPACITY_UTILIZATION_UNAVAILABLE"
	ReasonPickupSlackUnknown    = "PICKUP_SLACK_UNAVAILABLE"
	ReasonDeadheadUnknown       = "ROAD_DEADHEAD_UNAVAILABLE"
	ReasonTargetUnknown         = "TARGET_PROXIMITY_UNAVAILABLE"
	ReasonWaitingUnknown        = "WAITING_UNAVAILABLE"
	ReasonConfidenceUnknown     = "PREDICTION_CONFIDENCE_UNAVAILABLE"
	ReasonUncertaintyUnknown    = "ETA_UNCERTAINTY_UNAVAILABLE"

	ErrObjectiveNotExecutable     = scoreError("OBJECTIVE_PROFILE_NOT_EXECUTABLE")
	ErrRankingCurrencyRequired    = scoreError("RANKING_CURRENCY_REQUIRED")
	ErrTargetRequiredForObjective = scoreError("TARGET_LOCATION_REQUIRED_FOR_OBJECTIVE")
)

type scoreError string

func (e scoreError) Error() string { return string(e) }

type ScoreProfile struct {
	ID               uuid.UUID
	Code             string
	Scope            string
	Version          int
	Status           string
	AlgorithmVersion string
	Components       []ScoreProfileComponent
}

type ScoreProfileComponent struct {
	Code      string
	WeightBps int
	Required  bool
	Ordinal   int
}

type ScoreComponentResult struct {
	Code             string   `json:"code"`
	Status           string   `json:"status"`
	RawValue         *float64 `json:"raw_value,omitempty"`
	RawUnit          string   `json:"raw_unit,omitempty"`
	NormalizedPoints *int     `json:"normalized_points,omitempty"`
	WeightBps        int      `json:"weight_bps"`
	WeightedPoints   *int     `json:"weighted_points,omitempty"`
	ReasonCode       string   `json:"reason_code"`
	Explanation      string   `json:"explanation"`
}

type CandidateScore struct {
	Status             string
	Total              *int
	EvidenceBps        *int
	UnrankedReasons    []string
	Components         []ScoreComponentResult
	Fingerprint        string
	ProfileCode        string
	ProfileVersion     int
	AlgorithmVersion   string
	ProfileFingerprint string
}

type ScoreFacts struct {
	LoadID                   uuid.UUID
	DeadheadKm               *float64
	Utilization              *float64
	WaitingKnown             bool
	WaitingMinutes           *float64
	SlackKnown               bool
	SlackMinutes             *float64
	TargetKm                 *float64
	Revenue                  *float64
	RevenueCurrency          string
	Confidence               *float64
	UncertaintySeconds       *int
	CapacityID               uuid.UUID
	CapacityVersion          int
	LoadVersion              int
	CompatibilityFingerprint string
	PolicyFingerprint        string
}

type componentObservation struct {
	available bool
	fallback  bool
	value     float64
	unit      string
	higher    bool
	missing   string
	reason    string
	explain   string
}

func KnownObjectiveProfile(code string) bool {
	switch code {
	case ProfileMinDeadhead, ProfileMaxCapacityUtilization, ProfileReturnHome, ProfileMaxRevenue, ProfileMinRisk, ProfileBalanced, ProfileMaxContribution:
		return true
	default:
		return false
	}
}

func ObjectiveExecutable(code string) bool {
	return KnownObjectiveProfile(code) && code != ProfileMaxContribution
}

func ValidRankingCurrency(value string) bool {
	if value == "" {
		return true
	}
	if len(value) != 3 {
		return false
	}
	for _, r := range value {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

func (p ScoreProfile) Fingerprint() string {
	components := append([]ScoreProfileComponent(nil), p.Components...)
	sort.Slice(components, func(i, j int) bool { return components[i].Ordinal < components[j].Ordinal })
	body := struct {
		Code       string                  `json:"code"`
		Version    int                     `json:"version"`
		Algorithm  string                  `json:"algorithm_version"`
		Components []ScoreProfileComponent `json:"components"`
	}{Code: p.Code, Version: p.Version, Algorithm: p.AlgorithmVersion, Components: components}
	raw, _ := json.Marshal(body)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func (p ScoreProfile) WeightSum() int {
	total := 0
	for _, component := range p.Components {
		total += component.WeightBps
	}
	return total
}

func SystemScoreProfiles() []ScoreProfile {
	active := func(id, code string, components []ScoreProfileComponent) ScoreProfile {
		return ScoreProfile{
			ID: uuid.MustParse(id), Code: code, Scope: ProfileScopeSystem, Version: 1,
			Status: ProfileStatusActive, AlgorithmVersion: ScoringAlgorithmVersion, Components: components,
		}
	}
	comp := func(code string, weight, ordinal int, required bool) ScoreProfileComponent {
		return ScoreProfileComponent{Code: code, WeightBps: weight, Required: required, Ordinal: ordinal}
	}
	return []ScoreProfile{
		active("00000000-0000-4000-8000-000000000201", ProfileMinDeadhead, []ScoreProfileComponent{
			comp(ComponentDeadheadEfficiency, 10000, 1, true),
		}),
		active("00000000-0000-4000-8000-000000000202", ProfileMaxCapacityUtilization, []ScoreProfileComponent{
			comp(ComponentCapacityUtilization, 10000, 1, true),
		}),
		active("00000000-0000-4000-8000-000000000203", ProfileReturnHome, []ScoreProfileComponent{
			comp(ComponentTargetProximity, 10000, 1, true),
		}),
		active("00000000-0000-4000-8000-000000000204", ProfileMaxRevenue, []ScoreProfileComponent{
			comp(ComponentRevenue, 10000, 1, true),
		}),
		active("00000000-0000-4000-8000-000000000205", ProfileMinRisk, []ScoreProfileComponent{
			comp(ComponentPickupSlack, 5000, 1, true),
			comp(ComponentPredictionConfidence, 3000, 2, false),
			comp(ComponentETAUncertainty, 2000, 3, false),
		}),
		active("00000000-0000-4000-8000-000000000206", ProfileBalanced, []ScoreProfileComponent{
			comp(ComponentDeadheadEfficiency, 3500, 1, true),
			comp(ComponentCapacityUtilization, 2500, 2, false),
			comp(ComponentWaitingEfficiency, 1500, 3, false),
			comp(ComponentTargetProximity, 1000, 4, false),
			comp(ComponentPickupSlack, 1000, 5, false),
			comp(ComponentNetworkValue, 500, 6, false),
		}),
		{
			ID: uuid.MustParse("00000000-0000-4000-8000-000000000207"), Code: ProfileMaxContribution,
			Scope: ProfileScopeSystem, Version: 1, Status: ProfileStatusReserved, AlgorithmVersion: ScoringAlgorithmVersion,
		},
	}
}

func ScorePool(profile ScoreProfile, facts []ScoreFacts, rankingCurrency string) map[uuid.UUID]CandidateScore {
	profileFingerprint := profile.Fingerprint()
	observations := make([]map[string]componentObservation, len(facts))
	for i, fact := range facts {
		observations[i] = observe(profile, fact, rankingCurrency)
	}
	points := map[string]map[uuid.UUID]int{}
	bounds := map[string][2]float64{}
	for _, component := range profile.Components {
		var values []float64
		var ids []uuid.UUID
		for i, fact := range facts {
			obs := observations[i][component.Code]
			if !obs.available {
				continue
			}
			values = append(values, obs.value)
			ids = append(ids, fact.LoadID)
		}
		if len(values) == 0 {
			continue
		}
		min, max := values[0], values[0]
		for _, value := range values[1:] {
			if value < min {
				min = value
			}
			if value > max {
				max = value
			}
		}
		bounds[component.Code] = [2]float64{min, max}
		normalized := normalize(values, observationsFor(observations, ids, facts, component.Code))
		points[component.Code] = map[uuid.UUID]int{}
		for i, id := range ids {
			points[component.Code][id] = normalized[i]
		}
	}
	out := map[uuid.UUID]CandidateScore{}
	for i, fact := range facts {
		out[fact.LoadID] = scoreOne(profile, profileFingerprint, fact, observations[i], points, bounds, rankingCurrency)
	}
	return out
}

func observationsFor(all []map[string]componentObservation, ids []uuid.UUID, facts []ScoreFacts, code string) bool {
	if len(ids) == 0 {
		return false
	}
	for i, fact := range facts {
		if fact.LoadID == ids[0] {
			return all[i][code].higher
		}
	}
	return false
}

func normalize(values []float64, higher bool) []int {
	min, max := values[0], values[0]
	for _, value := range values[1:] {
		if value < min {
			min = value
		}
		if value > max {
			max = value
		}
	}
	out := make([]int, len(values))
	if max == min {
		for i := range out {
			out[i] = 5000
		}
		return out
	}
	span := max - min
	for i, value := range values {
		ratio := (max - value) / span
		if higher {
			ratio = (value - min) / span
		}
		points := int(math.Round(ratio * 10000))
		if points < 0 {
			points = 0
		}
		if points > 10000 {
			points = 10000
		}
		out[i] = points
	}
	return out
}

func scoreOne(profile ScoreProfile, profileFingerprint string, fact ScoreFacts, obs map[string]componentObservation, points map[string]map[uuid.UUID]int, bounds map[string][2]float64, rankingCurrency string) CandidateScore {
	result := CandidateScore{
		Status: ScoreUnranked, ProfileCode: profile.Code, ProfileVersion: profile.Version,
		AlgorithmVersion: profile.AlgorithmVersion, ProfileFingerprint: profileFingerprint,
	}
	var missing []string
	active := 0
	for _, component := range profile.Components {
		item := obs[component.Code]
		view := ScoreComponentResult{Code: component.Code, WeightBps: component.WeightBps, ReasonCode: item.reason, Explanation: item.explain}
		if !item.available {
			view.Status = ComponentUnavailable
			if component.Required {
				missing = append(missing, item.missing)
			}
			result.Components = append(result.Components, view)
			continue
		}
		if item.fallback {
			view.Status = ComponentFallback
		} else {
			view.Status = ComponentAvailable
		}
		value := item.value
		view.RawValue = &value
		view.RawUnit = item.unit
		if scored, ok := points[component.Code][fact.LoadID]; ok {
			view.NormalizedPoints = &scored
		}
		active += component.WeightBps
		result.Components = append(result.Components, view)
	}
	if len(missing) > 0 || active == 0 {
		result.UnrankedReasons = missing
		result.Fingerprint = scoreFingerprint(profile, profileFingerprint, fact, bounds, rankingCurrency, nil)
		return result
	}
	var numerator int64
	for i, component := range profile.Components {
		if result.Components[i].NormalizedPoints == nil {
			continue
		}
		weighted := divRound(int64(*result.Components[i].NormalizedPoints)*int64(component.WeightBps), int64(active))
		result.Components[i].WeightedPoints = &weighted
		numerator += int64(*result.Components[i].NormalizedPoints) * int64(component.WeightBps)
	}
	total := divRound(numerator, int64(active))
	if total < 0 {
		total = 0
	}
	if total > 10000 {
		total = 10000
	}
	evidence := divRound(int64(active)*10000, int64(profile.WeightSum()))
	result.Status = ScoreRanked
	result.Total = &total
	result.EvidenceBps = &evidence
	result.Fingerprint = scoreFingerprint(profile, profileFingerprint, fact, bounds, rankingCurrency, &total)
	return result
}

func divRound(num, den int64) int {
	if den == 0 {
		return 0
	}
	if num >= 0 {
		return int((num + den/2) / den)
	}
	return int((num - den/2) / den)
}

func observe(profile ScoreProfile, fact ScoreFacts, rankingCurrency string) map[string]componentObservation {
	out := map[string]componentObservation{}
	for _, component := range profile.Components {
		switch component.Code {
		case ComponentDeadheadEfficiency:
			out[component.Code] = measure(fact.DeadheadKm, "km", false, ReasonDeadheadUnknown, "DEADHEAD_MEASURED", "Road deadhead compared within the feasible pool")
		case ComponentCapacityUtilization:
			out[component.Code] = measure(fact.Utilization, "ratio", true, ReasonUtilizationUnknown, "UTILIZATION_MEASURED", "Limiting proven capacity dimension")
		case ComponentWaitingEfficiency:
			if !fact.WaitingKnown || fact.WaitingMinutes == nil {
				out[component.Code] = missingObs(ReasonWaitingUnknown, "Waiting time is unknown")
			} else {
				out[component.Code] = measure(fact.WaitingMinutes, "min", false, ReasonWaitingUnknown, "WAITING_MEASURED", "Known waiting before the pickup window")
			}
		case ComponentPickupSlack:
			if !fact.SlackKnown || fact.SlackMinutes == nil {
				out[component.Code] = missingObs(ReasonPickupSlackUnknown, "Pickup window end is unknown")
			} else {
				out[component.Code] = measure(fact.SlackMinutes, "min", true, ReasonPickupSlackUnknown, "PICKUP_SLACK_MEASURED", "Slack before the pickup window end")
			}
		case ComponentTargetProximity:
			out[component.Code] = measure(fact.TargetKm, "km", false, ReasonTargetUnknown, "TARGET_PROXIMITY_MEASURED", "Road distance from delivery to the explicit target")
		case ComponentRevenue:
			out[component.Code] = revenueObs(fact, rankingCurrency)
		case ComponentPredictionConfidence:
			out[component.Code] = measure(fact.Confidence, "ratio", true, ReasonConfidenceUnknown, "PREDICTION_CONFIDENCE_MEASURED", "Existing prediction confidence")
		case ComponentETAUncertainty:
			if fact.UncertaintySeconds == nil {
				out[component.Code] = missingObs(ReasonUncertaintyUnknown, "Prediction uncertainty is unknown")
			} else {
				value := float64(*fact.UncertaintySeconds)
				out[component.Code] = measure(&value, "s", false, ReasonUncertaintyUnknown, "ETA_UNCERTAINTY_MEASURED", "Existing prediction uncertainty")
			}
		case ComponentNetworkValue:
			zero := 0.0
			out[component.Code] = componentObservation{
				available: true, fallback: true, value: zero, unit: "index", higher: true,
				reason: "NETWORK_VALUE_NOT_OBSERVED", explain: "Network value is not yet observed",
			}
		default:
			out[component.Code] = missingObs("COMPONENT_UNKNOWN", "Component is not implemented")
		}
	}
	return out
}

func revenueObs(fact ScoreFacts, rankingCurrency string) componentObservation {
	if fact.Revenue == nil {
		return missingObs(ReasonCommercialUnavailable, "Published commercial amount is absent")
	}
	if fact.RevenueCurrency != rankingCurrency {
		return missingObs(ReasonCurrencyNotComparable, "Published currency differs from the ranking currency")
	}
	return measure(fact.Revenue, rankingCurrency, true, ReasonCommercialUnavailable, "REVENUE_MEASURED", "Published load commercial amount in the ranking currency")
}

func measure(value *float64, unit string, higher bool, missing, reason, explain string) componentObservation {
	if value == nil {
		return missingObs(missing, explain)
	}
	return componentObservation{available: true, value: *value, unit: unit, higher: higher, missing: missing, reason: reason, explain: explain}
}

func missingObs(reason, explain string) componentObservation {
	return componentObservation{missing: reason, reason: reason, explain: explain}
}

func scoreFingerprint(profile ScoreProfile, profileFingerprint string, fact ScoreFacts, bounds map[string][2]float64, rankingCurrency string, total *int) string {
	body := struct {
		Algorithm          string                `json:"algorithm_version"`
		ProfileFingerprint string                `json:"profile_fingerprint"`
		PolicyFingerprint  string                `json:"policy_fingerprint"`
		CapacityID         uuid.UUID             `json:"capacity_id"`
		CapacityVersion    int                   `json:"capacity_version"`
		LoadID             uuid.UUID             `json:"load_id"`
		LoadVersion        int                   `json:"load_version"`
		Compatibility      string                `json:"compatibility_fingerprint"`
		RankingCurrency    string                `json:"ranking_currency,omitempty"`
		DeadheadKm         *float64              `json:"deadhead_km,omitempty"`
		Utilization        *float64              `json:"utilization,omitempty"`
		Waiting            *float64              `json:"waiting_minutes,omitempty"`
		Slack              *float64              `json:"pickup_slack_minutes,omitempty"`
		TargetKm           *float64              `json:"target_km,omitempty"`
		Revenue            *float64              `json:"revenue,omitempty"`
		Confidence         *float64              `json:"confidence,omitempty"`
		Uncertainty        *int                  `json:"uncertainty_seconds,omitempty"`
		Bounds             map[string][2]float64 `json:"bounds"`
		Total              *int                  `json:"score_total,omitempty"`
		Profile            string                `json:"profile"`
	}{
		Algorithm: profile.AlgorithmVersion, ProfileFingerprint: profileFingerprint, PolicyFingerprint: fact.PolicyFingerprint,
		CapacityID: fact.CapacityID, CapacityVersion: fact.CapacityVersion, LoadID: fact.LoadID, LoadVersion: fact.LoadVersion,
		Compatibility: fact.CompatibilityFingerprint, RankingCurrency: rankingCurrency, DeadheadKm: fact.DeadheadKm,
		Utilization: fact.Utilization, Waiting: fact.WaitingMinutes, Slack: fact.SlackMinutes, TargetKm: fact.TargetKm,
		Revenue: fact.Revenue, Confidence: fact.Confidence, Uncertainty: fact.UncertaintySeconds, Bounds: bounds, Total: total,
		Profile: profile.Code,
	}
	raw, _ := json.Marshal(body)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func UtilizationRatio(weightUsed, weightAvailable, volumeUsed, volumeAvailable *float64, palletsUsed *float64, palletsAvailable *int, linearUsed, linearAvailable *float64) *float64 {
	var ratios []float64
	add := func(used, available *float64) {
		if used == nil || available == nil || *available <= 0 {
			return
		}
		ratios = append(ratios, *used / *available)
	}
	add(weightUsed, weightAvailable)
	add(volumeUsed, volumeAvailable)
	add(linearUsed, linearAvailable)
	if palletsUsed != nil && palletsAvailable != nil && *palletsAvailable > 0 {
		available := float64(*palletsAvailable)
		add(palletsUsed, &available)
	}
	if len(ratios) == 0 {
		return nil
	}
	max := ratios[0]
	for _, ratio := range ratios[1:] {
		if ratio > max {
			max = ratio
		}
	}
	return &max
}
