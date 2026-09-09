package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/repository"
)

type TemplateQuestionnaireService struct {
	tmplRepo  *repository.TemplateLibraryRepository
	qRepo     *repository.TemplateQuestionnaireRepository
	auditRepo *repository.AuditRepository
	tmplSvc   *TemplateLibraryService
	tx        *repository.TransactionRunner
}

func NewTemplateQuestionnaireService(
	pool *pgxpool.Pool,
	tmplRepo *repository.TemplateLibraryRepository,
	qRepo *repository.TemplateQuestionnaireRepository,
	auditRepo *repository.AuditRepository,
	tmplSvc *TemplateLibraryService,
) *TemplateQuestionnaireService {
	var tx *repository.TransactionRunner
	if pool != nil {
		tx = repository.NewTransactionRunner(pool)
	}
	return &TemplateQuestionnaireService{tmplRepo: tmplRepo, qRepo: qRepo, auditRepo: auditRepo, tmplSvc: tmplSvc, tx: tx}
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
	var section *domain.TemplateSection
	err = s.runMutation(ctx, func(qRepo *repository.TemplateQuestionnaireRepository, aRepo *repository.AuditRepository) error {
		var createErr error
		section, createErr = qRepo.CreateSection(ctx, actor.TenantID, tmpl.ID, draft.ID, in)
		if createErr != nil {
			return createErr
		}
		return recordAudit(ctx, aRepo, actor, ownerCompanyIDValue(tmpl.OwnerCompanyID), "rfx_template_section", section.ID, "rfx.template.section.created.v1", map[string]any{"section_code": section.SectionCode})
	})
	if err != nil {
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
	var section *domain.TemplateSection
	err = s.runMutation(ctx, func(qRepo *repository.TemplateQuestionnaireRepository, aRepo *repository.AuditRepository) error {
		var updateErr error
		section, updateErr = qRepo.UpdateSection(ctx, sectionID, actor.TenantID, in)
		if updateErr != nil {
			return updateErr
		}
		return recordAudit(ctx, aRepo, actor, ownerCompanyIDValue(tmpl.OwnerCompanyID), "rfx_template_section", section.ID, "rfx.template.section.updated.v1", nil)
	})
	if err != nil {
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
	return s.runMutation(ctx, func(qRepo *repository.TemplateQuestionnaireRepository, aRepo *repository.AuditRepository) error {
		if err := qRepo.DeleteSection(ctx, sectionID, actor.TenantID, expectedVersion); err != nil {
			return err
		}
		return recordAudit(ctx, aRepo, actor, ownerCompanyIDValue(tmpl.OwnerCompanyID), "rfx_template_section", sectionID, "rfx.template.section.deleted.v1", nil)
	})
}

func (s *TemplateQuestionnaireService) ReorderSections(ctx context.Context, actor domain.ActorContext, templateID uuid.UUID, orderedIDs []uuid.UUID) error {
	tmpl, draft, err := s.loadMutableDraft(ctx, actor, templateID)
	if err != nil {
		return err
	}
	if len(orderedIDs) == 0 {
		return apperrors.Validation("ordered_ids is required", map[string]any{"field": "ordered_ids"})
	}
	return s.runMutation(ctx, func(qRepo *repository.TemplateQuestionnaireRepository, aRepo *repository.AuditRepository) error {
		if err := qRepo.ReorderSections(ctx, actor.TenantID, tmpl.ID, draft.ID, orderedIDs); err != nil {
			return err
		}
		return recordAudit(ctx, aRepo, actor, ownerCompanyIDValue(tmpl.OwnerCompanyID), "rfx_template_version", draft.ID, "rfx.template.sections.reordered.v1", nil)
	})
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
	var question *domain.TemplateQuestion
	err = s.runMutation(ctx, func(qRepo *repository.TemplateQuestionnaireRepository, aRepo *repository.AuditRepository) error {
		var createErr error
		question, createErr = qRepo.CreateQuestion(ctx, actor.TenantID, sectionID, in)
		if createErr != nil {
			return createErr
		}
		return recordAudit(ctx, aRepo, actor, ownerCompanyIDValue(tmpl.OwnerCompanyID), "rfx_template_question", question.ID, "rfx.template.question.created.v1", map[string]any{"question_code": question.QuestionCode})
	})
	if err != nil {
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
	var question *domain.TemplateQuestion
	err = s.runMutation(ctx, func(qRepo *repository.TemplateQuestionnaireRepository, aRepo *repository.AuditRepository) error {
		var updateErr error
		question, updateErr = qRepo.UpdateQuestion(ctx, questionID, actor.TenantID, in)
		if updateErr != nil {
			return updateErr
		}
		return recordAudit(ctx, aRepo, actor, ownerCompanyIDValue(tmpl.OwnerCompanyID), "rfx_template_question", question.ID, "rfx.template.question.updated.v1", nil)
	})
	if err != nil {
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
	return s.runMutation(ctx, func(qRepo *repository.TemplateQuestionnaireRepository, aRepo *repository.AuditRepository) error {
		if err := qRepo.DeleteQuestion(ctx, questionID, actor.TenantID, expectedVersion); err != nil {
			return err
		}
		return recordAudit(ctx, aRepo, actor, ownerCompanyIDValue(tmpl.OwnerCompanyID), "rfx_template_question", questionID, "rfx.template.question.deleted.v1", nil)
	})
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
	return s.runMutation(ctx, func(qRepo *repository.TemplateQuestionnaireRepository, aRepo *repository.AuditRepository) error {
		if err := qRepo.ReorderQuestions(ctx, actor.TenantID, sectionID, orderedIDs); err != nil {
			return err
		}
		return recordAudit(ctx, aRepo, actor, ownerCompanyIDValue(tmpl.OwnerCompanyID), "rfx_template_version", draft.ID, "rfx.template.questions.reordered.v1", map[string]any{"section_id": sectionID.String()})
	})
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
	newCode, err := s.nextDuplicateQuestionCode(ctx, tmpl.ID, draft.ID, actor.TenantID, source.QuestionCode)
	if err != nil {
		return nil, err
	}
	var question *domain.TemplateQuestion
	err = s.runMutation(ctx, func(qRepo *repository.TemplateQuestionnaireRepository, aRepo *repository.AuditRepository) error {
		var dupErr error
		question, dupErr = qRepo.DuplicateQuestion(ctx, actor.TenantID, questionID, newCode)
		if dupErr != nil {
			return dupErr
		}
		return recordAudit(ctx, aRepo, actor, ownerCompanyIDValue(tmpl.OwnerCompanyID), "rfx_template_question", question.ID, "rfx.template.question.duplicated.v1", map[string]any{"source_question_id": questionID.String()})
	})
	if err != nil {
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
	var option *domain.TemplateQuestionOption
	err = s.runMutation(ctx, func(qRepo *repository.TemplateQuestionnaireRepository, aRepo *repository.AuditRepository) error {
		var createErr error
		option, createErr = qRepo.CreateOption(ctx, actor.TenantID, questionID, in)
		if createErr != nil {
			return createErr
		}
		return recordAudit(ctx, aRepo, actor, ownerCompanyIDValue(tmpl.OwnerCompanyID), "rfx_template_question_option", option.ID, "rfx.template.option.created.v1", map[string]any{"option_code": option.OptionCode})
	})
	if err != nil {
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
	var option *domain.TemplateQuestionOption
	err = s.runMutation(ctx, func(qRepo *repository.TemplateQuestionnaireRepository, aRepo *repository.AuditRepository) error {
		var updateErr error
		option, updateErr = qRepo.UpdateOption(ctx, optionID, actor.TenantID, in)
		if updateErr != nil {
			return updateErr
		}
		return recordAudit(ctx, aRepo, actor, ownerCompanyIDValue(tmpl.OwnerCompanyID), "rfx_template_question_option", option.ID, "rfx.template.option.updated.v1", nil)
	})
	if err != nil {
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
	return s.runMutation(ctx, func(qRepo *repository.TemplateQuestionnaireRepository, aRepo *repository.AuditRepository) error {
		if err := qRepo.DeleteOption(ctx, optionID, actor.TenantID, expectedVersion); err != nil {
			return err
		}
		return recordAudit(ctx, aRepo, actor, ownerCompanyIDValue(tmpl.OwnerCompanyID), "rfx_template_question_option", optionID, "rfx.template.option.deleted.v1", nil)
	})
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
	var rule *domain.TemplateQuestionRule
	err = s.runMutation(ctx, func(qRepo *repository.TemplateQuestionnaireRepository, aRepo *repository.AuditRepository) error {
		var createErr error
		rule, createErr = qRepo.CreateRule(ctx, actor.TenantID, tmpl.ID, draft.ID, targetQuestionID, in)
		if createErr != nil {
			return createErr
		}
		return recordAudit(ctx, aRepo, actor, ownerCompanyIDValue(tmpl.OwnerCompanyID), "rfx_template_question_rule", rule.ID, "rfx.template.rule.created.v1", map[string]any{"rule_code": rule.RuleCode})
	})
	if err != nil {
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
	var updated *domain.TemplateQuestionRule
	err = s.runMutation(ctx, func(qRepo *repository.TemplateQuestionnaireRepository, aRepo *repository.AuditRepository) error {
		var updateErr error
		updated, updateErr = qRepo.UpdateRule(ctx, ruleID, actor.TenantID, targetQuestionID, in)
		if updateErr != nil {
			return updateErr
		}
		return recordAudit(ctx, aRepo, actor, ownerCompanyIDValue(tmpl.OwnerCompanyID), "rfx_template_question_rule", updated.ID, "rfx.template.rule.updated.v1", nil)
	})
	if err != nil {
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
	return s.runMutation(ctx, func(qRepo *repository.TemplateQuestionnaireRepository, aRepo *repository.AuditRepository) error {
		if err := qRepo.DeleteRule(ctx, ruleID, actor.TenantID, expectedVersion); err != nil {
			return err
		}
		return recordAudit(ctx, aRepo, actor, ownerCompanyIDValue(tmpl.OwnerCompanyID), "rfx_template_question_rule", ruleID, "rfx.template.rule.deleted.v1", nil)
	})
}

func (s *TemplateQuestionnaireService) runMutation(ctx context.Context, fn func(qRepo *repository.TemplateQuestionnaireRepository, aRepo *repository.AuditRepository) error) error {
	if s.tx == nil {
		return apperrors.Internal("template questionnaire service misconfigured", nil)
	}
	return s.tx.Run(ctx, func(ctx context.Context, tx pgx.Tx) error {
		return fn(s.qRepo.WithTx(tx), s.auditRepo.WithTx(tx))
	})
}

func (s *TemplateQuestionnaireService) nextDuplicateQuestionCode(ctx context.Context, templateID, versionID, tenantID uuid.UUID, baseCode string) (string, error) {
	definition, err := s.qRepo.LoadQuestionnaire(ctx, templateID, versionID, tenantID)
	if err != nil {
		return "", err
	}
	existing := make(map[string]struct{})
	for _, swq := range definition.Sections {
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
