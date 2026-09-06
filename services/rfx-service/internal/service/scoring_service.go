package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/repository"
)

type ScoringResultStore interface {
	GetPublishedModelForVersion(ctx context.Context, tenantID, versionID uuid.UUID) (*domain.ScoreModel, error)
	ListCriteriaByModel(ctx context.Context, modelID, tenantID uuid.UUID) ([]domain.ScoreCriterion, error)
	ListBindingsByModel(ctx context.Context, modelID, tenantID uuid.UUID) ([]domain.ScoreBinding, error)
	ReplaceScoringResults(ctx context.Context, tenantID uuid.UUID, responseID uuid.UUID, model domain.ScoreModel, qualification domain.QualificationResult, answerScores []domain.AnswerScore) error
	MarkScoringFailed(ctx context.Context, tenantID, responseID uuid.UUID, model domain.ScoreModel) error
	GetLatestQualificationForResponse(ctx context.Context, responseID, tenantID uuid.UUID) (*domain.QualificationResult, error)
	GetQualificationResult(ctx context.Context, responseID, tenantID uuid.UUID, modelVersion int) (*domain.QualificationResult, error)
	ListAnswerScores(ctx context.Context, responseID, tenantID uuid.UUID, modelVersion int) ([]domain.AnswerScore, error)
}

type ScoringService struct {
	rfx     CarrierResponseStore
	answers CarrierAnswerStore
	q       CarrierQuestionnaireStore
	scores  ScoringResultStore
	audit   AuditRecorder
	tx      *repository.TransactionRunner
	log     *slog.Logger
}

func NewScoringService(
	pool *pgxpool.Pool,
	rfx CarrierResponseStore,
	answers CarrierAnswerStore,
	q CarrierQuestionnaireStore,
	scores ScoringResultStore,
	audit AuditRecorder,
) *ScoringService {
	var tx *repository.TransactionRunner
	if pool != nil {
		tx = repository.NewTransactionRunner(pool)
	}
	return &ScoringService{rfx: rfx, answers: answers, q: q, scores: scores, audit: audit, tx: tx, log: slog.Default()}
}

func (s *ScoringService) CalculateForSubmittedResponse(ctx context.Context, tenantID, responseID uuid.UUID) (*domain.ScoringRunResult, error) {
	response, err := s.rfx.GetResponseByID(ctx, responseID, tenantID)
	if err != nil {
		return nil, err
	}
	if response.RfxVersionID == nil {
		return &domain.ScoringRunResult{Skipped: true}, nil
	}
	if response.Status != domain.RfxResponseStatusSubmitted {
		return nil, apperrors.Validation("response must be submitted before scoring", map[string]any{"field": "status"})
	}

	model, err := s.scores.GetPublishedModelForVersion(ctx, tenantID, *response.RfxVersionID)
	if err != nil {
		var appErr *apperrors.AppError
		if errors.As(err, &appErr) && appErr.Code == apperrors.CodeNotFound {
			return &domain.ScoringRunResult{Skipped: true}, nil
		}
		return nil, err
	}

	result, calcErr := s.calculate(ctx, tenantID, response, *model)
	if calcErr != nil {
		var appErr *apperrors.AppError
		if !errors.As(calcErr, &appErr) || appErr.Code != apperrors.CodeValidation {
			_ = s.scores.MarkScoringFailed(ctx, tenantID, responseID, *model)
		}
		s.log.Error("scoring failed", slog.String("response_id", responseID.String()), slog.String("error", calcErr.Error()))
		return nil, calcErr
	}
	return result, nil
}

func (s *ScoringService) calculate(ctx context.Context, tenantID uuid.UUID, response *domain.RfxResponse, model domain.ScoreModel) (*domain.ScoringRunResult, error) {
	questionnaire, err := s.q.LoadQuestionnaire(ctx, model.RfxVersionID, tenantID)
	if err != nil {
		return nil, err
	}
	questions := flattenQuestions(questionnaire)
	questionByID := map[uuid.UUID]domain.Question{}
	for _, q := range questions {
		questionByID[q.ID] = q
	}

	criteria, err := s.scores.ListCriteriaByModel(ctx, model.ID, tenantID)
	if err != nil {
		return nil, err
	}
	criterionByID := map[uuid.UUID]domain.ScoreCriterion{}
	for _, c := range criteria {
		criterionByID[c.ID] = c
	}

	bindings, err := s.scores.ListBindingsByModel(ctx, model.ID, tenantID)
	if err != nil {
		return nil, err
	}

	answers, err := s.answers.ListByResponse(ctx, response.ID, tenantID)
	if err != nil {
		return nil, err
	}
	answerByQuestion := map[uuid.UUID]domain.CarrierAnswer{}
	for _, a := range answers {
		answerByQuestion[a.QuestionID] = a
	}

	now := time.Now().UTC()
	computations := make([]domain.AnswerScoreComputation, 0, len(bindings))
	answerScores := make([]domain.AnswerScore, 0, len(bindings))
	knockoutReasons := make([]string, 0)
	anyKnockout := false

	for _, binding := range bindings {
		criterion, ok := criterionByID[binding.CriterionID]
		if !ok {
			return nil, apperrors.Internal("criterion missing for binding", nil)
		}
		question, ok := questionByID[binding.QuestionID]
		if !ok {
			return nil, apperrors.Internal("question missing for binding", nil)
		}
		answer, ok := answerByQuestion[binding.QuestionID]
		if !ok {
			return nil, apperrors.Validation("persisted answer required for scoring", map[string]any{"question_code": question.QuestionCode})
		}

		computed, err := domain.ComputeAnswerScore(question, answer.AnswerValueJSON, criterion, binding, model)
		if err != nil {
			return nil, apperrors.Validation("answer not scoreable", map[string]any{"question_code": question.QuestionCode, "reason": err.Error()})
		}
		computations = append(computations, computed)
		if computed.Knockout {
			anyKnockout = true
			if computed.KnockoutReason != "" {
				knockoutReasons = append(knockoutReasons, computed.KnockoutReason)
			}
		}

		explanationJSON, _ := json.Marshal(computed.Explanation)
		answerScores = append(answerScores, domain.AnswerScore{
			ID: uuid.New(), TenantID: tenantID, RfxResponseID: response.ID, AnswerID: answer.ID,
			CriterionID: criterion.ID, ScoreModelID: model.ID, ScoreModelVersion: model.ModelVersion,
			RawScore: computed.RawScore, NormalizedScore: computed.NormalizedScore,
			WeightedContribution: computed.WeightedContribution, ExplanationJSON: explanationJSON,
			CalculatedAt: now,
		})
	}

	total := domain.SumWeightedContributions(computations)
	status, knockoutJSON := domain.AggregateQualificationStatus(total, anyKnockout, knockoutReasons)
	qualification := domain.QualificationResult{
		ID: uuid.New(), TenantID: tenantID, RfxResponseID: response.ID,
		ScoreModelID: model.ID, ScoreModelVersion: model.ModelVersion,
		Status: status, CalculationStatus: domain.ScoringCalculationCalculated,
		TotalScore: &total, KnockoutTriggered: anyKnockout, KnockoutReasonJSON: knockoutJSON,
		CalculatedAt: &now,
	}

	persist := func(repo *repository.ScoreRepository) error {
		return repo.ReplaceScoringResults(ctx, tenantID, response.ID, model, qualification, answerScores)
	}
	if s.tx != nil {
		scoreRepo, ok := s.scores.(*repository.ScoreRepository)
		if !ok {
			return nil, apperrors.Internal("scoring store misconfigured", nil)
		}
		err = s.tx.Run(ctx, func(ctx context.Context, tx pgx.Tx) error {
			return persist(scoreRepo.WithTx(tx))
		})
	} else if scoreRepo, ok := s.scores.(*repository.ScoreRepository); ok {
		err = persist(scoreRepo)
	} else {
		err = s.scores.ReplaceScoringResults(ctx, tenantID, response.ID, model, qualification, answerScores)
	}
	if err != nil {
		return nil, err
	}

	actor := domain.ActorContext{TenantID: tenantID}
	_ = recordAudit(ctx, s.audit, actor, response.ParticipantCompanyID, "rfx_response", response.ID, "rfx.score.calculated.v1", map[string]any{
		"rfx_event_id": response.RfxEventID.String(), "rfx_version_id": model.RfxVersionID.String(),
		"score_model_id": model.ID.String(), "score_model_version": model.ModelVersion,
		"total_score": total, "qualification_status": status, "knockout": anyKnockout,
	})
	if anyKnockout {
		_ = recordAudit(ctx, s.audit, actor, response.ParticipantCompanyID, "rfx_response", response.ID, "rfx.knockout.triggered.v1", map[string]any{
			"score_model_version": model.ModelVersion, "reasons": knockoutReasons,
		})
	}

	return &domain.ScoringRunResult{Qualification: qualification, AnswerScores: answerScores}, nil
}

func (s *ScoringService) GetResponseScore(ctx context.Context, actor domain.ActorContext, eventID, responseID uuid.UUID, auth *RfxService) (*domain.ResponseScoreView, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	if _, _, err := auth.requireBuyerEventAccess(ctx, actor, eventID); err != nil {
		return nil, err
	}
	response, err := s.rfx.GetResponseByID(ctx, responseID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	if response.RfxEventID != eventID {
		return nil, apperrors.NotFound("rfx response not found")
	}
	qualification, err := s.scores.GetLatestQualificationForResponse(ctx, responseID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	scores, err := s.scores.ListAnswerScores(ctx, responseID, actor.TenantID, qualification.ScoreModelVersion)
	if err != nil {
		return nil, err
	}
	return &domain.ResponseScoreView{Qualification: *qualification, AnswerScores: scores}, nil
}

func (s *ScoringService) GetResponseScoreExplanation(ctx context.Context, actor domain.ActorContext, eventID, responseID uuid.UUID, auth *RfxService) ([]domain.ScoreExplanation, error) {
	view, err := s.GetResponseScore(ctx, actor, eventID, responseID, auth)
	if err != nil {
		return nil, err
	}
	out := make([]domain.ScoreExplanation, 0, len(view.AnswerScores))
	for _, score := range view.AnswerScores {
		var explanation domain.ScoreExplanation
		if err := json.Unmarshal(score.ExplanationJSON, &explanation); err != nil {
			continue
		}
		out = append(out, explanation)
	}
	return out, nil
}
