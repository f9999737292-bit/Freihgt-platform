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

type VersionLifecycleHandler struct {
	service *service.VersionLifecycleService
}

func NewVersionLifecycleHandler(svc *service.VersionLifecycleService) *VersionLifecycleHandler {
	return &VersionLifecycleHandler{service: svc}
}

func (h *VersionLifecycleHandler) ListVersions(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	versions, err := h.service.ListVersions(r.Context(), actor, eventID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	items := make([]map[string]any, 0, len(versions))
	for i := range versions {
		items = append(items, toRfxVersionResponse(&versions[i]))
	}
	respond.JSON(w, http.StatusOK, map[string]any{"versions": items})
}

func (h *VersionLifecycleHandler) GetVersion(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	versionID, ok := parseVersionID(w, r)
	if !ok {
		return
	}
	view, err := h.service.GetVersion(r.Context(), actor, eventID, versionID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{
		"version":       toRfxVersionResponse(&view.Version),
		"questionnaire": toQuestionnaireDefinitionResponse(&view.Questionnaire),
	})
}

func (h *VersionLifecycleHandler) PublishQuestionnaire(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	var input domain.PublishQuestionnaireInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	version, err := h.service.PublishQuestionnaire(r.Context(), actor, eventID, r.Header.Get("Idempotency-Key"), input)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toRfxVersionResponse(version))
}

func (h *VersionLifecycleHandler) PreviewChangeImpact(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	var input domain.PreviewChangeImpactInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	analysis, err := h.service.PreviewChangeImpact(r.Context(), actor, eventID, input)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toChangeImpactAnalysisResponse(analysis))
}

func (h *VersionLifecycleHandler) ForkDraftFromPublished(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	version, err := h.service.ForkDraftFromPublished(r.Context(), actor, eventID, r.Header.Get("Idempotency-Key"))
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toRfxVersionResponse(version))
}

func (h *VersionLifecycleHandler) CompareVersions(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	var input domain.CompareVersionsInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	result, err := h.service.CompareVersions(r.Context(), actor, eventID, input)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toCompareVersionsResponse(result))
}

func (h *VersionLifecycleHandler) RestoreVersionAsDraft(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	sourceVersionID, ok := parseVersionID(w, r)
	if !ok {
		return
	}
	var input domain.RestoreVersionAsDraftInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	version, err := h.service.RestoreVersionAsDraft(r.Context(), actor, eventID, sourceVersionID, r.Header.Get("Idempotency-Key"), input)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toRfxVersionResponse(version))
}

func parseVersionID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "version_id"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid version_id", nil))
		return uuid.Nil, false
	}
	return id, true
}

func toCompareVersionsResponse(result *domain.CompareVersionsResult) map[string]any {
	return map[string]any{
		"source_version":        result.SourceVersion,
		"target_version":        result.TargetVersion,
		"source_version_number": result.SourceVersionNumber,
		"target_version_number": result.TargetVersionNumber,
		"summary":               result.Summary,
		"canonical_diff_hash":   result.CanonicalDiffHash,
		"differences":           toCompareItemDiffResponses(result.Differences),
		"sections":              toCompareItemDiffResponses(result.Sections),
		"questions":             toCompareItemDiffResponses(result.Questions),
		"options":               toCompareItemDiffResponses(result.Options),
		"rules":                 toCompareItemDiffResponses(result.Rules),
		"scoring": map[string]any{
			"criteria": toCompareItemDiffResponses(result.Scoring.Criteria),
			"bindings": toCompareItemDiffResponses(result.Scoring.Bindings),
			"model":    toCompareItemDiffResponse(result.Scoring.Model),
		},
	}
}

func toCompareItemDiffResponses(items []domain.CompareItemDiff) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for i := range items {
		out = append(out, toCompareItemDiffResponse(&items[i]))
	}
	return out
}

func toCompareItemDiffResponse(item *domain.CompareItemDiff) map[string]any {
	if item == nil {
		return nil
	}
	resp := map[string]any{
		"entity_type": item.EntityType,
		"change":      item.Change,
	}
	if item.SectionCode != "" {
		resp["section_code"] = item.SectionCode
	}
	if item.QuestionCode != "" {
		resp["question_code"] = item.QuestionCode
	}
	if item.OptionCode != "" {
		resp["option_code"] = item.OptionCode
	}
	if item.RuleCode != "" {
		resp["rule_code"] = item.RuleCode
	}
	if item.CriterionCode != "" {
		resp["criterion_code"] = item.CriterionCode
	}
	if len(item.Fields) > 0 {
		resp["fields"] = item.Fields
	}
	if len(item.FieldDiffs) > 0 {
		fieldDiffs := make([]map[string]any, 0, len(item.FieldDiffs))
		for _, diff := range item.FieldDiffs {
			fieldDiffs = append(fieldDiffs, map[string]any{
				"field":  diff.Field,
				"before": diff.Before,
				"after":  diff.After,
			})
		}
		resp["field_diffs"] = fieldDiffs
	}
	return resp
}

func toChangeImpactAnalysisResponse(analysis *domain.ChangeImpactAnalysis) map[string]any {
	resp := map[string]any{
		"impact_analysis_id":                analysis.ID,
		"tenant_id":                         analysis.TenantID,
		"event_id":                          analysis.EventID,
		"candidate_version_id":              analysis.CandidateVersionID,
		"canonical_diff_hash":               analysis.CanonicalDiffHash,
		"impact_classes":                    analysis.ImpactClasses,
		"affected_draft_response_count":     analysis.AffectedDraftResponseCount,
		"affected_submitted_response_count": analysis.AffectedSubmittedResponseCount,
		"scoring_affecting":                 analysis.ScoringAffecting,
		"knockout_affecting":                analysis.KnockoutAffecting,
		"expires_at":                        analysis.ExpiresAt,
	}
	if analysis.SourceVersionID != nil {
		resp["source_version_id"] = *analysis.SourceVersionID
	} else {
		resp["source_version_id"] = nil
	}
	return resp
}
