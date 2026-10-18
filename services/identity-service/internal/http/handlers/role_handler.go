package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/identity-service/internal/domain"
	"github.com/freight-platform/identity-service/internal/http/authority"
	"github.com/freight-platform/identity-service/internal/http/middleware"
	"github.com/freight-platform/identity-service/internal/platform/respond"
	"github.com/freight-platform/identity-service/internal/service"

	"github.com/google/uuid"
)

type RoleHandler struct {
	roleService *service.RoleService
	lookup      authority.Lookup
	auditor     authority.Auditor
}

func NewRoleHandler(roleService *service.RoleService, lookup authority.Lookup, auditor authority.Auditor) *RoleHandler {
	return &RoleHandler{roleService: roleService, lookup: lookup, auditor: auditor}
}

func (h *RoleHandler) List(w http.ResponseWriter, r *http.Request) {
	_, tenantID, ok := h.authorize(w, r, authority.ActionTenantRoleList, r.URL.Query().Get("tenant_id"))
	if !ok {
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
			"id":        role.ID.String(),
			"code":      role.Code,
			"name":      role.Name,
			"scope":     role.Scope,
			"is_system": role.IsSystem,
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
	_, tenantID, ok := h.authorize(w, r, authority.ActionRolePermissionRead, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}

	permissions, err := h.roleService.ListRolePermissions(r.Context(), roleID, tenantID)
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

func (h *RoleHandler) authorize(w http.ResponseWriter, r *http.Request, action authority.Action, supplied string) (authority.Principal, uuid.UUID, bool) {
	principal, err := middleware.PrincipalFromContext(r.Context())
	if err != nil {
		respond.Error(w, err)
		return authority.Principal{}, uuid.Nil, false
	}
	tenantID, err := authority.ResolveTenant(r.Context(), principal, supplied, action, h.lookup, h.auditor)
	if err != nil {
		respond.Error(w, err)
		return authority.Principal{}, uuid.Nil, false
	}
	if err := authority.Allow(principal, action, uuid.Nil); err != nil {
		respond.Error(w, err)
		return authority.Principal{}, uuid.Nil, false
	}
	return principal, tenantID, true
}
