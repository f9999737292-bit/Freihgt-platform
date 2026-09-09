package service

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/repository"
)

type TemplateQuestionnaireService struct {
	tmplRepo  *repository.TemplateLibraryRepository
	qRepo     *repository.TemplateQuestionnaireRepository
	auditRepo *repository.AuditRepository
	tmplSvc   *TemplateLibraryService
}

func NewTemplateQuestionnaireService(
	tmplRepo *repository.TemplateLibraryRepository,
	qRepo *repository.TemplateQuestionnaireRepository,
	auditRepo *repository.AuditRepository,
	tmplSvc *TemplateLibraryService,
) *TemplateQuestionnaireService {
	return &TemplateQuestionnaireService{tmplRepo: tmplRepo, qRepo: qRepo, auditRepo: auditRepo, tmplSvc: tmplSvc}
}

func (s *TemplateQuestionnaireService) GetQuestionnaire(ctx context.Context, actor domain.ActorContext, templateID uuid.UUID) (*domain.TemplateQuestionnaireDefinition, error) {
	tmpl, draft, err := s.loadDraftContext(ctx, actor, templateID)
	if err != nil {
		return nil, err
	}
	return s.qRepo.LoadQuestionnaire(ctx, tmpl.ID, draft.ID, actor.TenantID)
}

func (s *TemplateQuestionnaireService) CreateSection(ctx context.Context, actor domain.ActorContext, templateID uuid.UUID, in domain.CreateSectionInput) (*domain.TemplateSection, error) {
	if err := domain.ValidateCreateSectionInput(in); err != nil {
		return nil, err
	}
	tmpl, draft, err := s.loadMutableDraft(ctx, actor, templateID)
	if err != nil {
		return nil, err
	}
	section, err := s.qRepo.CreateSection(ctx, actor.TenantID, tmpl.ID, draft.ID, in)
	if err != nil {
		return nil, err
	}
	if err := recordAudit(ctx, s.auditRepo, actor, ownerCompanyIDValue(tmpl.OwnerCompanyID), "rfx_template_section", section.ID, "rfx.template.section.created.v1", map[string]any{"section_code": section.SectionCode}); err != nil {
		return nil, err
	}
	return section, nil
}

func (s *TemplateQuestionnaireService) UpdateSection(ctx context.Context, actor domain.ActorContext, templateID, sectionID uuid.UUID, in domain.UpdateSectionInput) (*domain.TemplateSection, error) {
	tmpl, draft, err := s.loadMutableDraft(ctx, actor, templateID)
	if err != nil {
		return nil, err
	}
	if err := s.qRepo.AssertSectionBelongsToVersion(ctx, sectionID, tmpl.ID, draft.ID, actor.TenantID); err != nil {
		return nil, err
	}
	section, err := s.qRepo.UpdateSection(ctx, sectionID, actor.TenantID, in)
	if err != nil {
		return nil, err
	}
	if err := recordAudit(ctx, s.auditRepo, actor, ownerCompanyIDValue(tmpl.OwnerCompanyID), "rfx_template_section", section.ID, "rfx.template.section.updated.v1", nil); err != nil {
		return nil, err
	}
	return section, nil
}

func (s *TemplateQuestionnaireService) DeleteSection(ctx context.Context, actor domain.ActorContext, templateID, sectionID uuid.UUID, expectedVersion int) error {
	tmpl, draft, err := s.loadMutableDraft(ctx, actor, templateID)
	if err != nil {
		return err
	}
	if err := s.qRepo.AssertSectionBelongsToVersion(ctx, sectionID, tmpl.ID, draft.ID, actor.TenantID); err != nil {
		return err
	}
	if err := s.qRepo.DeleteSection(ctx, sectionID, actor.TenantID, expectedVersion); err != nil {
		return err
	}
	return recordAudit(ctx, s.auditRepo, actor, ownerCompanyIDValue(tmpl.OwnerCompanyID), "rfx_template_section", sectionID, "rfx.template.section.deleted.v1", nil)
}

func (s *TemplateQuestionnaireService) ReorderSections(ctx context.Context, actor domain.ActorContext, templateID uuid.UUID, orderedIDs []uuid.UUID) error {
	tmpl, draft, err := s.loadMutableDraft(ctx, actor, templateID)
	if err != nil {
		return err
	}
	if len(orderedIDs) == 0 {
		return apperrors.Validation("ordered_ids is required", map[string]any{"field": "ordered_ids"})
	}
	if err := s.qRepo.ReorderSections(ctx, actor.TenantID, tmpl.ID, draft.ID, orderedIDs); err != nil {
		return err
	}
	return recordAudit(ctx, s.auditRepo, actor, ownerCompanyIDValue(tmpl.OwnerCompanyID), "rfx_template_version", draft.ID, "rfx.template.sections.reordered.v1", nil)
}

func (s *TemplateQuestionnaireService) CreateQuestion(ctx context.Context, actor domain.ActorContext, templateID, sectionID uuid.UUID, in domain.CreateQuestionInput) (*domain.TemplateQuestion, error) {
	if err := domain.ValidateCreateQuestionInput(in); err != nil {
		return nil, err
	}
	tmpl, draft, err := s.loadMutableDraft(ctx, actor, templateID)
	if err != nil {
		return nil, err
	}
	if err := s.qRepo.AssertSectionBelongsToVersion(ctx, sectionID, tmpl.ID, draft.ID, actor.TenantID); err != nil {
		return nil, err
	}
	question, err := s.qRepo.CreateQuestion(ctx, actor.TenantID, sectionID, in)
	if err != nil {
		return nil, err
	}
	if err := recordAudit(ctx, s.auditRepo, actor, ownerCompanyIDValue(tmpl.OwnerCompanyID), "rfx_template_question", question.ID, "rfx.template.question.created.v1", map[string]any{"question_code": question.QuestionCode}); err != nil {
		return nil, err
	}
	return question, nil
}

func (s *TemplateQuestionnaireService) UpdateQuestion(ctx context.Context, actor domain.ActorContext, templateID, questionID uuid.UUID, in domain.UpdateQuestionInput) (*domain.TemplateQuestion, error) {
	if in.QuestionType != nil {
		if err := domain.ValidateQuestionType(*in.QuestionType); err != nil {
			return nil, err
		}
	}
	tmpl, draft, err := s.loadMutableDraft(ctx, actor, templateID)
	if err != nil {
		return nil, err
	}
	if err := s.qRepo.AssertQuestionBelongsToVersion(ctx, questionID, tmpl.ID, draft.ID, actor.TenantID); err != nil {
		return nil, err
	}
	question, err := s.qRepo.UpdateQuestion(ctx, questionID, actor.TenantID, in)
	if err != nil {
		return nil, err
	}
	if err := recordAudit(ctx, s.auditRepo, actor, ownerCompanyIDValue(tmpl.OwnerCompanyID), "rfx_template_question", question.ID, "rfx.template.question.updated.v1", nil); err != nil {
		return nil, err
	}
	return question, nil
}

func (s *TemplateQuestionnaireService) DeleteQuestion(ctx context.Context, actor domain.ActorContext, templateID, questionID uuid.UUID, expectedVersion int) error {
	tmpl, draft, err := s.loadMutableDraft(ctx, actor, templateID)
	if err != nil {
		return err
	}
	if err := s.qRepo.AssertQuestionBelongsToVersion(ctx, questionID, tmpl.ID, draft.ID, actor.TenantID); err != nil {
		return err
	}
	if err := s.qRepo.DeleteQuestion(ctx, questionID, actor.TenantID, expectedVersion); err != nil {
		return err
	}
	return recordAudit(ctx, s.auditRepo, actor, ownerCompanyIDValue(tmpl.OwnerCompanyID), "rfx_template_question", questionID, "rfx.template.question.deleted.v1", nil)
}

func (s *TemplateQuestionnaireService) ReorderQuestions(ctx context.Context, actor domain.ActorContext, templateID, sectionID uuid.UUID, orderedIDs []uuid.UUID) error {
	tmpl, draft, err := s.loadMutableDraft(ctx, actor, templateID)
	if err != nil {
		return err
	}
	if err := s.qRepo.AssertSectionBelongsToVersion(ctx, sectionID, tmpl.ID, draft.ID, actor.TenantID); err != nil {
		return err
	}
	if len(orderedIDs) == 0 {
		return apperrors.Validation("ordered_ids is required", map[string]any{"field": "ordered_ids"})
	}
	if err := s.qRepo.ReorderQuestions(ctx, actor.TenantID, sectionID, orderedIDs); err != nil {
		return err
	}
	return recordAudit(ctx, s.auditRepo, actor, ownerCompanyIDValue(tmpl.OwnerCompanyID), "rfx_template_version", draft.ID, "rfx.template.questions.reordered.v1", map[string]any{"section_id": sectionID.String()})
}

func (s *TemplateQuestionnaireService) DuplicateQuestion(ctx context.Context, actor domain.ActorContext, templateID, questionID uuid.UUID) (*domain.TemplateQuestion, error) {
	tmpl, draft, err := s.loadMutableDraft(ctx, actor, templateID)
	if err != nil {
		return nil, err
	}
	if err := s.qRepo.AssertQuestionBelongsToVersion(ctx, questionID, tmpl.ID, draft.ID, actor.TenantID); err != nil {
		return nil, err
	}
	source, err := s.qRepo.GetQuestionByID(ctx, questionID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	newCode := source.QuestionCode + "_copy"
	question, err := s.qRepo.DuplicateQuestion(ctx, actor.TenantID, questionID, newCode)
	if err != nil {
		return nil, err
	}
	if err := recordAudit(ctx, s.auditRepo, actor, ownerCompanyIDValue(tmpl.OwnerCompanyID), "rfx_template_question", question.ID, "rfx.template.question.duplicated.v1", map[string]any{"source_question_id": questionID.String()}); err != nil {
		return nil, err
	}
	return question, nil
}

func (s *TemplateQuestionnaireService) CreateOption(ctx context.Context, actor domain.ActorContext, templateID, questionID uuid.UUID, in domain.CreateQuestionOptionInput) (*domain.TemplateQuestionOption, error) {
	if err := domain.ValidateCreateQuestionOptionInput(in); err != nil {
		return nil, err
	}
	tmpl, draft, err := s.loadMutableDraft(ctx, actor, templateID)
	if err != nil {
		return nil, err
	}
	if err := s.qRepo.AssertQuestionBelongsToVersion(ctx, questionID, tmpl.ID, draft.ID, actor.TenantID); err != nil {
		return nil, err
	}
	option, err := s.qRepo.CreateOption(ctx, actor.TenantID, questionID, in)
	if err != nil {
		return nil, err
	}
	if err := recordAudit(ctx, s.auditRepo, actor, ownerCompanyIDValue(tmpl.OwnerCompanyID), "rfx_template_question_option", option.ID, "rfx.template.option.created.v1", map[string]any{"option_code": option.OptionCode}); err != nil {
		return nil, err
	}
	return option, nil
}

func (s *TemplateQuestionnaireService) UpdateOption(ctx context.Context, actor domain.ActorContext, templateID, questionID, optionID uuid.UUID, in domain.UpdateQuestionOptionInput) (*domain.TemplateQuestionOption, error) {
	tmpl, draft, err := s.loadMutableDraft(ctx, actor, templateID)
	if err != nil {
		return nil, err
	}
	if err := s.qRepo.AssertQuestionBelongsToVersion(ctx, questionID, tmpl.ID, draft.ID, actor.TenantID); err != nil {
		return nil, err
	}
	opt, err := s.qRepo.GetOptionByID(ctx, optionID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	if opt.QuestionID != questionID {
		return nil, apperrors.NotFound("option not found")
	}
	option, err := s.qRepo.UpdateOption(ctx, optionID, actor.TenantID, in)
	if err != nil {
		return nil, err
	}
	if err := recordAudit(ctx, s.auditRepo, actor, ownerCompanyIDValue(tmpl.OwnerCompanyID), "rfx_template_question_option", option.ID, "rfx.template.option.updated.v1", nil); err != nil {
		return nil, err
	}
	return option, nil
}

func (s *TemplateQuestionnaireService) DeleteOption(ctx context.Context, actor domain.ActorContext, templateID, questionID, optionID uuid.UUID, expectedVersion int) error {
	tmpl, draft, err := s.loadMutableDraft(ctx, actor, templateID)
	if err != nil {
		return err
	}
	if err := s.qRepo.AssertQuestionBelongsToVersion(ctx, questionID, tmpl.ID, draft.ID, actor.TenantID); err != nil {
		return err
	}
	opt, err := s.qRepo.GetOptionByID(ctx, optionID, actor.TenantID)
	if err != nil {
		return err
	}
	if opt.QuestionID != questionID {
		return apperrors.NotFound("option not found")
	}
	if err := s.qRepo.DeleteOption(ctx, optionID, actor.TenantID, expectedVersion); err != nil {
		return err
	}
	return recordAudit(ctx, s.auditRepo, actor, ownerCompanyIDValue(tmpl.OwnerCompanyID), "rfx_template_question_option", optionID, "rfx.template.option.deleted.v1", nil)
}

func (s *TemplateQuestionnaireService) CreateRule(ctx context.Context, actor domain.ActorContext, templateID uuid.UUID, in domain.CreateQuestionRuleInput) (*domain.TemplateQuestionRule, error) {
	if err := domain.ValidateCreateQuestionRuleInput(in); err != nil {
		return nil, err
	}
	tmpl, draft, err := s.loadMutableDraft(ctx, actor, templateID)
	if err != nil {
		return nil, err
	}
	var targetQuestionID *uuid.UUID
	if in.TargetQuestionCode != nil && strings.TrimSpace(*in.TargetQuestionCode) != "" {
		targetQuestionID, err = s.qRepo.GetQuestionIDByCodeInVersion(ctx, tmpl.ID, draft.ID, actor.TenantID, *in.TargetQuestionCode)
		if err != nil {
			return nil, err
		}
	}
	rule, err := s.qRepo.CreateRule(ctx, actor.TenantID, tmpl.ID, draft.ID, targetQuestionID, in)
	if err != nil {
		return nil, err
	}
	if err := recordAudit(ctx, s.auditRepo, actor, ownerCompanyIDValue(tmpl.OwnerCompanyID), "rfx_template_question_rule", rule.ID, "rfx.template.rule.created.v1", map[string]any{"rule_code": rule.RuleCode}); err != nil {
		return nil, err
	}
	return rule, nil
}

func (s *TemplateQuestionnaireService) UpdateRule(ctx context.Context, actor domain.ActorContext, templateID, ruleID uuid.UUID, in domain.UpdateQuestionRuleInput) (*domain.TemplateQuestionRule, error) {
	tmpl, draft, err := s.loadMutableDraft(ctx, actor, templateID)
	if err != nil {
		return nil, err
	}
	rule, err := s.qRepo.GetRuleByID(ctx, ruleID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	if rule.TemplateID != tmpl.ID || rule.RfxTemplateVersionID != draft.ID {
		return nil, apperrors.NotFound("rule not found")
	}
	var targetQuestionID *uuid.UUID
	if in.TargetQuestionCode != nil && strings.TrimSpace(*in.TargetQuestionCode) != "" {
		targetQuestionID, err = s.qRepo.GetQuestionIDByCodeInVersion(ctx, tmpl.ID, draft.ID, actor.TenantID, *in.TargetQuestionCode)
		if err != nil {
			return nil, err
		}
	} else if in.TargetQuestionCode != nil {
		targetQuestionID = nil
	} else {
		targetQuestionID = rule.TargetQuestionID
	}
	updated, err := s.qRepo.UpdateRule(ctx, ruleID, actor.TenantID, targetQuestionID, in)
	if err != nil {
		return nil, err
	}
	if err := recordAudit(ctx, s.auditRepo, actor, ownerCompanyIDValue(tmpl.OwnerCompanyID), "rfx_template_question_rule", updated.ID, "rfx.template.rule.updated.v1", nil); err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *TemplateQuestionnaireService) DeleteRule(ctx context.Context, actor domain.ActorContext, templateID, ruleID uuid.UUID, expectedVersion int) error {
	tmpl, draft, err := s.loadMutableDraft(ctx, actor, templateID)
	if err != nil {
		return err
	}
	rule, err := s.qRepo.GetRuleByID(ctx, ruleID, actor.TenantID)
	if err != nil {
		return err
	}
	if rule.TemplateID != tmpl.ID || rule.RfxTemplateVersionID != draft.ID {
		return apperrors.NotFound("rule not found")
	}
	if err := s.qRepo.DeleteRule(ctx, ruleID, actor.TenantID, expectedVersion); err != nil {
		return err
	}
	return recordAudit(ctx, s.auditRepo, actor, ownerCompanyIDValue(tmpl.OwnerCompanyID), "rfx_template_question_rule", ruleID, "rfx.template.rule.deleted.v1", nil)
}

func (s *TemplateQuestionnaireService) loadDraftContext(ctx context.Context, actor domain.ActorContext, templateID uuid.UUID) (*domain.RfxTemplate, *domain.RfxTemplateVersion, error) {
	tmpl, err := s.tmplSvc.authorizeTemplateRead(ctx, actor, templateID)
	if err != nil {
		return nil, nil, err
	}
	draft, err := s.tmplRepo.GetDraftVersion(ctx, templateID, actor.TenantID)
	if err != nil {
		return nil, nil, err
	}
	if draft == nil {
		return nil, nil, apperrors.NotFound("draft template version not found")
	}
	return tmpl, draft, nil
}

func (s *TemplateQuestionnaireService) loadMutableDraft(ctx context.Context, actor domain.ActorContext, templateID uuid.UUID) (*domain.RfxTemplate, *domain.RfxTemplateVersion, error) {
	tmpl, err := s.tmplSvc.authorizeTemplateManage(ctx, actor, templateID)
	if err != nil {
		return nil, nil, err
	}
	if err := domain.EnsureTemplateActive(tmpl.Status); err != nil {
		return nil, nil, err
	}
	draft, err := s.tmplRepo.GetDraftVersion(ctx, templateID, actor.TenantID)
	if err != nil {
		return nil, nil, err
	}
	if draft == nil {
		return nil, nil, apperrors.Conflict("draft template version not found", map[string]any{"field": "draft_version_id"})
	}
	if err := domain.EnsureTemplateVersionDraft(draft.Status); err != nil {
		return nil, nil, err
	}
	return tmpl, draft, nil
}
