package controltower

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/api-gateway/internal/controltowerreadmodel"
	"github.com/freight-platform/api-gateway/internal/platform/respond"
)

func (h *Handler) ListAutomationRules(w http.ResponseWriter, r *http.Request) {
	h.handleAutomationRead(w, r, CanViewAutomation, "view automation rules denied", func(reqCtx RequestContext) (json.RawMessage, int, error) {
		payload, err := h.service.ListAutomationRules(r.Context(), reqCtx, r)
		return payload, http.StatusOK, err
	})
}

func (h *Handler) GetAutomationRule(w http.ResponseWriter, r *http.Request) {
	ruleID := chi.URLParam(r, "ruleId")
	h.handleAutomationProxy(w, r, http.MethodGet, CanViewAutomation, "view automation rule denied",
		controltowerreadmodel.AutomationRulePath(ruleID, ""), nil, http.StatusOK)
}

func (h *Handler) CreateAutomationRule(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	h.handleAutomationProxy(w, r, http.MethodPost, CanManageAutomationRules, "manage automation rules denied",
		controltowerreadmodel.AutomationRulesPath, body, http.StatusCreated)
}

func (h *Handler) UpdateAutomationRule(w http.ResponseWriter, r *http.Request) {
	ruleID := chi.URLParam(r, "ruleId")
	body, _ := io.ReadAll(r.Body)
	h.handleAutomationProxy(w, r, http.MethodPatch, CanManageAutomationRules, "manage automation rules denied",
		controltowerreadmodel.AutomationRulePath(ruleID, ""), body, http.StatusOK)
}

func (h *Handler) ActivateAutomationRule(w http.ResponseWriter, r *http.Request) {
	ruleID := chi.URLParam(r, "ruleId")
	h.handleAutomationProxy(w, r, http.MethodPost, CanManageAutomationRules, "manage automation rules denied",
		controltowerreadmodel.AutomationRulePath(ruleID, "activate"), nil, http.StatusOK)
}

func (h *Handler) DisableAutomationRule(w http.ResponseWriter, r *http.Request) {
	ruleID := chi.URLParam(r, "ruleId")
	h.handleAutomationProxy(w, r, http.MethodPost, CanManageAutomationRules, "manage automation rules denied",
		controltowerreadmodel.AutomationRulePath(ruleID, "disable"), nil, http.StatusOK)
}

func (h *Handler) RetireAutomationRule(w http.ResponseWriter, r *http.Request) {
	ruleID := chi.URLParam(r, "ruleId")
	h.handleAutomationProxy(w, r, http.MethodPost, CanManageAutomationRules, "manage automation rules denied",
		controltowerreadmodel.AutomationRulePath(ruleID, "retire"), nil, http.StatusOK)
}

func (h *Handler) DryRunAutomationRule(w http.ResponseWriter, r *http.Request) {
	ruleID := chi.URLParam(r, "ruleId")
	body, _ := io.ReadAll(r.Body)
	h.handleAutomationProxy(w, r, http.MethodPost, CanManageAutomationRules, "dry-run automation rule denied",
		controltowerreadmodel.AutomationRulePath(ruleID, "dry-run"), body, http.StatusOK)
}

func (h *Handler) ListPlaybooks(w http.ResponseWriter, r *http.Request) {
	h.handleAutomationRead(w, r, CanManagePlaybooks, "view playbooks denied", func(reqCtx RequestContext) (json.RawMessage, int, error) {
		payload, err := h.service.ListPlaybooks(r.Context(), reqCtx, r)
		return payload, http.StatusOK, err
	})
}

func (h *Handler) GetPlaybook(w http.ResponseWriter, r *http.Request) {
	playbookID := chi.URLParam(r, "playbookId")
	h.handleAutomationProxy(w, r, http.MethodGet, CanManagePlaybooks, "view playbook denied",
		controltowerreadmodel.FormatPlaybookPath(playbookID), nil, http.StatusOK)
}

func (h *Handler) CreatePlaybook(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	h.handleAutomationProxy(w, r, http.MethodPost, CanManagePlaybooks, "manage playbooks denied",
		controltowerreadmodel.PlaybooksPath, body, http.StatusCreated)
}

func (h *Handler) UpdatePlaybook(w http.ResponseWriter, r *http.Request) {
	playbookID := chi.URLParam(r, "playbookId")
	body, _ := io.ReadAll(r.Body)
	h.handleAutomationProxy(w, r, http.MethodPatch, CanManagePlaybooks, "manage playbooks denied",
		controltowerreadmodel.FormatPlaybookPath(playbookID), body, http.StatusOK)
}

func (h *Handler) ListRecommendations(w http.ResponseWriter, r *http.Request) {
	h.handleAutomationRead(w, r, CanViewRecommendations, "view recommendations denied", func(reqCtx RequestContext) (json.RawMessage, int, error) {
		payload, err := h.service.ListRecommendations(r.Context(), reqCtx, r)
		return payload, http.StatusOK, err
	})
}

func (h *Handler) GetRecommendation(w http.ResponseWriter, r *http.Request) {
	recID := chi.URLParam(r, "recommendationId")
	h.handleAutomationProxy(w, r, http.MethodGet, CanViewRecommendations, "view recommendation denied",
		controltowerreadmodel.AutomationRecommendationPath(recID, ""), nil, http.StatusOK)
}

func (h *Handler) AcceptRecommendation(w http.ResponseWriter, r *http.Request) {
	recID := chi.URLParam(r, "recommendationId")
	h.handleAutomationProxy(w, r, http.MethodPost, CanStartPlaybook, "start playbook denied",
		controltowerreadmodel.AutomationRecommendationPath(recID, "accept"), nil, http.StatusOK)
}

func (h *Handler) DismissRecommendation(w http.ResponseWriter, r *http.Request) {
	recID := chi.URLParam(r, "recommendationId")
	body, _ := io.ReadAll(r.Body)
	h.handleAutomationProxy(w, r, http.MethodPost, CanViewRecommendations, "dismiss recommendation denied",
		controltowerreadmodel.AutomationRecommendationPath(recID, "dismiss"), body, http.StatusOK)
}

func (h *Handler) ListPlaybookExecutions(w http.ResponseWriter, r *http.Request) {
	h.handleAutomationRead(w, r, CanManagePlaybookExecution, "view playbook executions denied", func(reqCtx RequestContext) (json.RawMessage, int, error) {
		payload, err := h.service.ListPlaybookExecutions(r.Context(), reqCtx, r)
		return payload, http.StatusOK, err
	})
}

func (h *Handler) GetPlaybookExecution(w http.ResponseWriter, r *http.Request) {
	execID := chi.URLParam(r, "executionId")
	h.handleAutomationProxy(w, r, http.MethodGet, CanManagePlaybookExecution, "view playbook execution denied",
		controltowerreadmodel.PlaybookExecutionPath(execID, ""), nil, http.StatusOK)
}

func (h *Handler) StartPlaybookExecution(w http.ResponseWriter, r *http.Request) {
	execID := chi.URLParam(r, "executionId")
	h.handleAutomationProxy(w, r, http.MethodPost, CanManagePlaybookExecution, "manage playbook execution denied",
		controltowerreadmodel.PlaybookExecutionPath(execID, "start"), nil, http.StatusOK)
}

func (h *Handler) CompletePlaybookExecution(w http.ResponseWriter, r *http.Request) {
	execID := chi.URLParam(r, "executionId")
	h.handleAutomationProxy(w, r, http.MethodPost, CanManagePlaybookExecution, "manage playbook execution denied",
		controltowerreadmodel.PlaybookExecutionPath(execID, "complete"), nil, http.StatusOK)
}

func (h *Handler) CancelPlaybookExecution(w http.ResponseWriter, r *http.Request) {
	execID := chi.URLParam(r, "executionId")
	h.handleAutomationProxy(w, r, http.MethodPost, CanManagePlaybookExecution, "manage playbook execution denied",
		controltowerreadmodel.PlaybookExecutionPath(execID, "cancel"), nil, http.StatusOK)
}

func (h *Handler) StartPlaybookExecutionStep(w http.ResponseWriter, r *http.Request) {
	execID := chi.URLParam(r, "executionId")
	stepID := chi.URLParam(r, "stepId")
	h.handleAutomationProxy(w, r, http.MethodPost, CanManagePlaybookExecution, "manage playbook execution denied",
		controltowerreadmodel.PlaybookExecutionPath(execID, "steps/"+stepID+"/start"), nil, http.StatusOK)
}

func (h *Handler) CompletePlaybookExecutionStep(w http.ResponseWriter, r *http.Request) {
	execID := chi.URLParam(r, "executionId")
	stepID := chi.URLParam(r, "stepId")
	h.handleAutomationProxy(w, r, http.MethodPost, CanManagePlaybookExecution, "manage playbook execution denied",
		controltowerreadmodel.PlaybookExecutionPath(execID, "steps/"+stepID+"/complete"), nil, http.StatusOK)
}

func (h *Handler) SkipPlaybookExecutionStep(w http.ResponseWriter, r *http.Request) {
	execID := chi.URLParam(r, "executionId")
	stepID := chi.URLParam(r, "stepId")
	body, _ := io.ReadAll(r.Body)
	h.handleAutomationProxy(w, r, http.MethodPost, CanManagePlaybookExecution, "manage playbook execution denied",
		controltowerreadmodel.PlaybookExecutionPath(execID, "steps/"+stepID+"/skip"), body, http.StatusOK)
}

func (h *Handler) GetAutomationKPI(w http.ResponseWriter, r *http.Request) {
	h.handleAutomationRead(w, r, CanViewAutomation, "view automation kpi denied", func(reqCtx RequestContext) (json.RawMessage, int, error) {
		payload, err := h.service.GetAutomationKPI(r.Context(), reqCtx)
		return payload, http.StatusOK, err
	})
}

func (h *Handler) handleAutomationRead(w http.ResponseWriter, r *http.Request, allowed func([]string) bool, deniedMsg string, fn func(RequestContext) (json.RawMessage, int, error)) {
	reqCtx, err := buildRequestContext(r, h.authEnabled, h.devTenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if h.authEnabled {
		if err := h.ensureRoleAccess(r, reqCtx, allowed, deniedMsg); err != nil {
			respond.Error(w, err)
			return
		}
	}
	result, status, err := fn(reqCtx)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSONRaw(w, status, result)
}

func (h *Handler) handleAutomationProxy(w http.ResponseWriter, r *http.Request, method string, allowed func([]string) bool, deniedMsg, path string, body []byte, successStatus int) {
	reqCtx, err := buildRequestContext(r, h.authEnabled, h.devTenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if h.authEnabled {
		if err := h.ensureRoleAccess(r, reqCtx, allowed, deniedMsg); err != nil {
			respond.Error(w, err)
			return
		}
	}
	payload, err := h.service.proxyAutomation(r.Context(), reqCtx, method, path, body)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if len(payload) == 0 {
		w.WriteHeader(successStatus)
		return
	}
	respond.JSONRaw(w, successStatus, payload)
}
