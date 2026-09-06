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

type CarrierResponseHandler struct {
	service *service.CarrierResponseService
}

func NewCarrierResponseHandler(svc *service.CarrierResponseService) *CarrierResponseHandler {
	return &CarrierResponseHandler{service: svc}
}

func (h *CarrierResponseHandler) GetCarrierResponse(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	carrierCompanyID, err := parseCarrierCompanyIDQuery(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	workspace, err := h.service.GetWorkspace(r.Context(), actor, eventID, carrierCompanyID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toCarrierResponseWorkspaceResponse(workspace))
}

func (h *CarrierResponseHandler) StartCarrierResponse(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	carrierCompanyID, err := parseCarrierCompanyIDQuery(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	workspace, err := h.service.StartOrResume(r.Context(), actor, eventID, carrierCompanyID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toCarrierResponseWorkspaceResponse(workspace))
}

func (h *CarrierResponseHandler) PatchCarrierAnswers(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	carrierCompanyID, err := parseCarrierCompanyIDQuery(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req struct {
		SaveVersion int64 `json:"save_version"`
		Answers     []struct {
			SectionID  string          `json:"section_id"`
			QuestionID string          `json:"question_id"`
			Field      string          `json:"field"`
			Value      json.RawMessage `json:"value"`
		} `json:"answers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	patches := make([]domain.AnswerPatchItem, 0, len(req.Answers))
	for _, item := range req.Answers {
		questionID, err := domain.ParseUUID(item.QuestionID, "question_id")
		if err != nil {
			respond.Error(w, err)
			return
		}
		var sectionID uuid.UUID
		if raw := item.SectionID; raw != "" {
			sectionID, err = domain.ParseUUID(raw, "section_id")
			if err != nil {
				respond.Error(w, err)
				return
			}
		}
		field := item.Field
		if field == "" {
			field = "value"
		}
		patches = append(patches, domain.AnswerPatchItem{
			SectionID: sectionID, QuestionID: questionID, Field: field, Value: item.Value,
		})
	}
	result, err := h.service.SaveAnswers(r.Context(), actor, eventID, carrierCompanyID, domain.AnswerBatchPatchInput{
		ExpectedSaveVersion: req.SaveVersion,
		Answers:             patches,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toResponseSaveResultResponse(result))
}

func (h *CarrierResponseHandler) ValidateCarrierResponse(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	carrierCompanyID, err := parseCarrierCompanyIDQuery(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	result, err := h.service.ValidateResponse(r.Context(), actor, eventID, carrierCompanyID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toResponseValidationResultResponse(result))
}

func (h *CarrierResponseHandler) SubmitCarrierResponse(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	carrierCompanyID, err := parseCarrierCompanyIDQuery(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req struct {
		SaveVersion int64 `json:"save_version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	result, err := h.service.Submit(r.Context(), actor, eventID, carrierCompanyID, req.SaveVersion)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toSubmitResultResponse(result))
}

func (h *CarrierResponseHandler) GetCarrierResponseSummary(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	carrierCompanyID, err := parseCarrierCompanyIDQuery(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	result, err := h.service.GetSummary(r.Context(), actor, eventID, carrierCompanyID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toResponseValidationResultResponse(result))
}

func toCarrierResponseWorkspaceResponse(workspace *domain.CarrierResponseWorkspace) map[string]any {
	resp := toCarrierResponseMetaResponse(&workspace.Response)
	resp["questionnaire"] = toQuestionnaireDefinitionResponse(&workspace.Questionnaire)
	resp["answers"] = toCarrierAnswerResponses(workspace.Answers)
	return resp
}

func toCarrierResponseMetaResponse(response *domain.RfxResponse) map[string]any {
	resp := toRfxResponseResponse(response)
	resp["product_status"] = domain.MapResponseStatusToProduct(response.Status)
	resp["save_version"] = response.SaveVersion
	resp["completion_percent"] = response.CompletionPercent
	resp["last_saved_at"] = formatDateTime(response.LastSavedAt)
	if response.RfxVersionID != nil {
		resp["rfx_version_id"] = response.RfxVersionID.String()
	} else {
		resp["rfx_version_id"] = nil
	}
	if response.LastSavedBy != nil {
		resp["last_saved_by"] = response.LastSavedBy.String()
	} else {
		resp["last_saved_by"] = nil
	}
	return resp
}

func toCarrierAnswerResponses(answers []domain.CarrierAnswer) []map[string]any {
	out := make([]map[string]any, 0, len(answers))
	for i := range answers {
		var value any
		if len(answers[i].AnswerValueJSON) > 0 {
			_ = json.Unmarshal(answers[i].AnswerValueJSON, &value)
		}
		entry := map[string]any{
			"id":                 answers[i].ID.String(),
			"question_id":        answers[i].QuestionID.String(),
			"value":              value,
			"answer_source":      answers[i].AnswerSource,
			"validation_version": answers[i].ValidationVersion,
			"updated_at":         answers[i].UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
			"version":            answers[i].Version,
		}
		if answers[i].UpdatedBy != nil {
			entry["updated_by"] = answers[i].UpdatedBy.String()
		}
		out = append(out, entry)
	}
	return out
}

func toResponseSaveResultResponse(result *domain.ResponseSaveResult) map[string]any {
	return map[string]any{
		"response_id":         result.ResponseID.String(),
		"save_version":        result.SaveVersion,
		"last_saved_at":       result.LastSavedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		"last_saved_by":       result.LastSavedBy.String(),
		"completion_percent":  result.CompletionPercent,
	}
}

func toResponseValidationResultResponse(result *domain.ResponseValidationResult) map[string]any {
	errors := make([]map[string]any, 0, len(result.Errors))
	for _, item := range result.Errors {
		entry := map[string]any{
			"field":       item.Field,
			"rule":        item.Rule,
			"message_key": item.MessageKey,
		}
		if item.SectionID != uuid.Nil {
			entry["section_id"] = item.SectionID.String()
		}
		if item.QuestionID != uuid.Nil {
			entry["question_id"] = item.QuestionID.String()
		}
		if len(item.Params) > 0 {
			entry["params"] = item.Params
		}
		errors = append(errors, entry)
	}
	return map[string]any{
		"valid":                result.Valid,
		"blocking_error_count": result.BlockingErrorCount,
		"errors":               errors,
		"completion_percent":   result.CompletionPercent,
	}
}

func toSubmitResultResponse(result *domain.SubmitResult) map[string]any {
	return map[string]any{
		"response_id":  result.ResponseID.String(),
		"status":       result.Status,
		"submitted_at": result.SubmittedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		"save_version": result.SaveVersion,
	}
}

func parseCarrierResponseEventID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return uuid.Nil, false
	}
	return id, true
}
