package service

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/repository"
	"github.com/freight-platform/rfx-service/internal/xlsxexchange"
)

type entityChangeCounts struct {
	Added   int `json:"added"`
	Updated int `json:"updated"`
	Deleted int `json:"deleted"`
}

type buyerImportCommitChanges struct {
	Lots      entityChangeCounts `json:"lots"`
	Sections  entityChangeCounts `json:"sections"`
	Questions entityChangeCounts `json:"questions"`
	Options   entityChangeCounts `json:"options"`
	Rules     entityChangeCounts `json:"rules"`
}

func reconcileImportLots(
	ctx context.Context,
	rfxRepo *repository.RfxRepository,
	tenantID, eventID uuid.UUID,
	stored xlsxexchange.StoredImportPayload,
	counts *entityChangeCounts,
) error {
	// Caller must already hold the canonical event lot-mutation lock in the same transaction.
	proposalLots := stored.Lots
	existing, err := rfxRepo.ListLotsByEvent(ctx, eventID, tenantID)
	if err != nil {
		return err
	}
	existingByNumber := make(map[string]domain.RfxLot, len(existing))
	for _, lot := range existing {
		existingByNumber[strings.TrimSpace(lot.LotNumber)] = lot
	}
	proposedNumbers := make(map[string]struct{}, len(proposalLots))
	for _, lot := range proposalLots {
		lotNumber := strings.TrimSpace(lot.LotNumber)
		proposedNumbers[lotNumber] = struct{}{}
		in := domain.CreateRfxLotInput{
			TenantID:       tenantID,
			RfxEventID:     eventID,
			LotNumber:      lotNumber,
			Name:           lot.Name,
			Description:    optionalStringField(lot.Description),
			Category:       optionalStringField(lot.Category),
			EstimatedValue: lot.EstimatedValue,
			CurrencyCode:   optionalStringField(lot.CurrencyCode),
		}
		status := strings.TrimSpace(lot.Status)
		if status == "" {
			status = "ACTIVE"
		}
		existingLot, deleted, err := rfxRepo.GetLotByEventAndNumber(ctx, eventID, tenantID, lotNumber)
		if err != nil {
			return err
		}
		if existingLot == nil {
			created, err := rfxRepo.CreateLot(ctx, in)
			if err != nil {
				return err
			}
			if status != "ACTIVE" {
				if _, err := rfxRepo.UpdateLotByEventAndNumber(ctx, eventID, tenantID, lotNumber, in, status); err != nil {
					return err
				}
			}
			_ = created
			counts.Added++
			continue
		}
		if _, err := rfxRepo.UpdateLotByEventAndNumber(ctx, eventID, tenantID, lotNumber, in, status); err != nil {
			return err
		}
		_ = deleted
		counts.Updated++
	}
	for lotNumber := range existingByNumber {
		if _, keep := proposedNumbers[lotNumber]; keep {
			continue
		}
		if err := rfxRepo.SoftDeleteLotByEventAndNumber(ctx, eventID, tenantID, lotNumber); err != nil {
			return err
		}
		counts.Deleted++
	}
	return nil
}

func reconcileImportQuestionnaire(
	ctx context.Context,
	qRepo *repository.QuestionnaireRepository,
	tenantID, versionID uuid.UUID,
	stored xlsxexchange.StoredImportPayload,
	counts *buyerImportCommitChanges,
) error {
	current, err := qRepo.LoadQuestionnaire(ctx, versionID, tenantID)
	if err != nil {
		return err
	}

	proposedSectionCodes := make(map[string]struct{})
	for _, sec := range stored.Questionnaire.Sections {
		proposedSectionCodes[sec.SectionCode] = struct{}{}
	}
	proposedQuestionCodes := make(map[string]struct{})
	for _, q := range stored.Questionnaire.Questions {
		proposedQuestionCodes[q.QuestionCode] = struct{}{}
	}
	proposedOptionKeys := make(map[string]struct{})
	for _, opt := range stored.Questionnaire.Options {
		proposedOptionKeys[optionKey(opt.QuestionCode, opt.OptionCode)] = struct{}{}
	}
	proposedRuleCodes := make(map[string]struct{})
	for _, rule := range stored.Questionnaire.Rules {
		proposedRuleCodes[rule.RuleCode] = struct{}{}
	}

	sectionByCode := make(map[string]domain.Section)
	questionByCode := make(map[string]domain.Question)
	optionByKey := make(map[string]domain.QuestionOption)
	ruleByCode := make(map[string]domain.QuestionRule)

	for _, swq := range current.Sections {
		sectionByCode[swq.Section.SectionCode] = swq.Section
		for _, q := range swq.Questions {
			questionByCode[q.QuestionCode] = q
			for _, opt := range q.Options {
				optionByKey[optionKey(q.QuestionCode, opt.OptionCode)] = opt
			}
		}
	}
	for _, rule := range current.Rules {
		ruleByCode[rule.RuleCode] = rule
	}

	for _, rule := range current.Rules {
		if _, keep := proposedRuleCodes[rule.RuleCode]; keep {
			continue
		}
		if err := qRepo.DeleteRule(ctx, rule.ID, tenantID, rule.Version); err != nil {
			return err
		}
		counts.Rules.Deleted++
		delete(ruleByCode, rule.RuleCode)
	}
	for _, swq := range current.Sections {
		for _, q := range swq.Questions {
			for _, opt := range q.Options {
				key := optionKey(q.QuestionCode, opt.OptionCode)
				if _, keep := proposedOptionKeys[key]; keep {
					continue
				}
				if err := qRepo.DeleteOption(ctx, opt.ID, tenantID, opt.Version); err != nil {
					return err
				}
				counts.Options.Deleted++
			}
		}
	}
	for _, swq := range current.Sections {
		for _, q := range swq.Questions {
			if _, keep := proposedQuestionCodes[q.QuestionCode]; keep {
				continue
			}
			if err := qRepo.DeleteQuestion(ctx, q.ID, tenantID, q.Version); err != nil {
				return err
			}
			counts.Questions.Deleted++
			delete(questionByCode, q.QuestionCode)
		}
	}
	for _, swq := range current.Sections {
		if _, keep := proposedSectionCodes[swq.Section.SectionCode]; keep {
			continue
		}
		if err := qRepo.DeleteSection(ctx, swq.Section.ID, tenantID, swq.Section.Version); err != nil {
			return err
		}
		counts.Sections.Deleted++
		delete(sectionByCode, swq.Section.SectionCode)
	}

	for _, sec := range stored.Questionnaire.Sections {
		desc := optionalStringField(sec.Description)
		sortOrder := sec.SortOrder
		if existing, ok := sectionByCode[sec.SectionCode]; ok {
			title := sec.Title
			if _, err := qRepo.UpdateSection(ctx, existing.ID, tenantID, domain.UpdateSectionInput{
				Title:           &title,
				Description:     desc,
				SortOrder:       &sortOrder,
				ExpectedVersion: existing.Version,
			}); err != nil {
				return err
			}
			counts.Sections.Updated++
			continue
		}
		created, err := qRepo.CreateSection(ctx, tenantID, versionID, domain.CreateSectionInput{
			SectionCode: sec.SectionCode,
			Title:       sec.Title,
			Description: desc,
			SortOrder:   &sortOrder,
		})
		if err != nil {
			return err
		}
		sectionByCode[sec.SectionCode] = *created
		counts.Sections.Added++
	}

	for _, q := range stored.Questionnaire.Questions {
		section, ok := sectionByCode[q.SectionCode]
		if !ok {
			continue
		}
		help := optionalStringField(q.HelpText)
		sortOrder := q.SortOrder
		if existing, ok := questionByCode[q.QuestionCode]; ok {
			label := q.Label
			qType := q.QuestionType
			required := q.Required
			if _, err := qRepo.UpdateQuestion(ctx, existing.ID, tenantID, domain.UpdateQuestionInput{
				QuestionType:       &qType,
				Label:              &label,
				HelpText:           help,
				Required:           &required,
				ValidationRuleJSON: q.ValidationJSON,
				SortOrder:          &sortOrder,
				ExpectedVersion:    existing.Version,
			}); err != nil {
				return err
			}
			counts.Questions.Updated++
			continue
		}
		created, err := qRepo.CreateQuestion(ctx, tenantID, section.ID, domain.CreateQuestionInput{
			QuestionCode:       q.QuestionCode,
			QuestionType:       q.QuestionType,
			Label:              q.Label,
			HelpText:           help,
			Required:           q.Required,
			ValidationRuleJSON: q.ValidationJSON,
			SortOrder:          &sortOrder,
		})
		if err != nil {
			return err
		}
		questionByCode[q.QuestionCode] = *created
		counts.Questions.Added++
	}

	for _, opt := range stored.Questionnaire.Options {
		question, ok := questionByCode[opt.QuestionCode]
		if !ok {
			continue
		}
		key := optionKey(opt.QuestionCode, opt.OptionCode)
		sortOrder := opt.SortOrder
		if existing, ok := optionByKey[key]; ok {
			label := opt.Label
			if _, err := qRepo.UpdateOption(ctx, existing.ID, tenantID, domain.UpdateQuestionOptionInput{
				Label:           &label,
				SortOrder:       &sortOrder,
				ExpectedVersion: existing.Version,
			}); err != nil {
				return err
			}
			counts.Options.Updated++
			continue
		}
		created, err := qRepo.CreateOption(ctx, tenantID, question.ID, domain.CreateQuestionOptionInput{
			OptionCode: opt.OptionCode,
			Label:      opt.Label,
			SortOrder:  &sortOrder,
		})
		if err != nil {
			return err
		}
		optionByKey[key] = *created
		counts.Options.Added++
	}

	for _, rule := range stored.Questionnaire.Rules {
		targetCode := strings.TrimSpace(rule.TargetQuestionCode)
		if targetCode == "" {
			return apperrors.Unprocessable("import rule target question is required", map[string]any{
				"machine_code": domain.MachineCodeProposalRevalidation,
				"rule_code":    rule.RuleCode,
			})
		}
		q, ok := questionByCode[targetCode]
		if !ok {
			return apperrors.Unprocessable("import rule target question unresolved", map[string]any{
				"machine_code": domain.MachineCodeProposalRevalidation,
				"rule_code":    rule.RuleCode,
				"target_code":  targetCode,
			})
		}
		targetID := q.ID
		sortOrder := rule.SortOrder
		if existing, ok := ruleByCode[rule.RuleCode]; ok {
			action := rule.Action
			if _, err := qRepo.UpdateQuestionRule(ctx, existing.ID, tenantID, &targetID, domain.UpdateQuestionRuleInput{
				Action:          &action,
				ConditionJSON:   rule.ConditionJSON,
				SortOrder:       &sortOrder,
				ExpectedVersion: existing.Version,
			}); err != nil {
				return err
			}
			counts.Rules.Updated++
			continue
		}
		if _, err := qRepo.CreateQuestionRule(ctx, tenantID, versionID, &targetID, domain.CreateQuestionRuleInput{
			RuleCode:      rule.RuleCode,
			Action:        rule.Action,
			ConditionJSON: rule.ConditionJSON,
			SortOrder:     &sortOrder,
		}); err != nil {
			return err
		}
		counts.Rules.Added++
	}
	return nil
}

func optionKey(questionCode, optionCode string) string {
	return strings.TrimSpace(questionCode) + "\x00" + strings.TrimSpace(optionCode)
}

func optionalStringField(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	out := trimmed
	return &out
}
