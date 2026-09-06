package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const (
	ScoreModelStatusDraft     = "DRAFT"
	ScoreModelStatusPublished = "PUBLISHED"

	ScoreModelTypeAutomatic = "AUTOMATIC"

	QualificationStatusQualified              = "QUALIFIED"
	QualificationStatusConditionallyQualified = "CONDITIONALLY_QUALIFIED"
	QualificationStatusRejected               = "REJECTED"
	QualificationStatusPendingReview          = "PENDING_REVIEW"

	ScoringCalculationPending   = "PENDING"
	ScoringCalculationCalculated = "CALCULATED"
	ScoringCalculationFailed    = "FAILED"

	NormalizationNumberLinear   = "NUMBER_LINEAR"
	NormalizationBooleanMap     = "BOOLEAN_MAP"
	NormalizationOptionMap      = "OPTION_MAP"
	NormalizationMultiSelect    = "MULTI_SELECT"

	MultiSelectAggregationSumCapped = "SUM_CAPPED"
	MultiSelectAggregationMax       = "MAX"
	MultiSelectAggregationAverage   = "AVERAGE"

	KnockoutBooleanEquals  = "BOOLEAN_EQUALS"
	KnockoutOptionEquals   = "OPTION_EQUALS"
	KnockoutNumberThreshold = "NUMBER_THRESHOLD"
)

type ScoreModel struct {
	ID             uuid.UUID
	TenantID       uuid.UUID
	RfxVersionID   uuid.UUID
	ModelVersion   int
	Status         string
	ModelType      string
	DefinitionJSON json.RawMessage
	CreatedBy      *uuid.UUID
	CreatedAt      time.Time
	UpdatedAt      time.Time
	PublishedAt    *time.Time
}

type ScoreCriterion struct {
	ID                uuid.UUID
	TenantID          uuid.UUID
	ScoreModelID      uuid.UUID
	CriterionCode     string
	Name              string
	Weight            float64
	NormalizationJSON json.RawMessage
	SortOrder         int
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type ScoreBinding struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	ScoreModelID     uuid.UUID
	CriterionID      uuid.UUID
	QuestionID       uuid.UUID
	BindingType      string
	ScoringRuleJSON  json.RawMessage
	KnockoutRuleJSON json.RawMessage
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type AnswerScore struct {
	ID                   uuid.UUID
	TenantID             uuid.UUID
	RfxResponseID        uuid.UUID
	AnswerID             uuid.UUID
	CriterionID          uuid.UUID
	ScoreModelID         uuid.UUID
	ScoreModelVersion    int
	RawScore             float64
	NormalizedScore      float64
	WeightedContribution float64
	ExplanationJSON      json.RawMessage
	CalculatedAt         time.Time
}

type QualificationResult struct {
	ID                 uuid.UUID
	TenantID           uuid.UUID
	RfxResponseID      uuid.UUID
	ScoreModelID       uuid.UUID
	ScoreModelVersion  int
	Status             string
	CalculationStatus  string
	TotalScore         *float64
	KnockoutTriggered  bool
	KnockoutReasonJSON json.RawMessage
	CalculatedAt       *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type ScoreExplanation struct {
	Source               string         `json:"source"`
	Input                map[string]any `json:"input"`
	Rule                 string         `json:"rule"`
	RuleVersion          int            `json:"rule_version,omitempty"`
	ScoreModelID         string         `json:"score_model_id"`
	ScoreModelVersion    int            `json:"score_model_version"`
	CriterionCode        string         `json:"criterion_code"`
	CriterionWeight      float64        `json:"criterion_weight"`
	RawScore             float64        `json:"raw_score"`
	NormalizedScore      float64        `json:"normalized_score"`
	WeightedContribution float64        `json:"weighted_contribution"`
	Knockout             bool           `json:"knockout"`
	KnockoutReason       string         `json:"knockout_reason,omitempty"`
}

type ScoreModelDefinition struct {
	Criteria []ScoreCriterionInput `json:"criteria"`
	Bindings []ScoreBindingInput   `json:"bindings"`
}

type ScoreCriterionInput struct {
	CriterionCode     string          `json:"criterion_code"`
	Name              string          `json:"name"`
	Weight            float64         `json:"weight"`
	NormalizationJSON json.RawMessage `json:"normalization_json"`
	SortOrder         int             `json:"sort_order"`
}

type ScoreBindingInput struct {
	CriterionCode    string          `json:"criterion_code"`
	QuestionCode     string          `json:"question_code"`
	ScoringRuleJSON  json.RawMessage `json:"scoring_rule_json"`
	KnockoutRuleJSON json.RawMessage `json:"knockout_rule_json,omitempty"`
}

type ScoreModelView struct {
	Model      ScoreModel
	Criteria   []ScoreCriterion
	Bindings   []ScoreBinding
	Readiness  ScoreModelReadinessResult
}

type ScoreModelReadinessResult struct {
	Ready  bool                     `json:"ready"`
	Errors []ScoreModelReadinessError `json:"errors,omitempty"`
}

type ScoreModelReadinessError struct {
	Code    string         `json:"code"`
	Field   string         `json:"field,omitempty"`
	Message string         `json:"message"`
	Params  map[string]any `json:"params,omitempty"`
}

type ScoringRunResult struct {
	Qualification QualificationResult
	AnswerScores  []AnswerScore
	Skipped       bool
}

type ResponseScoreView struct {
	Qualification QualificationResult `json:"qualification"`
	AnswerScores  []AnswerScore       `json:"answer_scores"`
}

type PutScoreModelInput struct {
	Criteria []ScoreCriterionInput `json:"criteria"`
	Bindings []ScoreBindingInput   `json:"bindings"`
}
