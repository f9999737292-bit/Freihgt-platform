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

type TemplateQuestionnaireHandler struct {
	service *service.TemplateQuestionnaireService
}

func NewTemplateQuestionnaireHandler(svc *service.TemplateQuestionnaireService) *TemplateQuestionnaireHandler {
	return &TemplateQuestionnaireHandler{service: svc}
}

func (h *TemplateQuestionnaireHandler) GetQuestionnaire(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	templateID, ok := parseTemplateID(w, r)
	if !ok {
		return
	}
	q, err := h.service.GetQuestionnaire(r.Context(), actor, templateID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toTemplateQuestionnaireResponse(q))
}

func (h *TemplateQuestionnaireHandler) GetVersionQuestionnaire(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	templateID, ok := parseTemplateID(w, r)
	if !ok {
		return
	}
	versionID, ok := parseVersionID(w, r)
	if !ok {
		return
	}
	q, err := h.service.GetVersionQuestionnaire(r.Context(), actor, templateID, versionID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toTemplateQuestionnaireResponse(q))
}

func (h *TemplateQuestionnaireHandler) CreateSection(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	templateID, ok := parseTemplateID(w, r)
	if !ok {
		return
	}
	var in domain.CreateSectionInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	sec, err := h.service.CreateSection(r.Context(), actor, templateID, in)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toTemplateSectionResponse(sec))
}

func (h *TemplateQuestionnaireHandler) UpdateSection(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	templateID, ok := parseTemplateID(w, r)
	if !ok {
		return
	}
	sectionID, ok := parseSectionID(w, r)
	if !ok {
		return
	}
	var in domain.UpdateSectionInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	sec, err := h.service.UpdateSection(r.Context(), actor, templateID, sectionID, in)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toTemplateSectionResponse(sec))
}

func (h *TemplateQuestionnaireHandler) DeleteSection(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	templateID, ok := parseTemplateID(w, r)
	if !ok {
		return
	}
	sectionID, ok := parseSectionID(w, r)
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
	if err := h.service.DeleteSection(r.Context(), actor, templateID, sectionID, req.ExpectedVersion); err != nil {
		respond.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *TemplateQuestionnaireHandler) ReorderSections(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	templateID, ok := parseTemplateID(w, r)
	if !ok {
		return
	}
	var in domain.ReorderInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	if err := h.service.ReorderSections(r.Context(), actor, templateID, in.OrderedIDs); err != nil {
		respond.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *TemplateQuestionnaireHandler) CreateQuestion(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	templateID, ok := parseTemplateID(w, r)
	if !ok {
		return
	}
	var in struct {
		SectionID uuid.UUID `json:"section_id"`
		domain.CreateQuestionInput
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	q, err := h.service.CreateQuestion(r.Context(), actor, templateID, in.SectionID, in.CreateQuestionInput)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toTemplateQuestionResponse(q))
}

func (h *TemplateQuestionnaireHandler) UpdateQuestion(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	templateID, ok := parseTemplateID(w, r)
	if !ok {
		return
	}
	questionID, ok := parseQuestionID(w, r)
	if !ok {
		return
	}
	var in domain.UpdateQuestionInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	q, err := h.service.UpdateQuestion(r.Context(), actor, templateID, questionID, in)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toTemplateQuestionResponse(q))
}

func (h *TemplateQuestionnaireHandler) DeleteQuestion(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	templateID, ok := parseTemplateID(w, r)
	if !ok {
		return
	}
	questionID, ok := parseQuestionID(w, r)
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
	if err := h.service.DeleteQuestion(r.Context(), actor, templateID, questionID, req.ExpectedVersion); err != nil {
		respond.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *TemplateQuestionnaireHandler) DuplicateQuestion(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	templateID, ok := parseTemplateID(w, r)
	if !ok {
		return
	}
	questionID, ok := parseQuestionID(w, r)
	if !ok {
		return
	}
	q, err := h.service.DuplicateQuestion(r.Context(), actor, templateID, questionID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toTemplateQuestionResponse(q))
}

func (h *TemplateQuestionnaireHandler) ReorderQuestions(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	templateID, ok := parseTemplateID(w, r)
	if !ok {
		return
	}
	sectionID, ok := parseSectionID(w, r)
	if !ok {
		return
	}
	var in domain.ReorderInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	if err := h.service.ReorderQuestions(r.Context(), actor, templateID, sectionID, in.OrderedIDs); err != nil {
		respond.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *TemplateQuestionnaireHandler) CreateOption(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	templateID, ok := parseTemplateID(w, r)
	if !ok {
		return
	}
	questionID, ok := parseQuestionID(w, r)
	if !ok {
		return
	}
	var in domain.CreateQuestionOptionInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	opt, err := h.service.CreateOption(r.Context(), actor, templateID, questionID, in)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toTemplateQuestionOptionResponse(opt))
}

func (h *TemplateQuestionnaireHandler) UpdateOption(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	templateID, ok := parseTemplateID(w, r)
	if !ok {
		return
	}
	questionID, ok := parseQuestionID(w, r)
	if !ok {
		return
	}
	optionID, ok := parseOptionID(w, r)
	if !ok {
		return
	}
	var in domain.UpdateQuestionOptionInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	opt, err := h.service.UpdateOption(r.Context(), actor, templateID, questionID, optionID, in)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toTemplateQuestionOptionResponse(opt))
}

func (h *TemplateQuestionnaireHandler) DeleteOption(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	templateID, ok := parseTemplateID(w, r)
	if !ok {
		return
	}
	questionID, ok := parseQuestionID(w, r)
	if !ok {
		return
	}
	optionID, ok := parseOptionID(w, r)
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
	if err := h.service.DeleteOption(r.Context(), actor, templateID, questionID, optionID, req.ExpectedVersion); err != nil {
		respond.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *TemplateQuestionnaireHandler) CreateRule(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	templateID, ok := parseTemplateID(w, r)
	if !ok {
		return
	}
	var in domain.CreateQuestionRuleInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	rule, err := h.service.CreateRule(r.Context(), actor, templateID, in)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toTemplateQuestionRuleResponse(rule))
}

func (h *TemplateQuestionnaireHandler) UpdateRule(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	templateID, ok := parseTemplateID(w, r)
	if !ok {
		return
	}
	ruleID, ok := parseRuleID(w, r)
	if !ok {
		return
	}
	var in domain.UpdateQuestionRuleInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	rule, err := h.service.UpdateRule(r.Context(), actor, templateID, ruleID, in)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toTemplateQuestionRuleResponse(rule))
}

func (h *TemplateQuestionnaireHandler) DeleteRule(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	templateID, ok := parseTemplateID(w, r)
	if !ok {
		return
	}
	ruleID, ok := parseRuleID(w, r)
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
	if err := h.service.DeleteRule(r.Context(), actor, templateID, ruleID, req.ExpectedVersion); err != nil {
		respond.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func parseSectionID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "section_id"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid section_id", nil))
		return uuid.Nil, false
	}
	return id, true
}

func parseQuestionID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "question_id"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid question_id", nil))
		return uuid.Nil, false
	}
	return id, true
}

func parseOptionID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "option_id"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid option_id", nil))
		return uuid.Nil, false
	}
	return id, true
}

func parseRuleID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "rule_id"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid rule_id", nil))
		return uuid.Nil, false
	}
	return id, true
}
