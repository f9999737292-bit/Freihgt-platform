package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain/tender"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/platform/respond"
	"github.com/freight-platform/rfx-service/internal/service"
)

type EvaluationHandler struct {
	svc *service.EvaluationService
}

func NewEvaluationHandler(svc *service.EvaluationService) *EvaluationHandler {
	return &EvaluationHandler{svc: svc}
}

type createScoringTemplateRequest struct {
	TenantID string                       `json:"tenant_id"`
	Code     string                       `json:"code"`
	Name     string                       `json:"name"`
	Factors  []tender.ScoringFactorWeight `json:"factors"`
}

func (h *EvaluationHandler) CreateScoringTemplate(w http.ResponseWriter, r *http.Request) {
	var req createScoringTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}
	tenantID, err := uuid.Parse(req.TenantID)
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid tenant_id", map[string]any{"field": "tenant_id"}))
		return
	}
	templateID, versionID, err := h.svc.CreateScoringTemplate(r.Context(), tenantID, req.Code, req.Name, req.Factors, nil)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, map[string]string{
		"template_id": templateID.String(),
		"version_id":  versionID.String(),
	})
}

type runEvaluationRequest struct {
	TenantID                 string                    `json:"tenant_id"`
	ScoringTemplateVersionID string                    `json:"scoring_template_version_id"`
	QualificationRules       tender.QualificationRules `json:"qualification_rules"`
	RequiredVolume           float64                   `json:"required_volume"`
}

func (h *EvaluationHandler) RunEvaluation(w http.ResponseWriter, r *http.Request) {
	eventID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid rfx event id", map[string]any{"field": "id"}))
		return
	}
	var req runEvaluationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}
	tenantID, err := uuid.Parse(req.TenantID)
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid tenant_id", map[string]any{"field": "tenant_id"}))
		return
	}
	versionID, err := uuid.Parse(req.ScoringTemplateVersionID)
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid scoring_template_version_id", map[string]any{"field": "scoring_template_version_id"}))
		return
	}
	result, err := h.svc.RunEvaluation(r.Context(), service.RunEvaluationInput{
		TenantID:                 tenantID,
		RfxEventID:               eventID,
		ScoringTemplateVersionID: versionID,
		QualificationRules:       req.QualificationRules,
		RequiredVolume:           req.RequiredVolume,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, result)
}

type runAllocationScenarioRequest struct {
	TenantID     string                    `json:"tenant_id"`
	EvaluationID string                    `json:"evaluation_id"`
	Name         string                    `json:"name"`
	Config       tender.AllocationConfig   `json:"config"`
	QuotaTargets []tender.QuotaTarget      `json:"quota_targets"`
	QuotaPolicy  tender.QuotaBalancePolicy `json:"quota_policy"`
	ActualShares map[string]float64        `json:"actual_shares"`
}

func (h *EvaluationHandler) RunAllocationScenario(w http.ResponseWriter, r *http.Request) {
	var req runAllocationScenarioRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}
	tenantID, err := uuid.Parse(req.TenantID)
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid tenant_id", map[string]any{"field": "tenant_id"}))
		return
	}
	evalID, err := uuid.Parse(req.EvaluationID)
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid evaluation_id", map[string]any{"field": "evaluation_id"}))
		return
	}
	scenarioID, outcome, positions, err := h.svc.RunAllocationScenario(r.Context(), service.CreateAllocationScenarioInput{
		TenantID:     tenantID,
		EvaluationID: evalID,
		Name:         req.Name,
		Config:       req.Config,
		QuotaTargets: req.QuotaTargets,
		QuotaPolicy:  req.QuotaPolicy,
		ActualShares: req.ActualShares,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{
		"scenario_id": scenarioID.String(),
		"outcome":     outcome,
		"quota":       positions,
	})
}

type awardProposalRequest struct {
	TenantID       string  `json:"tenant_id"`
	EvaluationID   string  `json:"evaluation_id"`
	ScenarioID     string  `json:"scenario_id"`
	RfxEventID     string  `json:"rfx_event_id"`
	IdempotencyKey *string `json:"idempotency_key"`
}

func (h *EvaluationHandler) CreateAwardProposal(w http.ResponseWriter, r *http.Request) {
	var req awardProposalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}
	tenantID, err := uuid.Parse(req.TenantID)
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid tenant_id", map[string]any{"field": "tenant_id"}))
		return
	}
	eventID, err := uuid.Parse(req.RfxEventID)
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid rfx_event_id", map[string]any{"field": "rfx_event_id"}))
		return
	}
	evalID, err := uuid.Parse(req.EvaluationID)
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid evaluation_id", map[string]any{"field": "evaluation_id"}))
		return
	}
	scenarioID, err := uuid.Parse(req.ScenarioID)
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid scenario_id", map[string]any{"field": "scenario_id"}))
		return
	}
	id, err := h.svc.CreateAwardProposal(r.Context(), tenantID, eventID, evalID, scenarioID, nil, req.IdempotencyKey)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, map[string]string{"proposal_id": id.String()})
}

func (h *EvaluationHandler) SubmitAwardProposal(w http.ResponseWriter, r *http.Request) {
	h.proposalAction(w, r, func(id, tenantID uuid.UUID) error {
		return h.svc.SubmitAwardProposal(r.Context(), id, tenantID)
	})
}

func (h *EvaluationHandler) ApproveAwardProposal(w http.ResponseWriter, r *http.Request) {
	h.proposalAction(w, r, func(id, tenantID uuid.UUID) error {
		return h.svc.ApproveAwardProposal(r.Context(), id, tenantID, uuid.Nil)
	})
}

func (h *EvaluationHandler) FinalizeAward(w http.ResponseWriter, r *http.Request) {
	proposalID, err := uuid.Parse(chi.URLParam(r, "proposal_id"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid proposal id", map[string]any{"field": "proposal_id"}))
		return
	}
	var body struct {
		TenantID       string  `json:"tenant_id"`
		IdempotencyKey *string `json:"idempotency_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}
	tenantID, err := uuid.Parse(body.TenantID)
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid tenant_id", map[string]any{"field": "tenant_id"}))
		return
	}
	awardID, err := h.svc.FinalizeAward(r.Context(), proposalID, tenantID, uuid.Nil, body.IdempotencyKey)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"award_id": awardID.String()})
}

func (h *EvaluationHandler) proposalAction(w http.ResponseWriter, r *http.Request, fn func(uuid.UUID, uuid.UUID) error) {
	proposalID, err := uuid.Parse(chi.URLParam(r, "proposal_id"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid proposal id", map[string]any{"field": "proposal_id"}))
		return
	}
	var body struct {
		TenantID string `json:"tenant_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}
	tenantID, err := uuid.Parse(body.TenantID)
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid tenant_id", map[string]any{"field": "tenant_id"}))
		return
	}
	if err := fn(proposalID, tenantID); err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
