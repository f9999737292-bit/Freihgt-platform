package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/identity-service/internal/domain"
	apperrors "github.com/freight-platform/identity-service/internal/platform/errors"
	"github.com/freight-platform/identity-service/internal/platform/respond"
	"github.com/freight-platform/identity-service/internal/service"
)

type UserHandler struct {
	userService *service.UserService
	roleService *service.RoleService
}

func NewUserHandler(userService *service.UserService, roleService *service.RoleService) *UserHandler {
	return &UserHandler{userService: userService, roleService: roleService}
}

type createUserRequest struct {
	TenantID        string  `json:"tenant_id"`
	Email           string  `json:"email"`
	Phone           *string `json:"phone"`
	Password        string  `json:"password"`
	FullName        string  `json:"full_name"`
	PreferredLocale string  `json:"preferred_locale"`
}

type updateUserRequest struct {
	Phone           *string `json:"phone"`
	FullName        *string `json:"full_name"`
	PreferredLocale *string `json:"preferred_locale"`
	Status          *string `json:"status"`
}

type userResponse struct {
	ID              string  `json:"id"`
	TenantID        string  `json:"tenant_id"`
	Email           string  `json:"email"`
	Phone           *string `json:"phone,omitempty"`
	FullName        string  `json:"full_name"`
	PreferredLocale string  `json:"preferred_locale"`
	Status          string  `json:"status"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at,omitempty"`
	Version         int     `json:"version,omitempty"`
}

type userPublicResponse struct {
	ID              string   `json:"id"`
	TenantID        string   `json:"tenant_id"`
	Email           string   `json:"email"`
	FullName        string   `json:"full_name"`
	PreferredLocale string   `json:"preferred_locale"`
	Roles           []string `json:"roles,omitempty"`
}

type userMeResponse struct {
	ID              string   `json:"id"`
	TenantID        string   `json:"tenant_id"`
	Email           string   `json:"email"`
	FullName        string   `json:"full_name"`
	PreferredLocale string   `json:"preferred_locale"`
	Status          string   `json:"status"`
	Roles           []string `json:"roles,omitempty"`
}

type listUsersResponse struct {
	Items  []userResponse `json:"items"`
	Total  int            `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

type assignRoleRequest struct {
	TenantID  string  `json:"tenant_id"`
	CompanyID *string `json:"company_id"`
	RoleID    string  `json:"role_id"`
}

func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}

	tenantID, err := domain.ParseUUID(req.TenantID, "tenant_id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	user, err := h.userService.Create(r.Context(), domain.CreateUserInput{
		TenantID:        tenantID,
		Email:           req.Email,
		Phone:           req.Phone,
		Password:        req.Password,
		FullName:        req.FullName,
		PreferredLocale: req.PreferredLocale,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusCreated, toUserResponse(user))
}

func (h *UserHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	user, err := h.userService.GetByID(r.Context(), id)
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, toUserResponse(user))
}

func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
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

	filter := domain.ListUsersFilter{TenantID: tenantID, Limit: limit, Offset: offset}
	if raw := strings.TrimSpace(r.URL.Query().Get("status")); raw != "" {
		filter.Status = &raw
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("search")); raw != "" {
		filter.Search = &raw
	}

	users, total, err := h.userService.List(r.Context(), filter)
	if err != nil {
		respond.Error(w, err)
		return
	}

	items := make([]userResponse, 0, len(users))
	for i := range users {
		items = append(items, toUserResponse(&users[i]))
	}

	respond.JSON(w, http.StatusOK, listUsersResponse{Items: items, Total: total, Limit: limit, Offset: offset})
}

func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	var req updateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}

	user, err := h.userService.Update(r.Context(), id, domain.UpdateUserInput{
		Phone:           req.Phone,
		FullName:        req.FullName,
		PreferredLocale: req.PreferredLocale,
		Status:          req.Status,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, toUserResponse(user))
}

func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	if err := h.userService.Delete(r.Context(), id); err != nil {
		respond.Error(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *UserHandler) AssignRole(w http.ResponseWriter, r *http.Request) {
	userID, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	var req assignRoleRequest
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

	var companyID *uuid.UUID
	if req.CompanyID != nil && strings.TrimSpace(*req.CompanyID) != "" {
		parsed, err := domain.ParseUUID(*req.CompanyID, "company_id")
		if err != nil {
			respond.Error(w, err)
			return
		}
		companyID = &parsed
	}

	if err := h.roleService.AssignRole(r.Context(), domain.AssignRoleInput{
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

func (h *UserHandler) ListRoles(w http.ResponseWriter, r *http.Request) {
	userID, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	tenantID, err := domain.ParseUUID(r.URL.Query().Get("tenant_id"), "tenant_id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	items, err := h.roleService.ListUserRoles(r.Context(), userID, tenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, map[string]any{"items": toUserRoleItems(items)})
}

func toUserResponse(user *domain.User) userResponse {
	return userResponse{
		ID:              user.ID.String(),
		TenantID:        user.TenantID.String(),
		Email:           user.Email,
		Phone:           user.Phone,
		FullName:        user.FullName,
		PreferredLocale: user.PreferredLocale,
		Status:          user.Status,
		CreatedAt:       user.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt:       user.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		Version:         user.Version,
	}
}

func toUserPublicResponse(user *domain.User, roles []string) userPublicResponse {
	return userPublicResponse{
		ID:              user.ID.String(),
		TenantID:        user.TenantID.String(),
		Email:           user.Email,
		FullName:        user.FullName,
		PreferredLocale: user.PreferredLocale,
		Roles:           roles,
	}
}

func toUserMeResponse(user *domain.User, roles []string) userMeResponse {
	return userMeResponse{
		ID:              user.ID.String(),
		TenantID:        user.TenantID.String(),
		Email:           user.Email,
		FullName:        user.FullName,
		PreferredLocale: user.PreferredLocale,
		Status:          user.Status,
		Roles:           roles,
	}
}

func toUserRoleItems(items []domain.UserRoleAssignment) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		entry := map[string]any{
			"role_id": item.RoleID.String(),
			"code":    item.Code,
			"name":    item.Name,
		}
		if item.CompanyID != nil {
			entry["company_id"] = item.CompanyID.String()
		}
		result = append(result, entry)
	}
	return result
}
