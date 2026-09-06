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

type QuestionnaireHandler struct {
	service *service.QuestionnaireService
}

func NewQuestionnaireHandler(svc *service.QuestionnaireService) *QuestionnaireHandler {
	return &QuestionnaireHandler{service: svc}
}

func (h *QuestionnaireHandler) GetStudio(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	view, err := h.service.GetStudio(r.Context(), actor, eventID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toQuestionnaireStudioResponse(view))
}

func (h *QuestionnaireHandler) GetQuestionnaire(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	q, err := h.service.GetQuestionnaire(r.Context(), actor, eventID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toQuestionnaireDefinitionResponse(q))
}

func (h *QuestionnaireHandler) SaveDraft(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	var req struct {
		ExpectedVersion int `json:"expected_version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	ver, err := h.service.SaveDraft(r.Context(), actor, eventID, req.ExpectedVersion)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toRfxVersionResponse(ver))
}

func (h *QuestionnaireHandler) ValidatePublish(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	result, err := h.service.ValidatePublish(r.Context(), actor, eventID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toPublishReadinessResponse(result))
}

func (h *QuestionnaireHandler) CreateSection(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	var in domain.CreateSectionInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	sec, err := h.service.CreateSection(r.Context(), actor, eventID, in)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toSectionResponse(sec))
}

func (h *QuestionnaireHandler) UpdateSection(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	sectionID, err := uuid.Parse(chi.URLParam(r, "section_id"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid section_id", nil))
		return
	}
	var in domain.UpdateSectionInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	sec, err := h.service.UpdateSection(r.Context(), actor, eventID, sectionID, in)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toSectionResponse(sec))
}

func (h *QuestionnaireHandler) DeleteSection(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	sectionID, err := uuid.Parse(chi.URLParam(r, "section_id"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid section_id", nil))
		return
	}
	var req struct {
		ExpectedVersion int `json:"expected_version"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if err := h.service.DeleteSection(r.Context(), actor, eventID, sectionID, req.ExpectedVersion); err != nil {
		respond.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *QuestionnaireHandler) ReorderSections(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	var req domain.ReorderInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	if err := h.service.ReorderSections(r.Context(), actor, eventID, req.OrderedIDs); err != nil {
		respond.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *QuestionnaireHandler) CreateQuestion(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	var req struct {
		SectionID string `json:"section_id"`
		domain.CreateQuestionInput
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	sectionID, err := uuid.Parse(req.SectionID)
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid section_id", nil))
		return
	}
	q, err := h.service.CreateQuestion(r.Context(), actor, eventID, sectionID, req.CreateQuestionInput)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toQuestionResponse(q))
}

func (h *QuestionnaireHandler) UpdateQuestion(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	questionID, err := uuid.Parse(chi.URLParam(r, "question_id"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid question_id", nil))
		return
	}
	var in domain.UpdateQuestionInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	q, err := h.service.UpdateQuestion(r.Context(), actor, eventID, questionID, in)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toQuestionResponse(q))
}

func (h *QuestionnaireHandler) DeleteQuestion(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	questionID, err := uuid.Parse(chi.URLParam(r, "question_id"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid question_id", nil))
		return
	}
	var req struct {
		ExpectedVersion int `json:"expected_version"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if err := h.service.DeleteQuestion(r.Context(), actor, eventID, questionID, req.ExpectedVersion); err != nil {
		respond.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *QuestionnaireHandler) DuplicateQuestion(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	questionID, err := uuid.Parse(chi.URLParam(r, "question_id"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid question_id", nil))
		return
	}
	q, err := h.service.DuplicateQuestion(r.Context(), actor, eventID, questionID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toQuestionResponse(q))
}

func (h *QuestionnaireHandler) ReorderQuestions(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	var req struct {
		SectionID  string      `json:"section_id"`
		OrderedIDs []uuid.UUID `json:"ordered_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	sectionID, err := uuid.Parse(req.SectionID)
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid section_id", nil))
		return
	}
	if err := h.service.ReorderQuestions(r.Context(), actor, eventID, sectionID, req.OrderedIDs); err != nil {
		respond.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *QuestionnaireHandler) CreateOption(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	questionID, err := uuid.Parse(chi.URLParam(r, "question_id"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid question_id", nil))
		return
	}
	var in domain.CreateQuestionOptionInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	opt, err := h.service.CreateOption(r.Context(), actor, eventID, questionID, in)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toQuestionOptionResponse(opt))
}

func (h *QuestionnaireHandler) UpdateOption(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	optionID, err := uuid.Parse(chi.URLParam(r, "option_id"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid option_id", nil))
		return
	}
	var in domain.UpdateQuestionOptionInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	opt, err := h.service.UpdateOption(r.Context(), actor, eventID, optionID, in)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toQuestionOptionResponse(opt))
}

func (h *QuestionnaireHandler) DeleteOption(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	optionID, err := uuid.Parse(chi.URLParam(r, "option_id"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid option_id", nil))
		return
	}
	var req struct {
		ExpectedVersion int `json:"expected_version"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if err := h.service.DeleteOption(r.Context(), actor, eventID, optionID, req.ExpectedVersion); err != nil {
		respond.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *QuestionnaireHandler) CreateRule(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	var in domain.CreateQuestionRuleInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	rule, err := h.service.CreateRule(r.Context(), actor, eventID, in)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toQuestionRuleResponse(rule))
}

func (h *QuestionnaireHandler) UpdateRule(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	ruleID, err := uuid.Parse(chi.URLParam(r, "rule_id"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid rule_id", nil))
		return
	}
	var in domain.UpdateQuestionRuleInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	rule, err := h.service.UpdateRule(r.Context(), actor, eventID, ruleID, in)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toQuestionRuleResponse(rule))
}

func (h *QuestionnaireHandler) DeleteRule(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	ruleID, err := uuid.Parse(chi.URLParam(r, "rule_id"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid rule_id", nil))
		return
	}
	var req struct {
		ExpectedVersion int `json:"expected_version"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if err := h.service.DeleteRule(r.Context(), actor, eventID, ruleID, req.ExpectedVersion); err != nil {
		respond.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func parseEventID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid rfx event id", nil))
		return uuid.Nil, false
	}
	return id, true
}
