package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

func CopyTemplateGraphToEventVersion(
	ctx context.Context,
	tmplQRepo *TemplateQuestionnaireRepository,
	qRepo *QuestionnaireRepository,
	tenantID, templateID, sourceTemplateVersionID, targetEventVersionID uuid.UUID,
) (map[uuid.UUID]uuid.UUID, error) {
	definition, err := tmplQRepo.LoadQuestionnaire(ctx, templateID, sourceTemplateVersionID, tenantID)
	if err != nil {
		return nil, err
	}

	questionIDMap := make(map[uuid.UUID]uuid.UUID)
	for _, sectionWithQuestions := range definition.Sections {
		section, err := qRepo.CreateSection(ctx, tenantID, targetEventVersionID, domain.CreateSectionInput{
			SectionCode: sectionWithQuestions.Section.SectionCode,
			Title:       sectionWithQuestions.Section.Title,
			Description: sectionWithQuestions.Section.Description,
			SortOrder:   intPtr(sectionWithQuestions.Section.SortOrder),
		})
		if err != nil {
			return nil, err
		}
		for _, sourceQuestion := range sectionWithQuestions.Questions {
			question, err := qRepo.CreateQuestion(ctx, tenantID, section.ID, domain.CreateQuestionInput{
				QuestionCode:       sourceQuestion.QuestionCode,
				QuestionType:       sourceQuestion.QuestionType,
				Label:              sourceQuestion.Label,
				HelpText:           sourceQuestion.HelpText,
				Required:           sourceQuestion.Required,
				ValidationRuleJSON: sourceQuestion.ValidationRuleJSON,
				SortOrder:          intPtr(sourceQuestion.SortOrder),
			})
			if err != nil {
				return nil, err
			}
			questionIDMap[sourceQuestion.ID] = question.ID
			for _, option := range sourceQuestion.Options {
				if _, err := qRepo.CreateOption(ctx, tenantID, question.ID, domain.CreateQuestionOptionInput{
					OptionCode: option.OptionCode,
					Label:      option.Label,
					SortOrder:  intPtr(option.SortOrder),
				}); err != nil {
					return nil, err
				}
			}
		}
	}

	for _, rule := range definition.Rules {
		var targetQuestionID *uuid.UUID
		if rule.TargetQuestionID != nil {
			mappedID, ok := questionIDMap[*rule.TargetQuestionID]
			if !ok {
				return nil, apperrors.Internal("failed to map copied rule target question", nil)
			}
			targetQuestionID = &mappedID
		}
		if _, err := qRepo.CreateQuestionRule(ctx, tenantID, targetEventVersionID, targetQuestionID, domain.CreateQuestionRuleInput{
			RuleCode:      rule.RuleCode,
			Action:        rule.Action,
			ConditionJSON: rule.ConditionJSON,
			SortOrder:     intPtr(rule.SortOrder),
		}); err != nil {
			return nil, err
		}
	}

	return questionIDMap, nil
}
