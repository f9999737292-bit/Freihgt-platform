package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/repository"
)

type QuestionnaireStore interface {
	GetOrCreateDraftVersion(ctx context.Context, tenantID, eventID uuid.UUID) (*domain.RfxVersion, error)
	GetVersionByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.RfxVersion, error)
	TouchVersion(ctx context.Context, id, tenantID uuid.UUID, expectedVersion int) (*domain.RfxVersion, error)
	LoadQuestionnaireTree(ctx context.Context, versionID, tenantID uuid.UUID) ([]domain.SectionWithQuestions, error)
	ListRulesByVersion(ctx context.Context, versionID, tenantID uuid.UUID) ([]domain.QuestionRule, error)

	GetSectionByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.Section, error)
	CreateSection(ctx context.Context, tenantID, versionID uuid.UUID, in domain.CreateSectionInput) (*domain.Section, error)
	UpdateSection(ctx context.Context, id, tenantID uuid.UUID, in domain.UpdateSectionInput) (*domain.Section, error)
	DeleteSection(ctx context.Context, id, tenantID uuid.UUID, expectedVersion int) error
	ReorderSections(ctx context.Context, tenantID, versionID uuid.UUID, orderedIDs []uuid.UUID) error
	AssertSectionBelongsToVersion(ctx context.Context, sectionID, versionID, tenantID uuid.UUID) error

	GetQuestionByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.Question, error)
	CreateQuestion(ctx context.Context, tenantID, sectionID uuid.UUID, in domain.CreateQuestionInput) (*domain.Question, error)
	UpdateQuestion(ctx context.Context, id, tenantID uuid.UUID, in domain.UpdateQuestionInput) (*domain.Question, error)
	DeleteQuestion(ctx context.Context, id, tenantID uuid.UUID, expectedVersion int) error
	ReorderQuestions(ctx context.Context, tenantID, sectionID uuid.UUID, orderedIDs []uuid.UUID) error
	DuplicateQuestion(ctx context.Context, tenantID, questionID uuid.UUID, newCode string) (*domain.Question, error)
	AssertQuestionBelongsToVersion(ctx context.Context, questionID, versionID, tenantID uuid.UUID) error

	GetQuestionOptionByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.QuestionOption, error)
	CreateQuestionOption(ctx context.Context, tenantID, questionID uuid.UUID, in domain.CreateQuestionOptionInput) (*domain.QuestionOption, error)
	UpdateQuestionOption(ctx context.Context, id, tenantID uuid.UUID, in domain.UpdateQuestionOptionInput) (*domain.QuestionOption, error)
	DeleteQuestionOption(ctx context.Context, id, tenantID uuid.UUID, expectedVersion int) error

	GetQuestionRuleByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.QuestionRule, error)
	CreateQuestionRule(ctx context.Context, tenantID, versionID uuid.UUID, targetQuestionID *uuid.UUID, in domain.CreateQuestionRuleInput) (*domain.QuestionRule, error)
	UpdateQuestionRule(ctx context.Context, id, tenantID uuid.UUID, targetQuestionID *uuid.UUID, in domain.UpdateQuestionRuleInput) (*domain.QuestionRule, error)
	DeleteQuestionRule(ctx context.Context, id, tenantID uuid.UUID, expectedVersion int) error
	GetQuestionIDByCodeInVersion(ctx context.Context, versionID, tenantID uuid.UUID, questionCode string) (*uuid.UUID, error)
}

type QuestionnaireService struct {
	rfxRepo RfxStore
	repo    QuestionnaireStore
	audit   AuditRecorder
	actors  ActorResolver
}

func NewQuestionnaireService(rfxRepo RfxStore, repo QuestionnaireStore, audit AuditRecorder, actors ActorResolver) *QuestionnaireService {
	return &QuestionnaireService{rfxRepo: rfxRepo, repo: repo, audit: audit, actors: actors}
}

func (s *QuestionnaireService) GetStudio(ctx context.Context, actor domain.ActorContext, eventID uuid.UUID) (*domain.QuestionnaireStudio, error) {
	event, version, err := s.loadDraftContext(ctx, actor, eventID)
	if err != nil {
		return nil, err
	}
	sections, err := s.repo.LoadQuestionnaireTree(ctx, version.ID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	rules, err := s.repo.ListRulesByVersion(ctx, version.ID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	return &domain.QuestionnaireStudio{
		Event:        *event,
		DraftVersion: version,
		Sections:     sections,
		Rules:        rules,
	}, nil
}

func (s *QuestionnaireService) GetQuestionnaire(ctx context.Context, actor domain.ActorContext, eventID uuid.UUID) (*domain.QuestionnaireDefinition, error) {
	event, version, err := s.loadDraftContext(ctx, actor, eventID)
	if err != nil {
		return nil, err
	}
	sections, err := s.repo.LoadQuestionnaireTree(ctx, version.ID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	rules, err := s.repo.ListRulesByVersion(ctx, version.ID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	return &domain.QuestionnaireDefinition{
		EventID:              event.ID,
		RfxVersionID:         version.ID,
		VersionNumber:        version.VersionNumber,
		QuestionnaireEnabled: version.QuestionnaireEnabled,
		VersionStatus:        version.Status,
		Sections:             sections,
		Rules:                rules,
	}, nil
}

func (s *QuestionnaireService) SaveDraft(ctx context.Context, actor domain.ActorContext, eventID uuid.UUID, expectedVersion int) (*domain.RfxVersion, error) {
	event, err := s.authorizeEvent(ctx, actor, eventID)
	if err != nil {
		return nil, err
	}
	version, err := s.repo.GetOrCreateDraftVersion(ctx, actor.TenantID, event.ID)
	if err != nil {
		return nil, err
	}
	if err := domain.EnsureDraftVersionMutable(version.Status); err != nil {
		return nil, err
	}
	if expectedVersion > 0 && version.Version != expectedVersion {
		return nil, apperrors.Conflict("questionnaire version was updated by another request", map[string]any{"field": "version"})
	}
	touched, err := s.repo.TouchVersion(ctx, version.ID, actor.TenantID, version.Version)
	if err != nil {
		return nil, err
	}
	if err := recordAudit(ctx, s.audit, actor, event.OwnerCompanyID, "rfx_questionnaire", version.ID, "save_draft", nil); err != nil {
		return nil, err
	}
	return touched, nil
}

func (s *QuestionnaireService) ValidatePublish(ctx context.Context, actor domain.ActorContext, eventID uuid.UUID) (*domain.PublishReadinessResult, error) {
	event, version, err := s.loadDraftContext(ctx, actor, eventID)
	if err != nil {
		return nil, err
	}
	sections, err := s.repo.LoadQuestionnaireTree(ctx, version.ID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	rules, err := s.repo.ListRulesByVersion(ctx, version.ID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	result := domain.EvaluatePublishReadiness(*version, sections, rules)
	if err := recordAudit(ctx, s.audit, actor, event.OwnerCompanyID, "rfx_questionnaire", version.ID, "validate_publish", map[string]any{
		"ready":               result.Ready,
		"blocking_fail_count": result.BlockingFail,
	}); err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *QuestionnaireService) CreateSection(ctx context.Context, actor domain.ActorContext, eventID uuid.UUID, in domain.CreateSectionInput) (*domain.Section, error) {
	if err := domain.ValidateCreateSectionInput(in); err != nil {
		return nil, err
	}
	event, version, err := s.loadMutableDraft(ctx, actor, eventID)
	if err != nil {
		return nil, err
	}
	section, err := s.repo.CreateSection(ctx, actor.TenantID, version.ID, in)
	if err != nil {
		return nil, err
	}
	if err := recordAudit(ctx, s.audit, actor, event.OwnerCompanyID, "rfx_section", section.ID, "create", map[string]any{"section_code": section.SectionCode}); err != nil {
		return nil, err
	}
	return section, nil
}

func (s *QuestionnaireService) UpdateSection(ctx context.Context, actor domain.ActorContext, eventID, sectionID uuid.UUID, in domain.UpdateSectionInput) (*domain.Section, error) {
	event, version, err := s.loadMutableDraft(ctx, actor, eventID)
	if err != nil {
		return nil, err
	}
	if err := s.repo.AssertSectionBelongsToVersion(ctx, sectionID, version.ID, actor.TenantID); err != nil {
		return nil, err
	}
	section, err := s.repo.UpdateSection(ctx, sectionID, actor.TenantID, in)
	if err != nil {
		return nil, err
	}
	if err := recordAudit(ctx, s.audit, actor, event.OwnerCompanyID, "rfx_section", section.ID, "update", nil); err != nil {
		return nil, err
	}
	return section, nil
}

func (s *QuestionnaireService) DeleteSection(ctx context.Context, actor domain.ActorContext, eventID, sectionID uuid.UUID, expectedVersion int) error {
	event, version, err := s.loadMutableDraft(ctx, actor, eventID)
	if err != nil {
		return err
	}
	if err := s.repo.AssertSectionBelongsToVersion(ctx, sectionID, version.ID, actor.TenantID); err != nil {
		return err
	}
	if err := s.repo.DeleteSection(ctx, sectionID, actor.TenantID, expectedVersion); err != nil {
		return err
	}
	return recordAudit(ctx, s.audit, actor, event.OwnerCompanyID, "rfx_section", sectionID, "delete", nil)
}

func (s *QuestionnaireService) ReorderSections(ctx context.Context, actor domain.ActorContext, eventID uuid.UUID, orderedIDs []uuid.UUID) error {
	event, version, err := s.loadMutableDraft(ctx, actor, eventID)
	if err != nil {
		return err
	}
	if len(orderedIDs) == 0 {
		return apperrors.Validation("ordered_ids is required", map[string]any{"field": "ordered_ids"})
	}
	if err := s.repo.ReorderSections(ctx, actor.TenantID, version.ID, orderedIDs); err != nil {
		return err
	}
	return recordAudit(ctx, s.audit, actor, event.OwnerCompanyID, "rfx_questionnaire", version.ID, "reorder_sections", nil)
}

func (s *QuestionnaireService) CreateQuestion(ctx context.Context, actor domain.ActorContext, eventID, sectionID uuid.UUID, in domain.CreateQuestionInput) (*domain.Question, error) {
	if err := domain.ValidateCreateQuestionInput(in); err != nil {
		return nil, err
	}
	event, version, err := s.loadMutableDraft(ctx, actor, eventID)
	if err != nil {
		return nil, err
	}
	if err := s.repo.AssertSectionBelongsToVersion(ctx, sectionID, version.ID, actor.TenantID); err != nil {
		return nil, err
	}
	question, err := s.repo.CreateQuestion(ctx, actor.TenantID, sectionID, in)
	if err != nil {
		return nil, err
	}
	if err := recordAudit(ctx, s.audit, actor, event.OwnerCompanyID, "rfx_question", question.ID, "create", map[string]any{"question_code": question.QuestionCode}); err != nil {
		return nil, err
	}
	return question, nil
}

func (s *QuestionnaireService) UpdateQuestion(ctx context.Context, actor domain.ActorContext, eventID, questionID uuid.UUID, in domain.UpdateQuestionInput) (*domain.Question, error) {
	if in.QuestionType != nil {
		if err := domain.ValidateQuestionType(*in.QuestionType); err != nil {
			return nil, err
		}
	}
	if len(in.ValidationRuleJSON) > 0 {
		qType := ""
		if in.QuestionType != nil {
			qType = *in.QuestionType
		} else {
			current, err := s.repo.GetQuestionByID(ctx, questionID, actor.TenantID)
			if err != nil {
				return nil, err
			}
			qType = current.QuestionType
		}
		if err := domain.ValidateValidationDefinition(qType, in.ValidationRuleJSON); err != nil {
			return nil, err
		}
	}
	event, version, err := s.loadMutableDraft(ctx, actor, eventID)
	if err != nil {
		return nil, err
	}
	if err := s.repo.AssertQuestionBelongsToVersion(ctx, questionID, version.ID, actor.TenantID); err != nil {
		return nil, err
	}
	question, err := s.repo.UpdateQuestion(ctx, questionID, actor.TenantID, in)
	if err != nil {
		return nil, err
	}
	if err := recordAudit(ctx, s.audit, actor, event.OwnerCompanyID, "rfx_question", question.ID, "update", nil); err != nil {
		return nil, err
	}
	return question, nil
}

func (s *QuestionnaireService) DeleteQuestion(ctx context.Context, actor domain.ActorContext, eventID, questionID uuid.UUID, expectedVersion int) error {
	event, version, err := s.loadMutableDraft(ctx, actor, eventID)
	if err != nil {
		return err
	}
	if err := s.repo.AssertQuestionBelongsToVersion(ctx, questionID, version.ID, actor.TenantID); err != nil {
		return err
	}
	if err := s.repo.DeleteQuestion(ctx, questionID, actor.TenantID, expectedVersion); err != nil {
		return err
	}
	return recordAudit(ctx, s.audit, actor, event.OwnerCompanyID, "rfx_question", questionID, "delete", nil)
}

func (s *QuestionnaireService) ReorderQuestions(ctx context.Context, actor domain.ActorContext, eventID, sectionID uuid.UUID, orderedIDs []uuid.UUID) error {
	event, version, err := s.loadMutableDraft(ctx, actor, eventID)
	if err != nil {
		return err
	}
	if err := s.repo.AssertSectionBelongsToVersion(ctx, sectionID, version.ID, actor.TenantID); err != nil {
		return err
	}
	if len(orderedIDs) == 0 {
		return apperrors.Validation("ordered_ids is required", map[string]any{"field": "ordered_ids"})
	}
	if err := s.repo.ReorderQuestions(ctx, actor.TenantID, sectionID, orderedIDs); err != nil {
		return err
	}
	return recordAudit(ctx, s.audit, actor, event.OwnerCompanyID, "rfx_questionnaire", version.ID, "reorder_questions", map[string]any{"section_id": sectionID.String()})
}

func (s *QuestionnaireService) DuplicateQuestion(ctx context.Context, actor domain.ActorContext, eventID, questionID uuid.UUID) (*domain.Question, error) {
	event, version, err := s.loadMutableDraft(ctx, actor, eventID)
	if err != nil {
		return nil, err
	}
	if err := s.repo.AssertQuestionBelongsToVersion(ctx, questionID, version.ID, actor.TenantID); err != nil {
		return nil, err
	}
	source, err := s.repo.GetQuestionByID(ctx, questionID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	newCode, err := s.nextDuplicateQuestionCode(ctx, version.ID, actor.TenantID, source.QuestionCode)
	if err != nil {
		return nil, err
	}
	question, err := s.repo.DuplicateQuestion(ctx, actor.TenantID, questionID, newCode)
	if err != nil {
		return nil, err
	}
	if err := recordAudit(ctx, s.audit, actor, event.OwnerCompanyID, "rfx_question", question.ID, "duplicate", map[string]any{"source_question_id": questionID.String()}); err != nil {
		return nil, err
	}
	return question, nil
}

func (s *QuestionnaireService) CreateOption(ctx context.Context, actor domain.ActorContext, eventID, questionID uuid.UUID, in domain.CreateQuestionOptionInput) (*domain.QuestionOption, error) {
	if err := domain.ValidateCreateQuestionOptionInput(in); err != nil {
		return nil, err
	}
	event, version, err := s.loadMutableDraft(ctx, actor, eventID)
	if err != nil {
		return nil, err
	}
	if err := s.repo.AssertQuestionBelongsToVersion(ctx, questionID, version.ID, actor.TenantID); err != nil {
		return nil, err
	}
	option, err := s.repo.CreateQuestionOption(ctx, actor.TenantID, questionID, in)
	if err != nil {
		return nil, err
	}
	if err := recordAudit(ctx, s.audit, actor, event.OwnerCompanyID, "rfx_question_option", option.ID, "create", map[string]any{"option_code": option.OptionCode}); err != nil {
		return nil, err
	}
	return option, nil
}

func (s *QuestionnaireService) UpdateOption(ctx context.Context, actor domain.ActorContext, eventID, optionID uuid.UUID, in domain.UpdateQuestionOptionInput) (*domain.QuestionOption, error) {
	event, version, err := s.loadMutableDraft(ctx, actor, eventID)
	if err != nil {
		return nil, err
	}
	option, err := s.repo.GetQuestionOptionByID(ctx, optionID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	if err := s.repo.AssertQuestionBelongsToVersion(ctx, option.QuestionID, version.ID, actor.TenantID); err != nil {
		return nil, err
	}
	updated, err := s.repo.UpdateQuestionOption(ctx, optionID, actor.TenantID, in)
	if err != nil {
		return nil, err
	}
	if err := recordAudit(ctx, s.audit, actor, event.OwnerCompanyID, "rfx_question_option", updated.ID, "update", nil); err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *QuestionnaireService) DeleteOption(ctx context.Context, actor domain.ActorContext, eventID, optionID uuid.UUID, expectedVersion int) error {
	event, version, err := s.loadMutableDraft(ctx, actor, eventID)
	if err != nil {
		return err
	}
	option, err := s.repo.GetQuestionOptionByID(ctx, optionID, actor.TenantID)
	if err != nil {
		return err
	}
	if err := s.repo.AssertQuestionBelongsToVersion(ctx, option.QuestionID, version.ID, actor.TenantID); err != nil {
		return err
	}
	if err := s.repo.DeleteQuestionOption(ctx, optionID, actor.TenantID, expectedVersion); err != nil {
		return err
	}
	return recordAudit(ctx, s.audit, actor, event.OwnerCompanyID, "rfx_question_option", optionID, "delete", nil)
}

func (s *QuestionnaireService) CreateRule(ctx context.Context, actor domain.ActorContext, eventID uuid.UUID, in domain.CreateQuestionRuleInput) (*domain.QuestionRule, error) {
	if err := domain.ValidateCreateQuestionRuleInput(in); err != nil {
		return nil, err
	}
	event, version, err := s.loadMutableDraft(ctx, actor, eventID)
	if err != nil {
		return nil, err
	}
	var targetID *uuid.UUID
	if in.TargetQuestionCode != nil && strings.TrimSpace(*in.TargetQuestionCode) != "" {
		targetID, err = s.repo.GetQuestionIDByCodeInVersion(ctx, version.ID, actor.TenantID, *in.TargetQuestionCode)
		if err != nil {
			return nil, err
		}
	}
	proposed := domain.QuestionRule{
		TargetQuestionID: targetID,
		RuleCode:         strings.TrimSpace(in.RuleCode),
		Action:           strings.TrimSpace(in.Action),
		ConditionJSON:    in.ConditionJSON,
	}
	if err := s.validateRulesIncluding(ctx, version.ID, actor.TenantID, proposed, nil); err != nil {
		return nil, err
	}
	rule, err := s.repo.CreateQuestionRule(ctx, actor.TenantID, version.ID, targetID, in)
	if err != nil {
		return nil, err
	}
	if err := recordAudit(ctx, s.audit, actor, event.OwnerCompanyID, "rfx_question_rule", rule.ID, "create", map[string]any{"rule_code": rule.RuleCode}); err != nil {
		return nil, err
	}
	return rule, nil
}

func (s *QuestionnaireService) UpdateRule(ctx context.Context, actor domain.ActorContext, eventID, ruleID uuid.UUID, in domain.UpdateQuestionRuleInput) (*domain.QuestionRule, error) {
	if in.Action != nil {
		if err := domain.ValidateRuleAction(*in.Action); err != nil {
			return nil, err
		}
	}
	if len(in.ConditionJSON) > 0 {
		if err := domain.ValidateConditionalExpression(in.ConditionJSON); err != nil {
			return nil, err
		}
	}
	event, version, err := s.loadMutableDraft(ctx, actor, eventID)
	if err != nil {
		return nil, err
	}
	current, err := s.repo.GetQuestionRuleByID(ctx, ruleID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	if current.RfxVersionID != version.ID {
		return nil, apperrors.NotFound("question rule not found")
	}
	var targetID *uuid.UUID
	if in.TargetQuestionCode != nil {
		code := strings.TrimSpace(*in.TargetQuestionCode)
		if code == "" {
			targetID = nil
		} else {
			targetID, err = s.repo.GetQuestionIDByCodeInVersion(ctx, version.ID, actor.TenantID, code)
			if err != nil {
				return nil, err
			}
		}
	} else {
		targetID = current.TargetQuestionID
	}
	action := current.Action
	if in.Action != nil {
		action = *in.Action
	}
	condition := current.ConditionJSON
	if len(in.ConditionJSON) > 0 {
		condition = in.ConditionJSON
	}
	proposed := domain.QuestionRule{
		ID:               current.ID,
		TargetQuestionID: targetID,
		RuleCode:         current.RuleCode,
		Action:           action,
		ConditionJSON:    condition,
	}
	if err := s.validateRulesIncluding(ctx, version.ID, actor.TenantID, proposed, &ruleID); err != nil {
		return nil, err
	}
	updated, err := s.repo.UpdateQuestionRule(ctx, ruleID, actor.TenantID, targetID, in)
	if err != nil {
		return nil, err
	}
	if err := recordAudit(ctx, s.audit, actor, event.OwnerCompanyID, "rfx_question_rule", updated.ID, "update", nil); err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *QuestionnaireService) DeleteRule(ctx context.Context, actor domain.ActorContext, eventID, ruleID uuid.UUID, expectedVersion int) error {
	event, version, err := s.loadMutableDraft(ctx, actor, eventID)
	if err != nil {
		return err
	}
	rule, err := s.repo.GetQuestionRuleByID(ctx, ruleID, actor.TenantID)
	if err != nil {
		return err
	}
	if rule.RfxVersionID != version.ID {
		return apperrors.NotFound("question rule not found")
	}
	if err := s.repo.DeleteQuestionRule(ctx, ruleID, actor.TenantID, expectedVersion); err != nil {
		return err
	}
	return recordAudit(ctx, s.audit, actor, event.OwnerCompanyID, "rfx_question_rule", ruleID, "delete", nil)
}

func (s *QuestionnaireService) validateRulesIncluding(ctx context.Context, versionID, tenantID uuid.UUID, include domain.QuestionRule, excludeID *uuid.UUID) error {
	sections, err := s.repo.LoadQuestionnaireTree(ctx, versionID, tenantID)
	if err != nil {
		return err
	}
	rules, err := s.repo.ListRulesByVersion(ctx, versionID, tenantID)
	if err != nil {
		return err
	}
	merged := make([]domain.QuestionRule, 0, len(rules)+1)
	for _, rule := range rules {
		if excludeID != nil && rule.ID == *excludeID {
			continue
		}
		merged = append(merged, rule)
	}
	merged = append(merged, include)
	return domain.ValidateQuestionnaireDefinition(sections, merged)
}

func (s *QuestionnaireService) nextDuplicateQuestionCode(ctx context.Context, versionID, tenantID uuid.UUID, baseCode string) (string, error) {
	sections, err := s.repo.LoadQuestionnaireTree(ctx, versionID, tenantID)
	if err != nil {
		return "", err
	}
	existing := make(map[string]struct{})
	for _, swq := range sections {
		for _, q := range swq.Questions {
			existing[q.QuestionCode] = struct{}{}
		}
	}
	candidate := baseCode + "_copy"
	for i := 2; ; i++ {
		if _, ok := existing[candidate]; !ok {
			return candidate, nil
		}
		candidate = fmt.Sprintf("%s_copy%d", baseCode, i)
		if i > 1000 {
			return "", apperrors.Internal("failed to generate duplicate question code", nil)
		}
	}
}

func (s *QuestionnaireService) loadDraftContext(ctx context.Context, actor domain.ActorContext, eventID uuid.UUID) (*domain.RfxEvent, *domain.RfxVersion, error) {
	event, err := s.authorizeEvent(ctx, actor, eventID)
	if err != nil {
		return nil, nil, err
	}
	version, err := s.repo.GetOrCreateDraftVersion(ctx, actor.TenantID, event.ID)
	if err != nil {
		return nil, nil, err
	}
	return event, version, nil
}

func (s *QuestionnaireService) loadMutableDraft(ctx context.Context, actor domain.ActorContext, eventID uuid.UUID) (*domain.RfxEvent, *domain.RfxVersion, error) {
	event, version, err := s.loadDraftContext(ctx, actor, eventID)
	if err != nil {
		return nil, nil, err
	}
	if err := domain.EnsureDraftVersionMutable(version.Status); err != nil {
		return nil, nil, err
	}
	return event, version, nil
}

func (s *QuestionnaireService) authorizeEvent(ctx context.Context, actor domain.ActorContext, eventID uuid.UUID) (*domain.RfxEvent, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	event, err := s.rfxRepo.GetEventByID(ctx, eventID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	if err := s.requireOwnerCompanyAccessForEvent(ctx, actor, event); err != nil {
		return nil, err
	}
	return event, nil
}

func (s *QuestionnaireService) requireOwnerCompanyAccessForEvent(ctx context.Context, actor domain.ActorContext, event *domain.RfxEvent) error {
	if err := s.requireBuyerActor(ctx, actor); err != nil {
		return err
	}
	buyerCompanyIDs, err := s.listBuyerCompanyIDs(ctx, actor)
	if err != nil {
		return err
	}
	if !domain.ContainsCompanyID(buyerCompanyIDs, event.OwnerCompanyID) {
		return apperrors.NotFound("rfx event not found")
	}
	return nil
}

func (s *QuestionnaireService) requireBuyerActor(ctx context.Context, actor domain.ActorContext) error {
	if actor.UserID == uuid.Nil {
		return apperrors.Forbidden("user context is required")
	}
	resolver, ok := s.actors.(CompanyMembershipResolver)
	if ok {
		roles, err := resolver.ListUserRoleCodes(ctx, actor.TenantID, actor.UserID)
		if err != nil {
			return err
		}
		if !domain.HasBuyerRole(roles) {
			return apperrors.Forbidden("buyer role is required")
		}
	}
	return nil
}

func (s *QuestionnaireService) listBuyerCompanyIDs(ctx context.Context, actor domain.ActorContext) ([]uuid.UUID, error) {
	if s.actors == nil {
		return nil, apperrors.Forbidden("buyer company membership is required")
	}
	resolver, ok := s.actors.(CompanyMembershipResolver)
	if !ok {
		return nil, apperrors.Forbidden("buyer company membership is required")
	}
	return resolver.ListBuyerCompanyIDs(ctx, actor)
}

var _ QuestionnaireStore = (*repository.QuestionnaireRepository)(nil)
