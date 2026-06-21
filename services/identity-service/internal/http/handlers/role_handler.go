package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/identity-service/internal/domain"
	"github.com/freight-platform/identity-service/internal/platform/respond"
	"github.com/freight-platform/identity-service/internal/service"
)

type RoleHandler struct {
	roleService *service.RoleService
}

func NewRoleHandler(roleService *service.RoleService) *RoleHandler {
	return &RoleHandler{roleService: roleService}
}

func (h *RoleHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, err := domain.ParseUUID(r.URL.Query().Get("tenant_id"), "tenant_id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	roles, err := h.roleService.ListRoles(r.Context(), tenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}

	items := make([]map[string]any, 0, len(roles))
	for _, role := range roles {
		item := map[string]any{
			"id":         role.ID.String(),
			"code":       role.Code,
			"name":       role.Name,
			"scope":      role.Scope,
			"is_system":  role.IsSystem,
		}
		if role.TenantID != nil {
			item["tenant_id"] = role.TenantID.String()
		}
		if role.Description != nil {
			item["description"] = *role.Description
		}
		items = append(items, item)
	}

	respond.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *RoleHandler) ListPermissions(w http.ResponseWriter, r *http.Request) {
	roleID, err := domain.ParseUUID(chi.URLParam(r, "role_id"), "role_id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	permissions, err := h.roleService.ListRolePermissions(r.Context(), roleID)
	if err != nil {
		respond.Error(w, err)
		return
	}

	items := make([]map[string]any, 0, len(permissions))
	for _, p := range permissions {
		item := map[string]any{
			"id":       p.ID.String(),
			"code":     p.Code,
			"resource": p.Resource,
			"action":   p.Action,
		}
		if p.Description != nil {
			item["description"] = *p.Description
		}
		items = append(items, item)
	}

	respond.JSON(w, http.StatusOK, map[string]any{"items": items})
}
