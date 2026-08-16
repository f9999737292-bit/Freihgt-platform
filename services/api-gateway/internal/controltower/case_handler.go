package controltower

import (
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/api-gateway/internal/controltowerreadmodel"
	"github.com/freight-platform/api-gateway/internal/platform/respond"
)

func (h *Handler) ListCases(w http.ResponseWriter, r *http.Request) {
	reqCtx, err := buildRequestContext(r, h.authEnabled, h.devTenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if h.authEnabled {
		if err := h.ensureRoleAccess(r, reqCtx, CanViewCase, "view cases denied"); err != nil {
			respond.Error(w, err)
			return
		}
	}
	result, err := h.service.ListCases(r.Context(), reqCtx, r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSONRaw(w, http.StatusOK, result)
}

func (h *Handler) GetCase(w http.ResponseWriter, r *http.Request) {
	caseID := chi.URLParam(r, "caseId")
	reqCtx, err := buildRequestContext(r, h.authEnabled, h.devTenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if h.authEnabled {
		if err := h.ensureRoleAccess(r, reqCtx, CanViewCase, "view case denied"); err != nil {
			respond.Error(w, err)
			return
		}
	}
	result, err := h.service.GetCase(r.Context(), reqCtx, caseID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSONRaw(w, http.StatusOK, result)
}

func (h *Handler) CreateCase(w http.ResponseWriter, r *http.Request) {
	h.handleCaseMutation(w, r, CanCreateCase, "create case denied", http.MethodPost, "", http.StatusCreated)
}

func (h *Handler) UpdateCase(w http.ResponseWriter, r *http.Request) {
	h.handleCaseMutation(w, r, CanManageCase, "update case denied", http.MethodPatch, "", http.StatusOK)
}

func (h *Handler) ClaimCase(w http.ResponseWriter, r *http.Request) {
	h.handleCaseMutation(w, r, CanAssignCase, "claim case denied", http.MethodPost, "claim", http.StatusOK)
}

func (h *Handler) AssignCase(w http.ResponseWriter, r *http.Request) {
	h.handleCaseMutation(w, r, CanAssignCase, "assign case denied", http.MethodPost, "assign", http.StatusOK)
}

func (h *Handler) UnassignCase(w http.ResponseWriter, r *http.Request) {
	h.handleCaseMutation(w, r, CanAssignCase, "unassign case denied", http.MethodPost, "unassign", http.StatusOK)
}

func (h *Handler) AddCaseLink(w http.ResponseWriter, r *http.Request) {
	h.handleCaseMutation(w, r, CanManageCase, "add case link denied", http.MethodPost, "links", http.StatusCreated)
}

func (h *Handler) RemoveCaseLink(w http.ResponseWriter, r *http.Request) {
	caseID := chi.URLParam(r, "caseId")
	linkID := chi.URLParam(r, "linkId")
	reqCtx, err := buildRequestContext(r, h.authEnabled, h.devTenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if h.authEnabled {
		if err := h.ensureRoleAccess(r, reqCtx, CanManageCase, "remove case link denied"); err != nil {
			respond.Error(w, err)
			return
		}
	}
	path := "/internal/v1/control-tower/cases/" + caseID + "/links/" + linkID
	depErr := h.service.readModel.ProxyCaseNoContent(r.Context(), http.MethodDelete, reqCtx.TenantID, reqCtx.UserID, reqCtx.RequestID, path, nil)
	if depErr != nil {
		respond.Error(w, mapWorkspaceDependencyError(depErr))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) CreateCaseNote(w http.ResponseWriter, r *http.Request) {
	h.handleCaseMutation(w, r, CanAddCaseNote, "add case note denied", http.MethodPost, "notes", http.StatusCreated)
}

func (h *Handler) UpdateCaseNote(w http.ResponseWriter, r *http.Request) {
	caseID := chi.URLParam(r, "caseId")
	noteID := chi.URLParam(r, "noteId")
	h.handleCaseMutationWithPath(w, r, CanAddCaseNote, "update case note denied", http.MethodPatch, "notes/"+noteID, http.StatusOK, caseID)
}

func (h *Handler) CreateCaseAction(w http.ResponseWriter, r *http.Request) {
	h.handleCaseMutation(w, r, CanManageCaseActions, "create case action denied", http.MethodPost, "actions", http.StatusCreated)
}

func (h *Handler) UpdateCaseAction(w http.ResponseWriter, r *http.Request) {
	caseID := chi.URLParam(r, "caseId")
	actionID := chi.URLParam(r, "actionId")
	h.handleCaseMutationWithPath(w, r, CanManageCaseActions, "update case action denied", http.MethodPatch, "actions/"+actionID, http.StatusOK, caseID)
}

func (h *Handler) CompleteCaseAction(w http.ResponseWriter, r *http.Request) {
	caseID := chi.URLParam(r, "caseId")
	actionID := chi.URLParam(r, "actionId")
	h.handleCaseMutationWithPath(w, r, CanManageCaseActions, "complete case action denied", http.MethodPost, "actions/"+actionID+"/complete", http.StatusOK, caseID)
}

func (h *Handler) CreateCaseDecision(w http.ResponseWriter, r *http.Request) {
	h.handleCaseMutation(w, r, CanManageCase, "record decision denied", http.MethodPost, "decisions", http.StatusCreated)
}

func (h *Handler) ResolveCase(w http.ResponseWriter, r *http.Request) {
	h.handleCaseMutation(w, r, CanResolveCase, "resolve case denied", http.MethodPost, "resolve", http.StatusOK)
}

func (h *Handler) CloseCase(w http.ResponseWriter, r *http.Request) {
	h.handleCaseMutation(w, r, CanResolveCase, "close case denied", http.MethodPost, "close", http.StatusOK)
}

func (h *Handler) ReopenCase(w http.ResponseWriter, r *http.Request) {
	h.handleCaseMutation(w, r, CanResolveCase, "reopen case denied", http.MethodPost, "reopen", http.StatusOK)
}

func (h *Handler) GetCaseTimeline(w http.ResponseWriter, r *http.Request) {
	caseID := chi.URLParam(r, "caseId")
	reqCtx, err := buildRequestContext(r, h.authEnabled, h.devTenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if h.authEnabled {
		if err := h.ensureRoleAccess(r, reqCtx, CanViewCase, "view case timeline denied"); err != nil {
			respond.Error(w, err)
			return
		}
	}
	path := "/internal/v1/control-tower/cases/" + caseID + "/timeline?" + r.URL.RawQuery
	result, depErr := h.service.readModel.ProxyCaseJSON(r.Context(), http.MethodGet, reqCtx.TenantID, reqCtx.UserID, reqCtx.RequestID, path, nil)
	if depErr != nil {
		respond.Error(w, mapWorkspaceDependencyError(depErr))
		return
	}
	respond.JSONRaw(w, http.StatusOK, result)
}

func (h *Handler) GetCaseKPI(w http.ResponseWriter, r *http.Request) {
	reqCtx, err := buildRequestContext(r, h.authEnabled, h.devTenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if h.authEnabled {
		if err := h.ensureRoleAccess(r, reqCtx, CanViewCase, "view case kpi denied"); err != nil {
			respond.Error(w, err)
			return
		}
	}
	result, err := h.service.GetCaseKPI(r.Context(), reqCtx)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSONRaw(w, http.StatusOK, result)
}

func (h *Handler) FindCaseDuplicates(w http.ResponseWriter, r *http.Request) {
	reqCtx, err := buildRequestContext(r, h.authEnabled, h.devTenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if h.authEnabled {
		if err := h.ensureRoleAccess(r, reqCtx, CanViewCase, "view case duplicates denied"); err != nil {
			respond.Error(w, err)
			return
		}
	}
	path := controltowerreadmodel.CaseDuplicatesPath + "?" + r.URL.RawQuery
	result, depErr := h.service.readModel.ProxyCaseJSON(r.Context(), http.MethodGet, reqCtx.TenantID, reqCtx.UserID, reqCtx.RequestID, path, nil)
	if depErr != nil {
		respond.Error(w, mapWorkspaceDependencyError(depErr))
		return
	}
	respond.JSONRaw(w, http.StatusOK, result)
}

func (h *Handler) AddCaseParticipant(w http.ResponseWriter, r *http.Request) {
	h.handleCaseMutation(w, r, CanManageCaseParticipants, "add case participant denied", http.MethodPost, "participants", http.StatusCreated)
}

func (h *Handler) UpdateCaseParticipant(w http.ResponseWriter, r *http.Request) {
	caseID := chi.URLParam(r, "caseId")
	userID := chi.URLParam(r, "userId")
	h.handleCaseMutationWithPath(w, r, CanManageCaseParticipants, "update case participant denied", http.MethodPatch, "participants/"+userID, http.StatusNoContent, caseID)
}

func (h *Handler) RemoveCaseParticipant(w http.ResponseWriter, r *http.Request) {
	caseID := chi.URLParam(r, "caseId")
	userID := chi.URLParam(r, "userId")
	reqCtx, err := buildRequestContext(r, h.authEnabled, h.devTenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if h.authEnabled {
		if err := h.ensureRoleAccess(r, reqCtx, CanManageCaseParticipants, "remove case participant denied"); err != nil {
			respond.Error(w, err)
			return
		}
	}
	path := "/internal/v1/control-tower/cases/" + caseID + "/participants/" + userID
	depErr := h.service.readModel.ProxyCaseNoContent(r.Context(), http.MethodDelete, reqCtx.TenantID, reqCtx.UserID, reqCtx.RequestID, path, nil)
	if depErr != nil {
		respond.Error(w, mapWorkspaceDependencyError(depErr))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleCaseMutation(w http.ResponseWriter, r *http.Request, access func([]string) bool, denied, method, suffix string, status int) {
	h.handleCaseMutationWithPath(w, r, access, denied, method, suffix, status, chi.URLParam(r, "caseId"))
}

func (h *Handler) handleCaseMutationWithPath(w http.ResponseWriter, r *http.Request, access func([]string) bool, denied, method, suffix string, status int, caseID string) {
	reqCtx, err := buildRequestContext(r, h.authEnabled, h.devTenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if h.authEnabled {
		if err := h.ensureRoleAccess(r, reqCtx, access, denied); err != nil {
			respond.Error(w, err)
			return
		}
	}
	raw, _ := io.ReadAll(r.Body)
	var result []byte
	if caseID == "" && method == http.MethodPost && suffix == "" {
		payload, err := h.service.ProxyCaseCreate(r.Context(), reqCtx, raw)
		if err != nil {
			respond.Error(w, err)
			return
		}
		result = payload
	} else {
		payload, err := h.service.ProxyCaseMutation(r.Context(), reqCtx, method, caseID, suffix, raw)
		if err != nil {
			respond.Error(w, err)
			return
		}
		result = payload
	}
	if len(result) == 0 {
		w.WriteHeader(status)
		return
	}
	respond.JSONRaw(w, status, result)
}
