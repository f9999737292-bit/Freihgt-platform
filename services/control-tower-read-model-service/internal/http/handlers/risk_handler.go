package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	apperrors "github.com/freight-platform/control-tower-read-model-service/internal/platform/errors"
	"github.com/freight-platform/control-tower-read-model-service/internal/platform/respond"
	"github.com/freight-platform/control-tower-read-model-service/internal/repository"
)

type RiskHandler struct {
	repo *repository.RiskRepository
}

func NewRiskHandler(repo *repository.RiskRepository) *RiskHandler {
	return &RiskHandler{repo: repo}
}

type syncRisksRequest struct {
	Evaluations      []syncRiskEvaluationRequest `json:"evaluations"`
	Materializations []materializeRiskRequest    `json:"materializations"`
	Clears           []clearRiskRequest          `json:"clears"`
}

type syncRiskEvaluationRequest struct {
	RiskKey                string              `json:"riskKey"`
	ShipmentID             string              `json:"shipmentId"`
	PredictedExceptionType string              `json:"predictedExceptionType"`
	Score                  int                 `json:"score"`
	RiskLevel              string              `json:"riskLevel"`
	EvaluatedAt            string              `json:"evaluatedAt"`
	NextEvaluationAt       string              `json:"nextEvaluationAt"`
	ThreatenedDeadlineAt   *string             `json:"threatenedDeadlineAt,omitempty"`
	SignalsHash            string              `json:"signalsHash"`
	Signals                []riskSignalRequest `json:"signals"`
}

type riskSignalRequest struct {
	Code           string         `json:"signalCode"`
	Severity       string         `json:"severity"`
	Weight         int            `json:"weight"`
	ObservedAt     string         `json:"observedAt"`
	Source         string         `json:"source"`
	Value          map[string]any `json:"value,omitempty"`
	ExplanationKey string         `json:"explanationKey"`
}

type materializeRiskRequest struct {
	RiskKey        string `json:"riskKey"`
	ShipmentID     string `json:"shipmentId"`
	PredictedType  string `json:"predictedType"`
	ActualEventID  string `json:"actualEventId"`
	MaterializedAt string `json:"materializedAt"`
}

type clearRiskRequest struct {
	RiskKey     string `json:"riskKey"`
	ShipmentID  string `json:"shipmentId"`
	ClearReason string `json:"clearReason"`
	ClearedAt   string `json:"clearedAt"`
}

type mitigateRiskRequest struct {
	MitigationCode string  `json:"mitigationCode"`
	Comment        *string `json:"comment,omitempty"`
}

func (h *RiskHandler) SyncRisks(w http.ResponseWriter, r *http.Request) {
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}

	var payload syncRisksRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", map[string]any{"field": "body"}))
		return
	}

	evaluations := make([]domain.SyncRiskEvaluation, 0, len(payload.Evaluations))
	for _, item := range payload.Evaluations {
		evaluatedAt, err := time.Parse(time.RFC3339, item.EvaluatedAt)
		if err != nil {
			continue
		}
		nextEval, err := time.Parse(time.RFC3339, item.NextEvaluationAt)
		if err != nil {
			nextEval = evaluatedAt.Add(5 * time.Minute)
		}
		eval := domain.SyncRiskEvaluation{
			RiskKey: item.RiskKey, ShipmentID: item.ShipmentID,
			PredictedExceptionType: item.PredictedExceptionType,
			Score:                  item.Score, RiskLevel: item.RiskLevel,
			EvaluatedAt: evaluatedAt.UTC(), NextEvaluationAt: nextEval.UTC(),
			SignalsHash: item.SignalsHash,
		}
		if item.ThreatenedDeadlineAt != nil {
			if t, err := time.Parse(time.RFC3339, *item.ThreatenedDeadlineAt); err == nil {
				utc := t.UTC()
				eval.ThreatenedDeadlineAt = &utc
			}
		}
		for _, sig := range item.Signals {
			observedAt, _ := time.Parse(time.RFC3339, sig.ObservedAt)
			eval.Signals = append(eval.Signals, domain.RiskSignal{
				Code: sig.Code, Severity: sig.Severity, Weight: sig.Weight,
				ObservedAt: observedAt.UTC(), Source: sig.Source,
				ValueJSON: sig.Value, ExplanationKey: sig.ExplanationKey,
			})
		}
		evaluations = append(evaluations, eval)
	}

	materializations := make([]domain.MaterializeRiskInput, 0, len(payload.Materializations))
	for _, item := range payload.Materializations {
		at, err := time.Parse(time.RFC3339, item.MaterializedAt)
		if err != nil {
			at = time.Now().UTC()
		}
		materializations = append(materializations, domain.MaterializeRiskInput{
			RiskKey: item.RiskKey, ShipmentID: item.ShipmentID,
			PredictedType: item.PredictedType, ActualEventID: item.ActualEventID,
			MaterializedAt: at.UTC(),
		})
	}

	clears := make([]domain.ClearRiskInput, 0, len(payload.Clears))
	for _, item := range payload.Clears {
		at, err := time.Parse(time.RFC3339, item.ClearedAt)
		if err != nil {
			at = time.Now().UTC()
		}
		clears = append(clears, domain.ClearRiskInput{
			RiskKey: item.RiskKey, ShipmentID: item.ShipmentID,
			ClearReason: item.ClearReason, ClearedAt: at.UTC(),
		})
	}

	if err := h.repo.SyncEvaluations(r.Context(), tenantID, evaluations, materializations, clears); err != nil {
		respond.Error(w, apperrors.Internal("failed to sync risks", err))
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{"synced": len(evaluations)})
}

func (h *RiskHandler) ListRisks(w http.ResponseWriter, r *http.Request) {
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	filter := repository.RiskListFilter{
		Level:          strings.TrimSpace(r.URL.Query().Get("level")),
		Status:         strings.TrimSpace(r.URL.Query().Get("status")),
		PredictedType:  strings.TrimSpace(r.URL.Query().Get("predictedType")),
		ShipmentID:     strings.TrimSpace(r.URL.Query().Get("shipmentId")),
		ActiveOnly:     r.URL.Query().Get("activeOnly") == "true",
		MitigatingOnly: r.URL.Query().Get("mitigatingOnly") == "true",
	}
	items, err := h.repo.ListRisks(r.Context(), tenantID, filter)
	if err != nil {
		respond.Error(w, apperrors.Internal("failed to list risks", err))
		return
	}
	resp := make([]any, 0, len(items))
	for _, item := range items {
		signals, _ := h.repo.GetLatestSignals(r.Context(), tenantID, item.ID)
		resp = append(resp, toRiskResponse(item, signals, nil))
	}
	respond.JSON(w, http.StatusOK, map[string]any{"items": resp})
}

func (h *RiskHandler) GetRisk(w http.ResponseWriter, r *http.Request) {
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	riskKey := strings.ToLower(strings.TrimSpace(chi.URLParam(r, "riskKey")))
	item, err := h.repo.GetRisk(r.Context(), tenantID, riskKey)
	if err != nil {
		respond.Error(w, apperrors.NotFound("risk not found"))
		return
	}
	signals, _ := h.repo.GetLatestSignals(r.Context(), tenantID, item.ID)
	actions, _ := h.repo.ListActions(r.Context(), tenantID, item.ID)
	respond.JSON(w, http.StatusOK, toRiskResponse(item, signals, actions))
}

func (h *RiskHandler) GetRiskKPI(w http.ResponseWriter, r *http.Request) {
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	kpi, err := h.repo.CountKPI(r.Context(), tenantID)
	if err != nil {
		respond.Error(w, apperrors.Internal("failed to count risk kpi", err))
		return
	}
	respond.JSON(w, http.StatusOK, kpi)
}

func (h *RiskHandler) AcknowledgeRisk(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, ok := h.resolveRiskMutation(w, r)
	if !ok {
		return
	}
	riskKey := strings.ToLower(strings.TrimSpace(chi.URLParam(r, "riskKey")))
	item, err := h.repo.AcknowledgeRisk(r.Context(), domain.AcknowledgeRiskInput{
		TenantID: tenantID, ActorUserID: userID, RiskKey: riskKey,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}
	signals, _ := h.repo.GetLatestSignals(r.Context(), tenantID, item.ID)
	respond.JSON(w, http.StatusOK, toRiskResponse(item, signals, nil))
}

func (h *RiskHandler) MitigateRisk(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, ok := h.resolveRiskMutation(w, r)
	if !ok {
		return
	}
	var payload mitigateRiskRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", map[string]any{"field": "body"}))
		return
	}
	code := strings.TrimSpace(payload.MitigationCode)
	if code == "" {
		respond.Error(w, apperrors.Validation("mitigationCode is required", map[string]any{"field": "mitigationCode"}))
		return
	}
	riskKey := strings.ToLower(strings.TrimSpace(chi.URLParam(r, "riskKey")))
	item, err := h.repo.MitigateRisk(r.Context(), domain.MitigateRiskInput{
		TenantID: tenantID, ActorUserID: userID, RiskKey: riskKey,
		MitigationCode: code, MitigationComment: payload.Comment,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}
	signals, _ := h.repo.GetLatestSignals(r.Context(), tenantID, item.ID)
	respond.JSON(w, http.StatusOK, toRiskResponse(item, signals, nil))
}

func (h *RiskHandler) resolveRiskMutation(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return uuid.Nil, uuid.Nil, false
	}
	userID, err := resolveVerifiedUser(r)
	if err != nil || userID == uuid.Nil {
		respond.Error(w, apperrors.Unauthorized("verified user context is required"))
		return uuid.Nil, uuid.Nil, false
	}
	return tenantID, userID, true
}

func toRiskResponse(item domain.ShipmentRisk, signals []domain.RiskSignal, actions []domain.RiskAction) map[string]any {
	resp := map[string]any{
		"riskId": item.RiskKey, "shipmentId": item.ShipmentID.String(),
		"predictedExceptionType": item.PredictedExceptionType,
		"score":                  item.Score, "level": item.RiskLevel, "status": item.Status,
		"firstDetectedAt": item.FirstDetectedAt.UTC().Format(time.RFC3339),
		"evaluatedAt":     item.EvaluatedAt.UTC().Format(time.RFC3339),
	}
	if item.NextEvaluationAt != nil {
		resp["nextEvaluationAt"] = item.NextEvaluationAt.UTC().Format(time.RFC3339)
	}
	if item.ThreatenedDeadlineAt != nil {
		resp["threatenedDeadlineAt"] = item.ThreatenedDeadlineAt.UTC().Format(time.RFC3339)
	}
	if item.ClearedAt != nil {
		resp["clearedAt"] = item.ClearedAt.UTC().Format(time.RFC3339)
	}
	if item.ClearReason != nil {
		resp["clearReason"] = *item.ClearReason
	}
	if item.MaterializedAt != nil {
		resp["materializedAt"] = item.MaterializedAt.UTC().Format(time.RFC3339)
	}
	if item.ActualEventID != nil {
		resp["actualEventId"] = *item.ActualEventID
	}
	if item.MitigationCode != nil {
		resp["mitigationCode"] = *item.MitigationCode
	}
	if item.MitigationComment != nil {
		resp["mitigationComment"] = *item.MitigationComment
	}
	sigItems := make([]map[string]any, 0, len(signals))
	for _, s := range signals {
		sigItems = append(sigItems, map[string]any{
			"signalCode": s.Code, "severity": s.Severity, "weight": s.Weight,
			"observedAt": s.ObservedAt.UTC().Format(time.RFC3339), "source": s.Source,
			"value": s.ValueJSON, "explanationKey": s.ExplanationKey,
		})
	}
	resp["signals"] = sigItems
	if actions != nil {
		actionItems := make([]map[string]any, 0, len(actions))
		for _, a := range actions {
			entry := map[string]any{
				"actionType": a.ActionType, "occurredAt": a.OccurredAt.UTC().Format(time.RFC3339),
			}
			if a.ActorUserID != nil {
				entry["actorUserId"] = a.ActorUserID.String()
			}
			if a.Metadata != nil {
				entry["metadata"] = a.Metadata
			}
			actionItems = append(actionItems, entry)
		}
		resp["actions"] = actionItems
	}
	return resp
}
