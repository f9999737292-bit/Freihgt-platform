package xlsxexchange

import (
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
)

func validateProposalGraph(target TargetDraftBaseline, proposal BuyerImportProposal, issues *issueCollector) {
	sections := proposal.Questionnaire.Sections
	rules := proposal.Questionnaire.Rules

	questionCodes := make(map[string]struct{})
	questionCodeByID := make(map[uuid.UUID]string)
	questionTypeByCode := make(map[string]string)
	sectionCodes := make(map[string]struct{})

	for _, swq := range sections {
		code := swq.Section.SectionCode
		if _, dup := sectionCodes[code]; dup {
			issues.addError(issueError(
				MachineCodeDuplicateStableCode,
				"rfx.buyer_xlsx_import.duplicate_section_code",
				sheetSections, "section_code", code, 0, nil,
			))
		}
		sectionCodes[code] = struct{}{}
		if err := domain.ValidateSectionCode(code); err != nil {
			issues.addError(issueError(
				MachineCodeInvalidType,
				"rfx.buyer_xlsx_import.invalid_section_code",
				sheetSections, "section_code", code, 0, nil,
			))
		}
		if strings.TrimSpace(swq.Section.Title) == "" {
			issues.addError(issueError(
				MachineCodeMissingRequiredValue,
				"rfx.buyer_xlsx_import.missing_section_title",
				sheetSections, "title_ru", code, 0, nil,
			))
		}

		for _, q := range swq.Questions {
			if _, dup := questionCodes[q.QuestionCode]; dup {
				issues.addError(issueError(
					MachineCodeDuplicateStableCode,
					"rfx.buyer_xlsx_import.duplicate_question_code",
					sheetQuestions, "question_code", q.QuestionCode, 0, nil,
				))
			}
			questionCodes[q.QuestionCode] = struct{}{}
			questionCodeByID[q.ID] = q.QuestionCode
			questionTypeByCode[q.QuestionCode] = q.QuestionType

			if err := domain.ValidateQuestionCode(q.QuestionCode); err != nil {
				issues.addError(issueError(
					MachineCodeInvalidType,
					"rfx.buyer_xlsx_import.invalid_question_code",
					sheetQuestions, "question_code", q.QuestionCode, 0, nil,
				))
			}
			if err := domain.ValidateQuestionType(q.QuestionType); err != nil {
				issues.addError(issueError(
					MachineCodeInvalidType,
					"rfx.buyer_xlsx_import.invalid_question_type",
					sheetQuestions, "question_type", q.QuestionCode, 0,
					map[string]any{"value": q.QuestionType},
				))
			}
			if strings.TrimSpace(q.Label) == "" {
				issues.addError(issueError(
					MachineCodeMissingRequiredValue,
					"rfx.buyer_xlsx_import.missing_question_label",
					sheetQuestions, "title_ru", q.QuestionCode, 0, nil,
				))
			}
			if err := domain.ValidateValidationDefinition(q.QuestionType, q.ValidationRuleJSON); err != nil {
				issues.addError(issueError(
					MachineCodeInvalidType,
					"rfx.buyer_xlsx_import.invalid_validation_json",
					sheetQuestions, "validation_json", q.QuestionCode, 0, nil,
				))
			}

			seenOptionCodes := make(map[string]struct{})
			for _, opt := range q.Options {
				if _, dup := seenOptionCodes[opt.OptionCode]; dup {
					issues.addError(issueError(
						MachineCodeDuplicateStableCode,
						"rfx.buyer_xlsx_import.duplicate_option_code",
						sheetOptions, "option_code", opt.OptionCode, 0,
						map[string]any{"question_code": q.QuestionCode},
					))
				}
				seenOptionCodes[opt.OptionCode] = struct{}{}
				if err := domain.ValidateOptionCode(opt.OptionCode); err != nil {
					issues.addError(issueError(
						MachineCodeInvalidType,
						"rfx.buyer_xlsx_import.invalid_option_code",
						sheetOptions, "option_code", opt.OptionCode, 0, nil,
					))
				}
				if strings.TrimSpace(opt.Label) == "" {
					issues.addError(issueError(
						MachineCodeMissingRequiredValue,
						"rfx.buyer_xlsx_import.missing_option_label",
						sheetOptions, "label_ru", opt.OptionCode, 0, nil,
					))
				}
			}
			if domain.QuestionTypeRequiresOptions(q.QuestionType) && len(q.Options) == 0 {
				issues.addError(issueError(
					MachineCodeMissingRequiredValue,
					"rfx.buyer_xlsx_import.select_requires_options",
					sheetOptions, "option_code", q.QuestionCode, 0, nil,
				))
			}
			if !domain.QuestionTypeRequiresOptions(q.QuestionType) && len(q.Options) > 0 {
				issues.addError(issueError(
					MachineCodeInvalidType,
					"rfx.buyer_xlsx_import.options_not_allowed",
					sheetOptions, "option_code", q.QuestionCode, 0,
					map[string]any{"question_type": q.QuestionType},
				))
			}
		}
	}

	seenRuleCodes := make(map[string]struct{})
	for _, rule := range rules {
		if _, dup := seenRuleCodes[rule.RuleCode]; dup {
			issues.addError(issueError(
				MachineCodeDuplicateStableCode,
				"rfx.buyer_xlsx_import.duplicate_rule_code",
				sheetRules, "rule_code", rule.RuleCode, 0, nil,
			))
		}
		seenRuleCodes[rule.RuleCode] = struct{}{}
		if err := domain.ValidateRuleAction(rule.Action); err != nil {
			issues.addError(issueError(
				MachineCodeInvalidType,
				"rfx.buyer_xlsx_import.invalid_rule_action",
				sheetRules, "action", rule.RuleCode, 0,
				map[string]any{"value": rule.Action},
			))
		}
	}

	if err := domain.ValidateRuleSet(rules, questionCodes, questionCodeByID, questionTypeByCode); err != nil {
		mapRuleSetError(err, issues)
	}

	for _, lot := range proposal.Lots {
		if err := domain.ValidateCreateRfxLotInput(domain.CreateRfxLotInput{
			TenantID:       target.TenantID,
			RfxEventID:     target.EventID,
			LotNumber:      lot.LotNumber,
			Name:           lot.Name,
			Description:    lot.Description,
			Category:       lot.Category,
			EstimatedValue: lot.EstimatedValue,
			CurrencyCode:   lot.CurrencyCode,
		}); err != nil {
			issues.addError(issueError(
				MachineCodeInvalidType,
				"rfx.buyer_xlsx_import.invalid_lot",
				sheetLots, "lot_number", lot.LotNumber, 0, nil,
			))
		}
		if strings.TrimSpace(lot.Status) == "" {
			issues.addError(issueError(
				MachineCodeMissingRequiredValue,
				"rfx.buyer_xlsx_import.missing_lot_status",
				sheetLots, "status", lot.LotNumber, 0, nil,
			))
		}
	}
}

func mapRuleSetError(err error, issues *issueCollector) {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "references itself"):
		issues.addError(issueError(
			MachineCodeSelfTargetRule,
			"rfx.buyer_xlsx_import.self_target_rule",
			sheetRules, "target_question_code", "", 0, nil,
		))
	case strings.Contains(msg, "cycle"):
		issues.addError(issueError(
			MachineCodeCyclicRule,
			"rfx.buyer_xlsx_import.cyclic_rule",
			sheetRules, "rule_code", "", 0, nil,
		))
	case strings.Contains(msg, "unknown question"):
		issues.addError(issueError(
			MachineCodeDanglingReference,
			"rfx.buyer_xlsx_import.dangling_rule_reference",
			sheetRules, "source_question_code", "", 0, nil,
		))
	default:
		issues.addError(issueError(
			MachineCodeInvalidType,
			"rfx.buyer_xlsx_import.invalid_rule",
			sheetRules, "condition", "", 0,
			map[string]any{"reason": msg},
		))
	}
}
