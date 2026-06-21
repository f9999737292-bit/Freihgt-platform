package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/company-service/internal/domain"
	apperrors "github.com/freight-platform/company-service/internal/platform/errors"
	"github.com/freight-platform/company-service/internal/platform/respond"
	"github.com/freight-platform/company-service/internal/service"
)

type MembershipHandler struct {
	service *service.MembershipService
}

func NewMembershipHandler(svc *service.MembershipService) *MembershipHandler {
	return &MembershipHandler{service: svc}
}

type addMemberRequest struct {
	TenantID string  `json:"tenant_id"`
	UserID   string  `json:"user_id"`
	Position *string `json:"position"`
	RoleID   *string `json:"role_id"`
}

type updateMemberRequest struct {
	Position *string `json:"position"`
	Status   *string `json:"status"`
}

type membershipResponse struct {
	ID        string  `json:"id"`
	TenantID  string  `json:"tenant_id"`
	CompanyID string  `json:"company_id"`
	UserID    string  `json:"user_id"`
	Position  *string `json:"position,omitempty"`
	Status    string  `json:"status"`
	CreatedAt string  `json:"created_at"`
}

func (h *MembershipHandler) AddMember(w http.ResponseWriter, r *http.Request) {
	companyID, err := domain.ParseUUID(chi.URLParam(r, "company_id"), "company_id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	var req addMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}

	tenantID, err := domain.ParseUUID(req.TenantID, "tenant_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	userID, err := domain.ParseUUID(req.UserID, "user_id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	var roleID *uuid.UUID
	if req.RoleID != nil && strings.TrimSpace(*req.RoleID) != "" {
		parsed, err := domain.ParseUUID(*req.RoleID, "role_id")
		if err != nil {
			respond.Error(w, err)
			return
		}
		roleID = &parsed
	}

	membership, err := h.service.AddMember(r.Context(), domain.CreateMembershipInput{
		TenantID:  tenantID,
		CompanyID: companyID,
		UserID:    userID,
		Position:  req.Position,
		RoleID:    roleID,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusCreated, toMembershipResponse(membership))
}

func (h *MembershipHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	companyID, err := domain.ParseUUID(chi.URLParam(r, "company_id"), "company_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	tenantID, err := domain.ParseUUID(r.URL.Query().Get("tenant_id"), "tenant_id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			respond.Error(w, apperrors.Validation("invalid limit", map[string]any{"field": "limit"}))
			return
		}
		limit = parsed
	}
	offset := 0
	if raw := r.URL.Query().Get("offset"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			respond.Error(w, apperrors.Validation("invalid offset", map[string]any{"field": "offset"}))
			return
		}
		offset = parsed
	}

	filter := domain.ListCompanyMembersFilter{
		TenantID:  tenantID,
		CompanyID: companyID,
		Limit:     limit,
		Offset:    offset,
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("status")); raw != "" {
		filter.Status = &raw
	}

	members, _, err := h.service.ListMembers(r.Context(), filter)
	if err != nil {
		respond.Error(w, err)
		return
	}

	items := make([]map[string]any, 0, len(members))
	for _, member := range members {
		roles := make([]map[string]any, 0, len(member.Roles))
		for _, role := range member.Roles {
			roles = append(roles, map[string]any{
				"role_id": role.RoleID.String(),
				"code":    role.Code,
				"name":    role.Name,
			})
		}
		items = append(items, map[string]any{
			"membership_id": member.MembershipID.String(),
			"user_id":       member.UserID.String(),
			"email":         member.Email,
			"full_name":     member.FullName,
			"phone":         member.Phone,
			"position":      member.Position,
			"status":        member.Status,
			"roles":         roles,
		})
	}

	respond.JSON(w, http.StatusOK, map[string]any{
		"items":  items,
		"limit":  limit,
		"offset": offset,
	})
}

func (h *MembershipHandler) UpdateMember(w http.ResponseWriter, r *http.Request) {
	companyID, err := domain.ParseUUID(chi.URLParam(r, "company_id"), "company_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	membershipID, err := domain.ParseUUID(chi.URLParam(r, "membership_id"), "membership_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req updateMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}

	membership, err := h.service.UpdateMember(r.Context(), membershipID, companyID, domain.UpdateMembershipInput{
		Position: req.Position,
		Status:   req.Status,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, toMembershipResponse(membership))
}

func (h *MembershipHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	companyID, err := domain.ParseUUID(chi.URLParam(r, "company_id"), "company_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	membershipID, err := domain.ParseUUID(chi.URLParam(r, "membership_id"), "membership_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	if err := h.service.RemoveMember(r.Context(), membershipID, companyID); err != nil {
		respond.Error(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func toMembershipResponse(m *domain.Membership) membershipResponse {
	return membershipResponse{
		ID:        m.ID.String(),
		TenantID:  m.TenantID.String(),
		CompanyID: m.CompanyID.String(),
		UserID:    m.UserID.String(),
		Position:  m.Position,
		Status:    m.Status,
		CreatedAt: m.CreatedAt,
	}
}
