package allocationpolicy

import "github.com/shopspring/decimal"

// Explanation is a deterministic, non-award explanation of an evaluation.
type Explanation struct {
	PolicyFingerprint   string           `json:"policy_fingerprint"`
	Eligible            bool             `json:"eligible"`
	HardFailures        []string         `json:"hard_failures"`
	UnavailableAttrs    []string         `json:"unavailable_attributes"`
	ScoreParts          []ScorePart      `json:"score_parts"`
	ScoreTotal          *decimal.Decimal `json:"score_total"`
	MatchedPreferences  []string         `json:"matched_preferences"`
	AppliedCAP          []string         `json:"applied_cap_constraints"`
	ValidationErrors    []string         `json:"validation_errors"`
	ReasonCodes         []string         `json:"reason_codes"`
	PreferenceRank      int              `json:"preference_rank"`
}

// EvaluationResult is the in-process evaluator output (not an Award).
type EvaluationResult struct {
	Eligible       bool
	ScoreTotal     *decimal.Decimal
	PreferenceRank int
	CarrierID      string
	Explanation    Explanation
}
