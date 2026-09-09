package domain

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

const (
	RfxTemplateStatusActive   = "ACTIVE"
	RfxTemplateStatusArchived = "ARCHIVED"

	TemplateLifecycleOperationPublish   = "PUBLISH_TEMPLATE_VERSION"
	TemplateLifecycleOperationForkDraft = "FORK_TEMPLATE_DRAFT"
)

type RfxTemplate struct {
	ID                  uuid.UUID
	TenantID            uuid.UUID
	TemplateCode        string
	NameI18nJSON        json.RawMessage
	DescriptionI18nJSON json.RawMessage
	RfxType             *string
	OwnerCompanyID      *uuid.UUID
	Status              string
	Version             int
	CreatedBy           uuid.UUID
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type RfxTemplateVersion struct {
	ID            uuid.UUID
	TenantID      uuid.UUID
	TemplateID    uuid.UUID
	VersionNumber int
	Status        string
	PublishedAt   *time.Time
	PublishedBy   *uuid.UUID
	ChangeSummary *string
	Version       int
	CreatedBy     uuid.UUID
	CreatedAt     time.Time
	UpdatedAt     time.Time
	IsActiveDraft bool
	IsPublished   bool
}

type TemplateSection struct {
	ID                   uuid.UUID
	TenantID             uuid.UUID
	TemplateID           uuid.UUID
	RfxTemplateVersionID uuid.UUID
	SectionCode          string
	Title                string
	Description          *string
	SortOrder            int
	CreatedAt            time.Time
	UpdatedAt            time.Time
	Version              int
}

type TemplateQuestion struct {
	ID                 uuid.UUID
	TenantID           uuid.UUID
	SectionID          uuid.UUID
	QuestionCode       string
	QuestionType       string
	Label              string
	HelpText           *string
	Required           bool
	ValidationRuleJSON json.RawMessage
	SortOrder          int
	CreatedAt          time.Time
	UpdatedAt          time.Time
	Version            int
	Options            []TemplateQuestionOption
}

type TemplateQuestionOption struct {
	ID         uuid.UUID
	TenantID   uuid.UUID
	QuestionID uuid.UUID
	OptionCode string
	Label      string
	SortOrder  int
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Version    int
}

type TemplateQuestionRule struct {
	ID                   uuid.UUID
	TenantID             uuid.UUID
	TemplateID           uuid.UUID
	RfxTemplateVersionID uuid.UUID
	TargetQuestionID     *uuid.UUID
	RuleCode             string
	Action               string
	ConditionJSON        json.RawMessage
	SortOrder            int
	CreatedAt            time.Time
	UpdatedAt            time.Time
	Version              int
}

type TemplateSectionWithQuestions struct {
	Section   TemplateSection
	Questions []TemplateQuestion
}

type TemplateQuestionnaireDefinition struct {
	TemplateID           uuid.UUID
	RfxTemplateVersionID uuid.UUID
	VersionNumber        int
	VersionStatus        string
	Sections             []TemplateSectionWithQuestions
	Rules                []TemplateQuestionRule
}

type TemplateDetail struct {
	Template         RfxTemplate
	DraftVersion     *RfxTemplateVersion
	PublishedVersion *RfxTemplateVersion
	Versions         []RfxTemplateVersion
}

type TemplateListFilter struct {
	Status         *string
	OwnerCompanyID *uuid.UUID
	RfxType        *string
	Search         *string
	Limit          int
	Offset         int

	// AccessScope is populated by the service from verified membership; never trust client input.
	AccessibleOwnerCompanyIDs []uuid.UUID
	IncludeTenantWide         bool
	DenyAll                   bool
}

type CreateTemplateInput struct {
	TemplateCode    string          `json:"template_code"`
	NameI18n        json.RawMessage `json:"name_i18n"`
	DescriptionI18n json.RawMessage `json:"description_i18n,omitempty"`
	RfxType         *string         `json:"rfx_type,omitempty"`
	OwnerCompanyID  *uuid.UUID      `json:"owner_company_id,omitempty"`
}

type UpdateTemplateInput struct {
	NameI18n        json.RawMessage `json:"name_i18n,omitempty"`
	DescriptionI18n json.RawMessage `json:"description_i18n,omitempty"`
	RfxType         *string         `json:"rfx_type,omitempty"`
	ExpectedVersion int             `json:"expected_version"`
}

type PublishTemplateVersionInput struct {
	ExpectedTemplateVersion int    `json:"expected_template_version"`
	ExpectedDraftVersion    int    `json:"expected_draft_version"`
	ChangeSummary           string `json:"change_summary"`
}

func ValidateCreateTemplateInput(in CreateTemplateInput) error {
	code := strings.TrimSpace(in.TemplateCode)
	if code == "" {
		return apperrors.Validation("template_code is required", map[string]any{"field": "template_code"})
	}
	if len(code) > 128 {
		return apperrors.Validation("template_code is too long", map[string]any{"field": "template_code"})
	}
	if _, err := ValidateTemplateI18nMap(in.NameI18n, "name_i18n", true); err != nil {
		return err
	}
	if len(in.DescriptionI18n) > 0 {
		if _, err := ValidateTemplateI18nMap(in.DescriptionI18n, "description_i18n", false); err != nil {
			return err
		}
	}
	return nil
}

func ValidateUpdateTemplateInput(in UpdateTemplateInput) error {
	if in.ExpectedVersion <= 0 {
		return apperrors.Validation("expected_version must be positive", map[string]any{"field": "expected_version"})
	}
	if len(in.NameI18n) > 0 {
		if _, err := ValidateTemplateI18nMap(in.NameI18n, "name_i18n", true); err != nil {
			return err
		}
	}
	if len(in.DescriptionI18n) > 0 {
		if _, err := ValidateTemplateI18nMap(in.DescriptionI18n, "description_i18n", false); err != nil {
			return err
		}
	}
	return nil
}

func ValidatePublishTemplateVersionInput(in PublishTemplateVersionInput) error {
	if in.ExpectedTemplateVersion <= 0 {
		return apperrors.Validation("expected_template_version must be positive", map[string]any{"field": "expected_template_version"})
	}
	if in.ExpectedDraftVersion <= 0 {
		return apperrors.Validation("expected_draft_version must be positive", map[string]any{"field": "expected_draft_version"})
	}
	if strings.TrimSpace(in.ChangeSummary) == "" {
		return apperrors.Validation("change_summary is required", map[string]any{"field": "change_summary"})
	}
	return nil
}

func EnsureTemplateActive(status string) error {
	if status != RfxTemplateStatusActive {
		return apperrors.Conflict("template is archived", map[string]any{"status": status})
	}
	return nil
}

func EnsureTemplateVersionDraft(status string) error {
	if status != RfxVersionStatusDraft {
		return apperrors.Conflict("template version is not editable", map[string]any{"status": status})
	}
	return nil
}

func EnsureTemplateVersionPublishable(status string) error {
	if status != RfxVersionStatusDraft {
		return apperrors.Conflict("template version is not publishable", map[string]any{"status": status})
	}
	return nil
}

func EnsureTemplateVersionForkSource(status string) error {
	if status != RfxVersionStatusPublished {
		return apperrors.Conflict("only the published template version can be forked", map[string]any{"status": status})
	}
	return nil
}

func TemplatePublishReadinessFailure(result PublishReadinessResult) error {
	return apperrors.Validation("template publish readiness failed", map[string]any{
		"ready":               result.Ready,
		"blocking_fail_count": result.BlockingFail,
		"items":               result.Items,
	})
}

func ToQuestionnaireSections(sections []TemplateSectionWithQuestions) []SectionWithQuestions {
	out := make([]SectionWithQuestions, 0, len(sections))
	for _, swq := range sections {
		questions := make([]Question, 0, len(swq.Questions))
		for _, q := range swq.Questions {
			opts := make([]QuestionOption, 0, len(q.Options))
			for _, o := range q.Options {
				opts = append(opts, QuestionOption{
					ID: o.ID, TenantID: o.TenantID, QuestionID: o.QuestionID,
					OptionCode: o.OptionCode, Label: o.Label, SortOrder: o.SortOrder,
					CreatedAt: o.CreatedAt, UpdatedAt: o.UpdatedAt, Version: o.Version,
				})
			}
			questions = append(questions, Question{
				ID: q.ID, TenantID: q.TenantID, SectionID: q.SectionID,
				QuestionCode: q.QuestionCode, QuestionType: q.QuestionType, Label: q.Label,
				HelpText: q.HelpText, Required: q.Required, ValidationRuleJSON: q.ValidationRuleJSON,
				SortOrder: q.SortOrder, CreatedAt: q.CreatedAt, UpdatedAt: q.UpdatedAt, Version: q.Version,
				Options: opts,
			})
		}
		out = append(out, SectionWithQuestions{
			Section: Section{
				ID: swq.Section.ID, TenantID: swq.Section.TenantID, RfxVersionID: swq.Section.RfxTemplateVersionID,
				SectionCode: swq.Section.SectionCode, Title: swq.Section.Title, Description: swq.Section.Description,
				SortOrder: swq.Section.SortOrder, CreatedAt: swq.Section.CreatedAt, UpdatedAt: swq.Section.UpdatedAt,
				Version: swq.Section.Version,
			},
			Questions: questions,
		})
	}
	return out
}

func ToQuestionnaireRules(rules []TemplateQuestionRule) []QuestionRule {
	out := make([]QuestionRule, 0, len(rules))
	for _, r := range rules {
		out = append(out, QuestionRule{
			ID: r.ID, TenantID: r.TenantID, RfxVersionID: r.RfxTemplateVersionID,
			TargetQuestionID: r.TargetQuestionID, RuleCode: r.RuleCode, Action: r.Action,
			ConditionJSON: r.ConditionJSON, SortOrder: r.SortOrder,
			CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, Version: r.Version,
		})
	}
	return out
}
