package service

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/repository"
)

type ScoreModelStore interface {
	GetOrCreateDraftModel(ctx context.Context, tenantID, versionID uuid.UUID, createdBy *uuid.UUID) (*domain.ScoreModel, error)
	GetModelByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.ScoreModel, error)
	GetPublishedModelForVersion(ctx context.Context, tenantID, versionID uuid.UUID) (*domain.ScoreModel, error)
	GetDraftModelForVersion(ctx context.Context, tenantID, versionID uuid.UUID) (*domain.ScoreModel, error)
	ReplaceDraftDefinition(ctx context.Context, modelID, tenantID uuid.UUID, criteria []domain.ScoreCriterion, bindings []domain.ScoreBinding) error
	ListCriteriaByModel(ctx context.Context, modelID, tenantID uuid.UUID) ([]domain.ScoreCriterion, error)
	ListBindingsByModel(ctx context.Context, modelID, tenantID uuid.UUID) ([]domain.ScoreBinding, error)
	PublishModel(ctx context.Context, modelID, tenantID uuid.UUID) (*domain.ScoreModel, error)
	AssertVersionBelongsToTenant(ctx context.Context, versionID, tenantID uuid.UUID) error
}

type scoreQuestionnaireStore interface {
	CarrierQuestionnaireStore
	GetOrCreateDraftVersion(ctx context.Context, tenantID, eventID uuid.UUID) (*domain.RfxVersion, error)
}

type ScoreModelService struct {
	rfxRepo RfxStore
	scores  ScoreModelStore
	qRepo   scoreQuestionnaireStore
	audit   AuditRecorder
	actors  ActorResolver
	auth    *RfxService
}

func NewScoreModelService(
	rfxRepo RfxStore,
	scores ScoreModelStore,
	qRepo scoreQuestionnaireStore,
	audit AuditRecorder,
	actors ActorResolver,
	auth *RfxService,
) *ScoreModelService {
	return &ScoreModelService{rfxRepo: rfxRepo, scores: scores, qRepo: qRepo, audit: audit, actors: actors, auth: auth}
}

func (s *ScoreModelService) GetScoreModel(ctx context.Context, actor domain.ActorContext, eventID uuid.UUID) (*domain.ScoreModelView, error) {
	event, version, err := s.loadScoreModelContext(ctx, actor, eventID)
	if err != nil {
		return nil, err
	}
	_ = event
	model, criteria, bindings, questions, err := s.loadModelView(ctx, actor.TenantID, version.ID)
	if err != nil {
		var appErr *apperrors.AppError
		if errorsAsNotFound(err, &appErr) {
			draft, createErr := s.scores.GetOrCreateDraftModel(ctx, actor.TenantID, version.ID, &actor.UserID)
			if createErr != nil {
				return nil, createErr
			}
			readiness := domain.ValidateScoreModelReadiness(*draft, nil, nil, questions)
			return &domain.ScoreModelView{Model: *draft, Readiness: readiness}, nil
		}
		return nil, err
	}
	readiness := domain.ValidateScoreModelReadiness(*model, criteria, bindings, questions)
	return &domain.ScoreModelView{Model: *model, Criteria: criteria, Bindings: bindings, Readiness: readiness}, nil
}

func (s *ScoreModelService) PutScoreModel(ctx context.Context, actor domain.ActorContext, eventID uuid.UUID, input domain.PutScoreModelInput) (*domain.ScoreModelView, error) {
	event, version, err := s.loadScoreModelContext(ctx, actor, eventID)
	if err != nil {
		return nil, err
	}
	_ = event
	questionnaire, err := s.qRepo.LoadQuestionnaire(ctx, version.ID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	questions := flattenQuestions(questionnaire)
	questionByCode := map[string]domain.Question{}
	for _, q := range questions {
		questionByCode[q.QuestionCode] = q
	}

	model, err := s.scores.GetOrCreateDraftModel(ctx, actor.TenantID, version.ID, &actor.UserID)
	if err != nil {
		return nil, err
	}
	if model.Status != domain.ScoreModelStatusDraft {
		return nil, apperrors.Conflict("published score model is immutable", map[string]any{"field": "status"})
	}

	criteria := make([]domain.ScoreCriterion, 0, len(input.Criteria))
	criterionIDs := map[string]uuid.UUID{}
	for i, in := range input.Criteria {
		id := uuid.New()
		criterionIDs[in.CriterionCode] = id
		criteria = append(criteria, domain.ScoreCriterion{
			ID: id, TenantID: actor.TenantID, ScoreModelID: model.ID,
			CriterionCode: in.CriterionCode, Name: in.Name, Weight: in.Weight,
			NormalizationJSON: in.NormalizationJSON, SortOrder: in.SortOrder,
		})
		if in.SortOrder == 0 {
			criteria[i].SortOrder = i + 1
		}
	}

	bindings := make([]domain.ScoreBinding, 0, len(input.Bindings))
	for _, in := range input.Bindings {
		criterionID, ok := criterionIDs[in.CriterionCode]
		if !ok {
			return nil, apperrors.Validation("binding references unknown criterion", map[string]any{"criterion_code": in.CriterionCode})
		}
		question, ok := questionByCode[in.QuestionCode]
		if !ok {
			return nil, apperrors.Validation("binding references unknown question", map[string]any{"question_code": in.QuestionCode})
		}
		bindings = append(bindings, domain.ScoreBinding{
			ID: uuid.New(), TenantID: actor.TenantID, ScoreModelID: model.ID,
			CriterionID: criterionID, QuestionID: question.ID, BindingType: "QUESTION",
			ScoringRuleJSON: in.ScoringRuleJSON, KnockoutRuleJSON: in.KnockoutRuleJSON,
		})
	}

	if err := s.scores.ReplaceDraftDefinition(ctx, model.ID, actor.TenantID, criteria, bindings); err != nil {
		return nil, err
	}
	if err := recordAudit(ctx, s.audit, actor, event.OwnerCompanyID, "rfx_score_model", model.ID, "score_model.updated", map[string]any{
		"rfx_event_id": eventID.String(), "rfx_version_id": version.ID.String(),
	}); err != nil {
		return nil, err
	}
	return s.GetScoreModel(ctx, actor, eventID)
}

func (s *ScoreModelService) ValidateScoreModel(ctx context.Context, actor domain.ActorContext, eventID uuid.UUID) (*domain.ScoreModelReadinessResult, error) {
	view, err := s.GetScoreModel(ctx, actor, eventID)
	if err != nil {
		return nil, err
	}
	return &view.Readiness, nil
}

func (s *ScoreModelService) PublishScoreModel(ctx context.Context, actor domain.ActorContext, eventID uuid.UUID) (*domain.ScoreModelView, error) {
	event, version, err := s.loadScoreModelContext(ctx, actor, eventID)
	if err != nil {
		return nil, err
	}
	model, criteria, bindings, questions, err := s.loadModelView(ctx, actor.TenantID, version.ID)
	if err != nil {
		return nil, err
	}
	if model.Status != domain.ScoreModelStatusDraft {
		return nil, apperrors.Conflict("published score model is immutable", map[string]any{"field": "status"})
	}
	readiness := domain.ValidateScoreModelReadiness(*model, criteria, bindings, questions)
	if !readiness.Ready {
		return nil, apperrors.Validation("score model readiness failed", map[string]any{"errors": readiness.Errors})
	}
	published, err := s.scores.PublishModel(ctx, model.ID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	if err := recordAudit(ctx, s.audit, actor, event.OwnerCompanyID, "rfx_score_model", published.ID, "score_model.published", map[string]any{
		"rfx_event_id": eventID.String(), "model_version": published.ModelVersion,
	}); err != nil {
		return nil, err
	}
	readiness = domain.ValidateScoreModelReadiness(*published, criteria, bindings, questions)
	return &domain.ScoreModelView{Model: *published, Criteria: criteria, Bindings: bindings, Readiness: readiness}, nil
}

func (s *ScoreModelService) loadScoreModelContext(ctx context.Context, actor domain.ActorContext, eventID uuid.UUID) (*domain.RfxEvent, *domain.RfxVersion, error) {
	if err := actor.Validate(); err != nil {
		return nil, nil, err
	}
	event, err := s.rfxRepo.GetEventByID(ctx, eventID, actor.TenantID)
	if err != nil {
		return nil, nil, err
	}
	if err := s.auth.requireOwnerCompanyAccessForEvent(ctx, actor, event); err != nil {
		return nil, nil, err
	}
	version, err := s.qRepo.GetPublishedVersionForEvent(ctx, eventID, actor.TenantID)
	if err != nil {
		var appErr *apperrors.AppError
		if errorsAsNotFound(err, &appErr) {
			draft, draftErr := s.qRepo.GetOrCreateDraftVersion(ctx, actor.TenantID, eventID)
			if draftErr != nil {
				return nil, nil, draftErr
			}
			return event, draft, nil
		}
		return nil, nil, err
	}
	return event, version, nil
}

func (s *ScoreModelService) loadModelView(ctx context.Context, tenantID, versionID uuid.UUID) (*domain.ScoreModel, []domain.ScoreCriterion, []domain.ScoreBinding, []domain.Question, error) {
	questionnaire, err := s.qRepo.LoadQuestionnaire(ctx, versionID, tenantID)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	questions := flattenQuestions(questionnaire)

	model, err := s.scores.GetDraftModelForVersion(ctx, tenantID, versionID)
	if err != nil {
		published, pubErr := s.scores.GetPublishedModelForVersion(ctx, tenantID, versionID)
		if pubErr != nil {
			return nil, nil, nil, questions, err
		}
		model = published
	}
	criteria, err := s.scores.ListCriteriaByModel(ctx, model.ID, tenantID)
	if err != nil {
		return nil, nil, nil, questions, err
	}
	bindings, err := s.scores.ListBindingsByModel(ctx, model.ID, tenantID)
	if err != nil {
		return nil, nil, nil, questions, err
	}
	return model, criteria, bindings, questions, nil
}

func flattenQuestions(q *domain.QuestionnaireDefinition) []domain.Question {
	out := make([]domain.Question, 0)
	for _, sec := range q.Sections {
		out = append(out, sec.Questions...)
	}
	return out
}

func errorsAsNotFound(err error, target **apperrors.AppError) bool {
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) && appErr.Code == apperrors.CodeNotFound {
		*target = appErr
		return true
	}
	return false
}

var _ ScoreModelStore = (*repository.ScoreRepository)(nil)
