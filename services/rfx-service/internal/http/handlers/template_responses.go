package handlers

import (
	"encoding/json"

	"github.com/freight-platform/rfx-service/internal/domain"
)

func toRfxTemplateResponse(tmpl *domain.RfxTemplate) map[string]any {
	resp := map[string]any{
		"id":            tmpl.ID.String(),
		"tenant_id":     tmpl.TenantID.String(),
		"template_code": tmpl.TemplateCode,
		"name_i18n":     jsonRawToAny(tmpl.NameI18nJSON),
		"status":        tmpl.Status,
		"version":       tmpl.Version,
		"created_by":    tmpl.CreatedBy.String(),
		"created_at":    tmpl.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		"updated_at":    tmpl.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
	if len(tmpl.DescriptionI18nJSON) > 0 {
		resp["description_i18n"] = jsonRawToAny(tmpl.DescriptionI18nJSON)
	} else {
		resp["description_i18n"] = nil
	}
	if tmpl.RfxType != nil {
		resp["rfx_type"] = *tmpl.RfxType
	} else {
		resp["rfx_type"] = nil
	}
	if tmpl.OwnerCompanyID != nil {
		resp["owner_company_id"] = tmpl.OwnerCompanyID.String()
	} else {
		resp["owner_company_id"] = nil
	}
	return resp
}

func toRfxTemplateVersionResponse(version *domain.RfxTemplateVersion) map[string]any {
	resp := map[string]any{
		"id":              version.ID.String(),
		"tenant_id":       version.TenantID.String(),
		"template_id":     version.TemplateID.String(),
		"version_number":  version.VersionNumber,
		"status":          version.Status,
		"change_summary":  version.ChangeSummary,
		"is_active_draft": version.IsActiveDraft,
		"is_published":    version.IsPublished,
		"published_at":    formatDateTime(version.PublishedAt),
		"created_by":      version.CreatedBy.String(),
		"created_at":      version.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		"updated_at":      version.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		"version":         version.Version,
	}
	if version.PublishedBy != nil {
		resp["published_by"] = version.PublishedBy.String()
	} else {
		resp["published_by"] = nil
	}
	return resp
}

func toTemplateDetailResponse(detail *domain.TemplateDetail) map[string]any {
	versions := make([]map[string]any, 0, len(detail.Versions))
	for i := range detail.Versions {
		versions = append(versions, toRfxTemplateVersionResponse(&detail.Versions[i]))
	}
	resp := map[string]any{
		"template": toRfxTemplateResponse(&detail.Template),
		"versions": versions,
	}
	if detail.DraftVersion != nil {
		resp["draft_version"] = toRfxTemplateVersionResponse(detail.DraftVersion)
	} else {
		resp["draft_version"] = nil
	}
	if detail.PublishedVersion != nil {
		resp["published_version"] = toRfxTemplateVersionResponse(detail.PublishedVersion)
	} else {
		resp["published_version"] = nil
	}
	return resp
}

func toTemplateQuestionnaireResponse(def *domain.TemplateQuestionnaireDefinition) map[string]any {
	return map[string]any{
		"template_id":             def.TemplateID.String(),
		"rfx_template_version_id": def.RfxTemplateVersionID.String(),
		"version_number":          def.VersionNumber,
		"version_status":          def.VersionStatus,
		"sections":                toTemplateSectionWithQuestionsResponses(def.Sections),
		"rules":                   toTemplateQuestionRuleResponses(def.Rules),
	}
}

func toTemplateSectionWithQuestionsResponses(sections []domain.TemplateSectionWithQuestions) []map[string]any {
	out := make([]map[string]any, 0, len(sections))
	for _, swq := range sections {
		questions := make([]map[string]any, 0, len(swq.Questions))
		for i := range swq.Questions {
			questions = append(questions, toTemplateQuestionResponse(&swq.Questions[i]))
		}
		out = append(out, map[string]any{
			"section":   toTemplateSectionResponse(&swq.Section),
			"questions": questions,
		})
	}
	return out
}

func toTemplateSectionResponse(section *domain.TemplateSection) map[string]any {
	return map[string]any{
		"id":                      section.ID.String(),
		"tenant_id":               section.TenantID.String(),
		"template_id":             section.TemplateID.String(),
		"rfx_template_version_id": section.RfxTemplateVersionID.String(),
		"section_code":            section.SectionCode,
		"title":                   section.Title,
		"description":             section.Description,
		"sort_order":              section.SortOrder,
		"created_at":              section.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		"updated_at":              section.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		"version":                 section.Version,
	}
}

func toTemplateQuestionResponse(question *domain.TemplateQuestion) map[string]any {
	opts := make([]map[string]any, 0, len(question.Options))
	for i := range question.Options {
		opts = append(opts, toTemplateQuestionOptionResponse(&question.Options[i]))
	}
	return map[string]any{
		"id":                   question.ID.String(),
		"tenant_id":            question.TenantID.String(),
		"section_id":           question.SectionID.String(),
		"question_code":        question.QuestionCode,
		"question_type":        question.QuestionType,
		"label":                question.Label,
		"help_text":            question.HelpText,
		"required":             question.Required,
		"validation_rule_json": jsonRawToAny(question.ValidationRuleJSON),
		"sort_order":           question.SortOrder,
		"created_at":           question.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		"updated_at":           question.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		"version":              question.Version,
		"options":              opts,
	}
}

func toTemplateQuestionOptionResponse(option *domain.TemplateQuestionOption) map[string]any {
	return map[string]any{
		"id":          option.ID.String(),
		"tenant_id":   option.TenantID.String(),
		"question_id": option.QuestionID.String(),
		"option_code": option.OptionCode,
		"label":       option.Label,
		"sort_order":  option.SortOrder,
		"created_at":  option.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		"updated_at":  option.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		"version":     option.Version,
	}
}

func toTemplateQuestionRuleResponses(rules []domain.TemplateQuestionRule) []map[string]any {
	out := make([]map[string]any, 0, len(rules))
	for i := range rules {
		out = append(out, toTemplateQuestionRuleResponse(&rules[i]))
	}
	return out
}

func toTemplateQuestionRuleResponse(rule *domain.TemplateQuestionRule) map[string]any {
	resp := map[string]any{
		"id":                      rule.ID.String(),
		"tenant_id":               rule.TenantID.String(),
		"template_id":             rule.TemplateID.String(),
		"rfx_template_version_id": rule.RfxTemplateVersionID.String(),
		"rule_code":               rule.RuleCode,
		"action":                  rule.Action,
		"condition_json":          jsonRawToAny(rule.ConditionJSON),
		"sort_order":              rule.SortOrder,
		"created_at":              rule.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		"updated_at":              rule.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		"version":                 rule.Version,
	}
	if rule.TargetQuestionID != nil {
		resp["target_question_id"] = rule.TargetQuestionID.String()
	} else {
		resp["target_question_id"] = nil
	}
	return resp
}

func jsonRawToAny(raw json.RawMessage) any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return map[string]any{}
	}
	return v
}
