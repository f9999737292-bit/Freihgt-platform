package domain

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

const (
	RfxVersionStatusDraft      = "DRAFT"
	RfxVersionStatusPublished  = "PUBLISHED"
	RfxVersionStatusSuperseded = "SUPERSEDED"
	RfxVersionStatusArchived   = "ARCHIVED"

	RuleActionShow    = "SHOW"
	RuleActionHide    = "HIDE"
	RuleActionRequire = "REQUIRE"

	PublishCheckPass = "PASS"
	PublishCheckFail = "FAIL"
	PublishCheckWarn = "WARN"
)

type RfxVersion struct {
	ID                  uuid.UUID
	TenantID            uuid.UUID
	RfxEventID          uuid.UUID
	VersionNumber       int
	Status              string
	QuestionnaireEnabled bool
	PublishedAt         *time.Time
	PublishedBy         *uuid.UUID
	CreatedAt           time.Time
	UpdatedAt           time.Time
	Version             int
}

type Section struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	RfxVersionID uuid.UUID
	SectionCode  string
	Title        string
	Description  *string
	SortOrder    int
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Version      int
}

type Question struct {
	ID                   uuid.UUID
	TenantID             uuid.UUID
	SectionID            uuid.UUID
	QuestionCode         string
	QuestionType         string
	Label                string
	HelpText             *string
	Required             bool
	ValidationRuleJSON   json.RawMessage
	SortOrder            int
	CreatedAt            time.Time
	UpdatedAt            time.Time
	Version              int
	Options              []QuestionOption
}

type QuestionOption struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	QuestionID  uuid.UUID
	OptionCode  string
	Label       string
	SortOrder   int
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Version     int
}

type QuestionRule struct {
	ID               uuid.UUID
	TenantID         uuid.UUID
	RfxVersionID     uuid.UUID
	TargetQuestionID *uuid.UUID
	RuleCode         string
	Action           string
	ConditionJSON    json.RawMessage
	SortOrder        int
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Version          int
}

type ValidationDefinition struct {
	MinLength   *int     `json:"min_length,omitempty"`
	MaxLength   *int     `json:"max_length,omitempty"`
	MinValue    *float64 `json:"min_value,omitempty"`
	MaxValue    *float64 `json:"max_value,omitempty"`
	Pattern     *string  `json:"pattern,omitempty"`
	AllowedMIME []string `json:"allowed_mime,omitempty"`
	MaxFileSize *int64   `json:"max_file_size,omitempty"`
}

type ConditionalExpression struct {
	Operator           string                  `json:"operator"`
	SourceQuestionCode string                  `json:"source_question_code,omitempty"`
	Value              json.RawMessage         `json:"value,omitempty"`
	Children           []ConditionalExpression `json:"children,omitempty"`
}

type PublishReadinessItem struct {
	Code    string         `json:"code"`
	Status  string         `json:"status"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

type PublishReadinessResult struct {
	Ready        bool                   `json:"ready"`
	BlockingFail int                    `json:"blocking_fail_count"`
	WarningCount int                    `json:"warning_count"`
	Items        []PublishReadinessItem `json:"items"`
}

type QuestionnaireStudio struct {
	Event        RfxEvent
	DraftVersion *RfxVersion
	Sections     []SectionWithQuestions
	Rules        []QuestionRule
}

type QuestionnaireDefinition struct {
	EventID              uuid.UUID
	RfxVersionID         uuid.UUID
	VersionNumber        int
	QuestionnaireEnabled bool
	VersionStatus        string
	Sections             []SectionWithQuestions
	Rules                []QuestionRule
}

func (d QuestionnaireDefinition) Version() RfxVersion {
	return RfxVersion{
		ID:                   d.RfxVersionID,
		RfxEventID:           d.EventID,
		VersionNumber:        d.VersionNumber,
		Status:               d.VersionStatus,
		QuestionnaireEnabled: d.QuestionnaireEnabled,
	}
}

type SectionWithQuestions struct {
	Section   Section
	Questions []Question
}

type CreateSectionInput struct {
	SectionCode string  `json:"section_code"`
	Title       string  `json:"title"`
	Description *string `json:"description,omitempty"`
	SortOrder   *int    `json:"sort_order,omitempty"`
}

type UpdateSectionInput struct {
	Title           *string `json:"title,omitempty"`
	Description     *string `json:"description,omitempty"`
	SortOrder       *int    `json:"sort_order,omitempty"`
	ExpectedVersion int     `json:"expected_version"`
}

type CreateQuestionInput struct {
	QuestionCode       string          `json:"question_code"`
	QuestionType       string          `json:"question_type"`
	Label              string          `json:"label"`
	HelpText           *string         `json:"help_text,omitempty"`
	Required           bool            `json:"required"`
	ValidationRuleJSON json.RawMessage `json:"validation_rule_json,omitempty"`
	SortOrder          *int            `json:"sort_order,omitempty"`
}

type UpdateQuestionInput struct {
	QuestionType       *string         `json:"question_type,omitempty"`
	Label              *string         `json:"label,omitempty"`
	HelpText           *string         `json:"help_text,omitempty"`
	Required           *bool           `json:"required,omitempty"`
	ValidationRuleJSON json.RawMessage `json:"validation_rule_json,omitempty"`
	SortOrder          *int            `json:"sort_order,omitempty"`
	ExpectedVersion    int             `json:"expected_version"`
}

type CreateQuestionOptionInput struct {
	OptionCode string `json:"option_code"`
	Label      string `json:"label"`
	SortOrder  *int   `json:"sort_order,omitempty"`
}

type UpdateQuestionOptionInput struct {
	Label           *string `json:"label,omitempty"`
	SortOrder       *int    `json:"sort_order,omitempty"`
	ExpectedVersion int     `json:"expected_version"`
}

type CreateQuestionRuleInput struct {
	RuleCode           string          `json:"rule_code"`
	Action             string          `json:"action"`
	TargetQuestionCode *string         `json:"target_question_code,omitempty"`
	ConditionJSON      json.RawMessage `json:"condition_json,omitempty"`
	SortOrder          *int            `json:"sort_order,omitempty"`
}

type UpdateQuestionRuleInput struct {
	Action             *string         `json:"action,omitempty"`
	TargetQuestionCode *string         `json:"target_question_code,omitempty"`
	ConditionJSON      json.RawMessage `json:"condition_json,omitempty"`
	SortOrder          *int            `json:"sort_order,omitempty"`
	ExpectedVersion    int             `json:"expected_version"`
}

type ReorderInput struct {
	OrderedIDs []uuid.UUID `json:"ordered_ids"`
}

func ValidateSectionCode(code string) error {
	code = strings.TrimSpace(code)
	if code == "" {
		return apperrors.Validation("section_code is required", map[string]any{"field": "section_code"})
	}
	if len(code) > 100 {
		return apperrors.Validation("section_code is too long", map[string]any{"field": "section_code"})
	}
	return nil
}

func ValidateQuestionCode(code string) error {
	code = strings.TrimSpace(code)
	if code == "" {
		return apperrors.Validation("question_code is required", map[string]any{"field": "question_code"})
	}
	if len(code) > 100 {
		return apperrors.Validation("question_code is too long", map[string]any{"field": "question_code"})
	}
	return nil
}

func ValidateOptionCode(code string) error {
	code = strings.TrimSpace(code)
	if code == "" {
		return apperrors.Validation("option_code is required", map[string]any{"field": "option_code"})
	}
	if len(code) > 100 {
		return apperrors.Validation("option_code is too long", map[string]any{"field": "option_code"})
	}
	return nil
}

func ValidateRuleCode(code string) error {
	code = strings.TrimSpace(code)
	if code == "" {
		return apperrors.Validation("rule_code is required", map[string]any{"field": "rule_code"})
	}
	if len(code) > 100 {
		return apperrors.Validation("rule_code is too long", map[string]any{"field": "rule_code"})
	}
	return nil
}

func ValidateRuleAction(action string) error {
	action = strings.TrimSpace(action)
	switch action {
	case RuleActionShow, RuleActionHide, RuleActionRequire:
		return nil
	default:
		return apperrors.Validation("invalid rule action", map[string]any{"field": "action", "value": action})
	}
}

func ValidateCreateSectionInput(in CreateSectionInput) error {
	if err := ValidateSectionCode(in.SectionCode); err != nil {
		return err
	}
	if strings.TrimSpace(in.Title) == "" {
		return apperrors.Validation("title is required", map[string]any{"field": "title"})
	}
	return nil
}

func ValidateCreateQuestionInput(in CreateQuestionInput) error {
	if err := ValidateQuestionCode(in.QuestionCode); err != nil {
		return err
	}
	if err := ValidateQuestionType(in.QuestionType); err != nil {
		return err
	}
	if strings.TrimSpace(in.Label) == "" {
		return apperrors.Validation("label is required", map[string]any{"field": "label"})
	}
	return ValidateValidationDefinition(in.QuestionType, in.ValidationRuleJSON)
}

func ValidateCreateQuestionOptionInput(in CreateQuestionOptionInput) error {
	if err := ValidateOptionCode(in.OptionCode); err != nil {
		return err
	}
	if strings.TrimSpace(in.Label) == "" {
		return apperrors.Validation("label is required", map[string]any{"field": "label"})
	}
	return nil
}

func ValidateCreateQuestionRuleInput(in CreateQuestionRuleInput) error {
	if err := ValidateRuleCode(in.RuleCode); err != nil {
		return err
	}
	if err := ValidateRuleAction(in.Action); err != nil {
		return err
	}
	return ValidateConditionalExpression(in.ConditionJSON)
}

func EnsureDraftVersionMutable(status string) error {
	if status != RfxVersionStatusDraft {
		return apperrors.Conflict("questionnaire version is not editable", map[string]any{"status": status})
	}
	return nil
}
