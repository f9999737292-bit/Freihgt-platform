package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain/tender"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	rfxmetrics "github.com/freight-platform/rfx-service/internal/platform/metrics"
	"github.com/freight-platform/rfx-service/internal/platform/respond"
	"github.com/freight-platform/rfx-service/internal/repository"
	"github.com/freight-platform/rfx-service/internal/service"
)

type EvaluationHandler struct {
	svc  *service.EvaluationService
	auth PermissionChecker
}

func NewEvaluationHandler(svc *service.EvaluationService, auth PermissionChecker) *EvaluationHandler {
	return &EvaluationHandler{svc: svc, auth: auth}
}

type createScoringTemplateRequest struct {
	TenantID string                       `json:"tenant_id"`
	Code     string                       `json:"code"`
	Name     string                       `json:"name"`
	Factors  []tender.ScoringFactorWeight `json:"factors"`
}

func (h *EvaluationHandler) CreateScoringTemplate(w http.ResponseWriter, r *http.Request) {
	if err := h.auth.UserHasPermission(r, repository.PermissionRfxEvaluate); err != nil {
		respond.Error(w, err)
		return
	}
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
	if err := h.auth.UserHasPermission(r, repository.PermissionRfxEvaluate); err != nil {
		respond.Error(w, err)
		return
	}
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
		rfxmetrics.IncEvaluation("error")
		respond.Error(w, err)
		return
	}
	rfxmetrics.IncEvaluation("success")
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
	if err := h.auth.UserHasPermission(r, repository.PermissionRfxEvaluate); err != nil {
		respond.Error(w, err)
		return
	}
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
		rfxmetrics.IncAllocation("error")
		respond.Error(w, err)
		return
	}
	rfxmetrics.IncAllocation(string(outcome.Status))
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
	if err := h.auth.UserHasPermission(r, repository.PermissionRfxEvaluate); err != nil {
		respond.Error(w, err)
		return
	}
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
		rfxmetrics.IncAwardProposal("error")
		respond.Error(w, err)
		return
	}
	rfxmetrics.IncAwardProposal("success")
	respond.JSON(w, http.StatusCreated, map[string]string{"proposal_id": id.String()})
}

func (h *EvaluationHandler) SubmitAwardProposal(w http.ResponseWriter, r *http.Request) {
	h.proposalAction(w, r, func(id, tenantID uuid.UUID) error {
		return h.svc.SubmitAwardProposal(r.Context(), id, tenantID)
	})
}

func (h *EvaluationHandler) ApproveAwardProposal(w http.ResponseWriter, r *http.Request) {
	h.proposalActionWithUser(w, r, repository.PermissionRfxApproveAward, func(id, tenantID, userID uuid.UUID) error {
		return h.svc.ApproveAwardProposal(r.Context(), id, tenantID, userID)
	})
}

func (h *EvaluationHandler) RejectAwardProposal(w http.ResponseWriter, r *http.Request) {
	h.proposalActionWithUser(w, r, repository.PermissionRfxApproveAward, func(id, tenantID, userID uuid.UUID) error {
		return h.svc.RejectAwardProposal(r.Context(), id, tenantID, userID)
	})
}

func (h *EvaluationHandler) FinalizeAward(w http.ResponseWriter, r *http.Request) {
	if err := h.auth.UserHasPermission(r, repository.PermissionRfxAward); err != nil {
		respond.Error(w, err)
		return
	}
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
	userID, err := resolveVerifiedUser(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	awardID, conversion, err := h.svc.FinalizeAward(r.Context(), proposalID, tenantID, userID, body.IdempotencyKey)
	if err != nil {
		rfxmetrics.IncAward("error")
		respond.Error(w, err)
		return
	}
	rfxmetrics.IncAward("success")
	if conversion != nil {
		rfxmetrics.IncAwardConversion(conversion.Status)
	}
	resp := map[string]any{"award_id": awardID.String()}
	if conversion != nil {
		resp["conversion"] = conversion
	}
	respond.JSON(w, http.StatusOK, resp)
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

func (h *EvaluationHandler) proposalActionWithUser(w http.ResponseWriter, r *http.Request, permission string, fn func(uuid.UUID, uuid.UUID, uuid.UUID) error) {
	if err := h.auth.UserHasPermission(r, permission); err != nil {
		respond.Error(w, err)
		return
	}
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
	userID, err := resolveVerifiedUser(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if err := fn(proposalID, tenantID, userID); err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
