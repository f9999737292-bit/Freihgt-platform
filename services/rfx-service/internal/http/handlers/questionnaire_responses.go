package handlers

import (
	"encoding/json"
	"time"

	"github.com/freight-platform/rfx-service/internal/domain"
)

func toQuestionnaireStudioResponse(view *domain.QuestionnaireStudio) map[string]any {
	resp := map[string]any{
		"event":    toRfxEventResponse(&view.Event),
		"sections": toSectionWithQuestionsResponses(view.Sections),
		"rules":    toQuestionRuleResponses(view.Rules),
	}
	if view.DraftVersion != nil {
		resp["draft_version"] = toRfxVersionResponse(view.DraftVersion)
	} else {
		resp["draft_version"] = nil
	}
	return resp
}

func toQuestionnaireDefinitionResponse(def *domain.QuestionnaireDefinition) map[string]any {
	return map[string]any{
		"event_id":              def.EventID.String(),
		"rfx_version_id":        def.RfxVersionID.String(),
		"version_number":        def.VersionNumber,
		"questionnaire_enabled": def.QuestionnaireEnabled,
		"version_status":        def.VersionStatus,
		"sections":              toSectionWithQuestionsResponses(def.Sections),
		"rules":                 toQuestionRuleResponses(def.Rules),
	}
}

func toRfxVersionResponse(version *domain.RfxVersion) map[string]any {
	resp := map[string]any{
		"id":                    version.ID.String(),
		"tenant_id":             version.TenantID.String(),
		"rfx_event_id":          version.RfxEventID.String(),
		"version_number":        version.VersionNumber,
		"status":                version.Status,
		"questionnaire_enabled": version.QuestionnaireEnabled,
		"published_at":          formatDateTime(version.PublishedAt),
		"created_at":            version.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at":            version.UpdatedAt.UTC().Format(time.RFC3339),
		"version":               version.Version,
	}
	if version.PublishedBy != nil {
		resp["published_by"] = version.PublishedBy.String()
	} else {
		resp["published_by"] = nil
	}
	return resp
}

func toSectionResponse(section *domain.Section) map[string]any {
	return map[string]any{
		"id":             section.ID.String(),
		"tenant_id":      section.TenantID.String(),
		"rfx_version_id": section.RfxVersionID.String(),
		"section_code":   section.SectionCode,
		"title":          section.Title,
		"description":    section.Description,
		"sort_order":     section.SortOrder,
		"created_at":     section.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at":     section.UpdatedAt.UTC().Format(time.RFC3339),
		"version":        section.Version,
	}
}

func toQuestionResponse(question *domain.Question) map[string]any {
	var validation any
	if len(question.ValidationRuleJSON) > 0 {
		_ = json.Unmarshal(question.ValidationRuleJSON, &validation)
	} else {
		validation = map[string]any{}
	}
	opts := make([]map[string]any, 0, len(question.Options))
	for i := range question.Options {
		opts = append(opts, toQuestionOptionResponse(&question.Options[i]))
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
		"validation_rule_json": validation,
		"sort_order":           question.SortOrder,
		"created_at":           question.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at":           question.UpdatedAt.UTC().Format(time.RFC3339),
		"version":              question.Version,
		"options":              opts,
	}
}

func toQuestionOptionResponse(option *domain.QuestionOption) map[string]any {
	return map[string]any{
		"id":          option.ID.String(),
		"tenant_id":   option.TenantID.String(),
		"question_id": option.QuestionID.String(),
		"option_code": option.OptionCode,
		"label":       option.Label,
		"sort_order":  option.SortOrder,
		"created_at":  option.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at":  option.UpdatedAt.UTC().Format(time.RFC3339),
		"version":     option.Version,
	}
}

func toQuestionRuleResponse(rule *domain.QuestionRule) map[string]any {
	var condition any
	if len(rule.ConditionJSON) > 0 {
		_ = json.Unmarshal(rule.ConditionJSON, &condition)
	} else {
		condition = map[string]any{}
	}
	var targetID any
	if rule.TargetQuestionID != nil {
		targetID = rule.TargetQuestionID.String()
	}
	return map[string]any{
		"id":                 rule.ID.String(),
		"tenant_id":          rule.TenantID.String(),
		"rfx_version_id":     rule.RfxVersionID.String(),
		"target_question_id": targetID,
		"rule_code":          rule.RuleCode,
		"action":             rule.Action,
		"condition_json":     condition,
		"sort_order":         rule.SortOrder,
		"created_at":         rule.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at":         rule.UpdatedAt.UTC().Format(time.RFC3339),
		"version":            rule.Version,
	}
}

func toSectionWithQuestionsResponses(sections []domain.SectionWithQuestions) []map[string]any {
	out := make([]map[string]any, 0, len(sections))
	for _, swq := range sections {
		questions := make([]map[string]any, 0, len(swq.Questions))
		for i := range swq.Questions {
			questions = append(questions, toQuestionResponse(&swq.Questions[i]))
		}
		out = append(out, map[string]any{
			"section":   toSectionResponse(&swq.Section),
			"questions": questions,
		})
	}
	return out
}

func toQuestionRuleResponses(rules []domain.QuestionRule) []map[string]any {
	out := make([]map[string]any, 0, len(rules))
	for i := range rules {
		out = append(out, toQuestionRuleResponse(&rules[i]))
	}
	return out
}

func toPublishReadinessResponse(result *domain.PublishReadinessResult) map[string]any {
	items := make([]map[string]any, 0, len(result.Items))
	for _, item := range result.Items {
		entry := map[string]any{
			"code":    item.Code,
			"status":  item.Status,
			"message": item.Message,
		}
		if len(item.Details) > 0 {
			entry["details"] = item.Details
		}
		items = append(items, entry)
	}
	return map[string]any{
		"ready":               result.Ready,
		"blocking_fail_count": result.BlockingFail,
		"warning_count":       result.WarningCount,
		"items":               items,
	}
}
