package controltower

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/api-gateway/internal/platform/respond"
)

func (h *Handler) ListWorkItems(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	query, err := ParseListQuery(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	reqCtx, err := buildRequestContext(r, h.authEnabled, h.devTenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if h.authEnabled {
		if err := h.ensureRoleAccess(r, reqCtx, CanViewWorkspace, "view operator workspace denied"); err != nil {
			respond.Error(w, err)
			return
		}
	}
	result, err := h.service.ListWorkItems(r.Context(), reqCtx, query)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, result)
	h.log.Info("control_tower_work_items_list_completed",
		slog.String("request_id", requestIDFromRequest(r)),
		slog.String("tenant_id", reqCtx.TenantID),
		slog.Int("items_count", len(result.Items)),
		slog.Int64("duration_ms", time.Since(started).Milliseconds()),
	)
}

func (h *Handler) GetWorkItem(w http.ResponseWriter, r *http.Request) {
	itemType := strings.TrimSpace(chi.URLParam(r, "itemType"))
	itemID := strings.TrimSpace(chi.URLParam(r, "itemId"))
	reqCtx, err := buildRequestContext(r, h.authEnabled, h.devTenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if h.authEnabled {
		if err := h.ensureRoleAccess(r, reqCtx, CanViewWorkspace, "view operator workspace denied"); err != nil {
			respond.Error(w, err)
			return
		}
	}
	result, err := h.service.GetWorkItem(r.Context(), reqCtx, itemType, itemID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, result)
}

func (h *Handler) ClaimWorkItem(w http.ResponseWriter, r *http.Request) {
	h.handleWorkItemMutation(w, r, CanClaimWork, "claim work denied", func(ctx context.Context, reqCtx RequestContext, itemType, itemID string, _ []byte) (ControlTowerWorkItem, error) {
		return h.service.ClaimWorkItem(ctx, reqCtx, itemType, itemID)
	})
}

func (h *Handler) AssignWorkItem(w http.ResponseWriter, r *http.Request) {
	h.handleWorkItemMutation(w, r, CanAssignWork, "assign work denied", func(ctx context.Context, reqCtx RequestContext, itemType, itemID string, raw []byte) (ControlTowerWorkItem, error) {
		return h.service.AssignWorkItem(ctx, reqCtx, itemType, itemID, raw)
	})
}

func (h *Handler) UnassignWorkItem(w http.ResponseWriter, r *http.Request) {
	h.handleWorkItemMutation(w, r, CanAssignWork, "unassign work denied", func(ctx context.Context, reqCtx RequestContext, itemType, itemID string, _ []byte) (ControlTowerWorkItem, error) {
		return h.service.UnassignWorkItem(ctx, reqCtx, itemType, itemID)
	})
}

func (h *Handler) BulkWorkItemsAction(w http.ResponseWriter, r *http.Request) {
	reqCtx, err := buildRequestContext(r, h.authEnabled, h.devTenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if h.authEnabled {
		if err := h.ensureRoleAccess(r, reqCtx, CanBulkManageWork, "bulk work action denied"); err != nil {
			respond.Error(w, err)
			return
		}
	}
	raw, _ := io.ReadAll(r.Body)
	result, err := h.service.BulkWorkItemsAction(r.Context(), reqCtx, raw)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, result)
}

func (h *Handler) GetWorkload(w http.ResponseWriter, r *http.Request) {
	reqCtx, err := buildRequestContext(r, h.authEnabled, h.devTenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if h.authEnabled {
		if err := h.ensureRoleAccess(r, reqCtx, CanViewTeamWorkload, "view team workload denied"); err != nil {
			respond.Error(w, err)
			return
		}
	}
	result, err := h.service.GetWorkload(r.Context(), reqCtx)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, result)
}

func (h *Handler) ListSavedViews(w http.ResponseWriter, r *http.Request) {
	reqCtx, err := buildRequestContext(r, h.authEnabled, h.devTenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if h.authEnabled {
		if err := h.ensureRoleAccess(r, reqCtx, CanViewWorkspace, "view saved views denied"); err != nil {
			respond.Error(w, err)
			return
		}
	}
	result, err := h.service.ListSavedViews(r.Context(), reqCtx, strings.TrimSpace(r.URL.Query().Get("workspaceScope")))
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, result)
}

func (h *Handler) CreateSavedView(w http.ResponseWriter, r *http.Request) {
	reqCtx, err := buildRequestContext(r, h.authEnabled, h.devTenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if h.authEnabled {
		if err := h.ensureRoleAccess(r, reqCtx, CanViewWorkspace, "create saved view denied"); err != nil {
			respond.Error(w, err)
			return
		}
	}
	raw, _ := io.ReadAll(r.Body)
	result, err := h.service.CreateSavedView(r.Context(), reqCtx, raw)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, result)
}

func (h *Handler) UpdateSavedView(w http.ResponseWriter, r *http.Request) {
	viewID := strings.TrimSpace(chi.URLParam(r, "viewId"))
	reqCtx, err := buildRequestContext(r, h.authEnabled, h.devTenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if h.authEnabled {
		if err := h.ensureRoleAccess(r, reqCtx, CanViewWorkspace, "update saved view denied"); err != nil {
			respond.Error(w, err)
			return
		}
	}
	raw, _ := io.ReadAll(r.Body)
	result, err := h.service.UpdateSavedView(r.Context(), reqCtx, viewID, raw)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, result)
}

func (h *Handler) DeleteSavedView(w http.ResponseWriter, r *http.Request) {
	viewID := strings.TrimSpace(chi.URLParam(r, "viewId"))
	reqCtx, err := buildRequestContext(r, h.authEnabled, h.devTenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if h.authEnabled {
		if err := h.ensureRoleAccess(r, reqCtx, CanViewWorkspace, "delete saved view denied"); err != nil {
			respond.Error(w, err)
			return
		}
	}
	if err := h.service.DeleteSavedView(r.Context(), reqCtx, viewID); err != nil {
		respond.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) SetDefaultSavedView(w http.ResponseWriter, r *http.Request) {
	viewID := strings.TrimSpace(chi.URLParam(r, "viewId"))
	reqCtx, err := buildRequestContext(r, h.authEnabled, h.devTenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if h.authEnabled {
		if err := h.ensureRoleAccess(r, reqCtx, CanViewWorkspace, "set default view denied"); err != nil {
			respond.Error(w, err)
			return
		}
	}
	if err := h.service.SetDefaultSavedView(r.Context(), reqCtx, viewID); err != nil {
		respond.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) CreateHandoff(w http.ResponseWriter, r *http.Request) {
	reqCtx, err := buildRequestContext(r, h.authEnabled, h.devTenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if h.authEnabled {
		if err := h.ensureRoleAccess(r, reqCtx, CanCreateHandoff, "create handoff denied"); err != nil {
			respond.Error(w, err)
			return
		}
	}
	raw, _ := io.ReadAll(r.Body)
	result, err := h.service.CreateHandoff(r.Context(), reqCtx, raw)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, result)
}

func (h *Handler) ListHandoffs(w http.ResponseWriter, r *http.Request) {
	reqCtx, err := buildRequestContext(r, h.authEnabled, h.devTenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if h.authEnabled {
		if err := h.ensureRoleAccess(r, reqCtx, CanViewWorkspace, "view handoffs denied"); err != nil {
			respond.Error(w, err)
			return
		}
	}
	result, err := h.service.ListHandoffs(r.Context(), reqCtx, r.URL.Query())
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{"items": result})
}

func (h *Handler) GetHandoff(w http.ResponseWriter, r *http.Request) {
	handoffID := strings.TrimSpace(chi.URLParam(r, "handoffId"))
	reqCtx, err := buildRequestContext(r, h.authEnabled, h.devTenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if h.authEnabled {
		if err := h.ensureRoleAccess(r, reqCtx, CanViewWorkspace, "view handoff denied"); err != nil {
			respond.Error(w, err)
			return
		}
	}
	result, err := h.service.GetHandoff(r.Context(), reqCtx, handoffID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, result)
}

type workItemMutator func(ctx context.Context, reqCtx RequestContext, itemType, itemID string, rawBody []byte) (ControlTowerWorkItem, error)

func (h *Handler) handleWorkItemMutation(
	w http.ResponseWriter,
	r *http.Request,
	check func([]string) bool,
	deniedMessage string,
	mutate workItemMutator,
) {
	itemType := strings.TrimSpace(chi.URLParam(r, "itemType"))
	itemID := strings.TrimSpace(chi.URLParam(r, "itemId"))
	rawBody, _ := io.ReadAll(r.Body)
	reqCtx, err := buildRequestContext(r, h.authEnabled, h.devTenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if h.authEnabled {
		if err := h.ensureRoleAccess(r, reqCtx, check, deniedMessage); err != nil {
			respond.Error(w, err)
			return
		}
	}
	result, err := mutate(r.Context(), reqCtx, itemType, itemID, rawBody)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, result)
}
