package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/platform/respond"
	"github.com/freight-platform/rfx-service/internal/service"
)

type ScoreHandler struct {
	models  *service.ScoreModelService
	scoring *service.ScoringService
	rfx     *service.RfxService
}

func NewScoreHandler(models *service.ScoreModelService, scoring *service.ScoringService, rfx *service.RfxService) *ScoreHandler {
	return &ScoreHandler{models: models, scoring: scoring, rfx: rfx}
}

func (h *ScoreHandler) GetScoreModel(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	view, err := h.models.GetScoreModel(r.Context(), actor, eventID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toScoreModelViewResponse(view))
}

func (h *ScoreHandler) PutScoreModel(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	var input domain.PutScoreModelInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	view, err := h.models.PutScoreModel(r.Context(), actor, eventID, input)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toScoreModelViewResponse(view))
}

func (h *ScoreHandler) ValidateScoreModel(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	result, err := h.models.ValidateScoreModel(r.Context(), actor, eventID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, result)
}

func (h *ScoreHandler) PublishScoreModel(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	view, err := h.models.PublishScoreModel(r.Context(), actor, eventID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toScoreModelViewResponse(view))
}

func (h *ScoreHandler) GetResponseScore(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	responseID, err := uuid.Parse(chi.URLParam(r, "response_id"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid response_id", nil))
		return
	}
	view, err := h.scoring.GetResponseScore(r.Context(), actor, eventID, responseID, h.rfx)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toResponseScoreViewResponse(view))
}

func (h *ScoreHandler) GetResponseScoreExplanation(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	responseID, err := uuid.Parse(chi.URLParam(r, "response_id"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid response_id", nil))
		return
	}
	explanations, err := h.scoring.GetResponseScoreExplanation(r.Context(), actor, eventID, responseID, h.rfx)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{"explanations": explanations})
}

func toScoreModelViewResponse(view *domain.ScoreModelView) map[string]any {
	criteria := make([]map[string]any, 0, len(view.Criteria))
	for _, c := range view.Criteria {
		criteria = append(criteria, map[string]any{
			"id": c.ID.String(), "criterion_code": c.CriterionCode, "name": c.Name,
			"weight": c.Weight, "normalization_json": json.RawMessage(c.NormalizationJSON), "sort_order": c.SortOrder,
		})
	}
	bindings := make([]map[string]any, 0, len(view.Bindings))
	for _, b := range view.Bindings {
		bindings = append(bindings, map[string]any{
			"id": b.ID.String(), "criterion_id": b.CriterionID.String(), "question_id": b.QuestionID.String(),
			"binding_type": b.BindingType, "scoring_rule_json": json.RawMessage(b.ScoringRuleJSON),
			"knockout_rule_json": json.RawMessage(b.KnockoutRuleJSON),
		})
	}
	return map[string]any{
		"model": map[string]any{
			"id": view.Model.ID.String(), "rfx_version_id": view.Model.RfxVersionID.String(),
			"model_version": view.Model.ModelVersion, "status": view.Model.Status, "model_type": view.Model.ModelType,
			"published_at": view.Model.PublishedAt,
		},
		"criteria":  criteria,
		"bindings":  bindings,
		"readiness": view.Readiness,
	}
}

func toResponseScoreViewResponse(view *domain.ResponseScoreView) map[string]any {
	scores := make([]map[string]any, 0, len(view.AnswerScores))
	for _, s := range view.AnswerScores {
		scores = append(scores, map[string]any{
			"id": s.ID.String(), "answer_id": s.AnswerID.String(), "criterion_id": s.CriterionID.String(),
			"score_model_version": s.ScoreModelVersion, "raw_score": s.RawScore,
			"normalized_score": s.NormalizedScore, "weighted_contribution": s.WeightedContribution,
			"explanation_json": json.RawMessage(s.ExplanationJSON),
		})
	}
	return map[string]any{
		"qualification": map[string]any{
			"status": view.Qualification.Status, "calculation_status": view.Qualification.CalculationStatus,
			"total_score": view.Qualification.TotalScore, "knockout_triggered": view.Qualification.KnockoutTriggered,
			"score_model_version": view.Qualification.ScoreModelVersion,
			"knockout_reason_json": json.RawMessage(view.Qualification.KnockoutReasonJSON),
		},
		"answer_scores": scores,
	}
}
