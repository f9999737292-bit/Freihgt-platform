package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/identity-service/internal/domain"
	apperrors "github.com/freight-platform/identity-service/internal/platform/errors"
	"github.com/freight-platform/identity-service/internal/platform/respond"
	"github.com/freight-platform/identity-service/internal/service"
)

type MembershipHandler struct {
	service *service.MembershipService
}

func NewMembershipHandler(svc *service.MembershipService) *MembershipHandler {
	return &MembershipHandler{service: svc}
}

type assignCompanyRoleRequest struct {
	TenantID string `json:"tenant_id"`
	RoleID   string `json:"role_id"`
}

func (h *MembershipHandler) ListUserCompanies(w http.ResponseWriter, r *http.Request) {
	userID, err := domain.ParseUUID(chi.URLParam(r, "user_id"), "user_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	tenantID, err := domain.ParseUUID(r.URL.Query().Get("tenant_id"), "tenant_id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	filter := domain.ListUserCompaniesFilter{
		TenantID: tenantID,
		UserID:   userID,
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("status")); raw != "" {
		filter.Status = &raw
	}

	companies, err := h.service.GetUserCompanies(r.Context(), filter)
	if err != nil {
		respond.Error(w, err)
		return
	}

	items := make([]map[string]any, 0, len(companies))
	for _, company := range companies {
		roles := make([]map[string]any, 0, len(company.Roles))
		for _, role := range company.Roles {
			roles = append(roles, map[string]any{
				"role_id": role.RoleID.String(),
				"code":    role.Code,
				"name":    role.Name,
			})
		}
		items = append(items, map[string]any{
			"membership_id":     company.MembershipID.String(),
			"company_id":        company.CompanyID.String(),
			"legal_name":        company.LegalName,
			"short_name":        company.ShortName,
			"company_type":      company.CompanyType,
			"position":          company.Position,
			"membership_status": company.MembershipStatus,
			"roles":             roles,
		})
	}

	respond.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *MembershipHandler) AssignCompanyRole(w http.ResponseWriter, r *http.Request) {
	userID, err := domain.ParseUUID(chi.URLParam(r, "user_id"), "user_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	companyID, err := domain.ParseUUID(chi.URLParam(r, "company_id"), "company_id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	var req assignCompanyRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}

	tenantID, err := domain.ParseUUID(req.TenantID, "tenant_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	roleID, err := domain.ParseUUID(req.RoleID, "role_id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	if err := h.service.AddCompanyRoleToUser(r.Context(), domain.AssignCompanyRoleInput{
		TenantID:  tenantID,
		UserID:    userID,
		CompanyID: companyID,
		RoleID:    roleID,
	}); err != nil {
		respond.Error(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *MembershipHandler) RemoveCompanyRole(w http.ResponseWriter, r *http.Request) {
	userID, err := domain.ParseUUID(chi.URLParam(r, "user_id"), "user_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	companyID, err := domain.ParseUUID(chi.URLParam(r, "company_id"), "company_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	roleID, err := domain.ParseUUID(chi.URLParam(r, "role_id"), "role_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	tenantID, err := domain.ParseUUID(r.URL.Query().Get("tenant_id"), "tenant_id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	if err := h.service.RemoveCompanyRoleFromUser(r.Context(), tenantID, userID, companyID, roleID); err != nil {
		respond.Error(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
