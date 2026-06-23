package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/freight-platform/identity-service/internal/domain"
	"github.com/freight-platform/identity-service/internal/http/middleware"
	apperrors "github.com/freight-platform/identity-service/internal/platform/errors"
	"github.com/freight-platform/identity-service/internal/platform/respond"
	"github.com/freight-platform/identity-service/internal/service"
)

type AuthHandler struct {
	authService *service.AuthService
	roleService *service.RoleService
}

func NewAuthHandler(authService *service.AuthService, roleService *service.RoleService) *AuthHandler {
	return &AuthHandler{authService: authService, roleService: roleService}
}

type loginRequest struct {
	TenantID string `json:"tenant_id"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	AccessToken string             `json:"access_token"`
	TokenType   string             `json:"token_type"`
	ExpiresIn   int64              `json:"expires_in"`
	User        userPublicResponse `json:"user"`
}

func (h *AuthHandler) Health(w http.ResponseWriter, _ *http.Request) {
	respond.JSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "identity-service",
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}

	tenantID, err := domain.ParseUUID(req.TenantID, "tenant_id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	result, err := h.authService.Login(r.Context(), domain.LoginInput{
		TenantID: tenantID,
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, loginResponse{
		AccessToken: result.AccessToken,
		TokenType:   result.TokenType,
		ExpiresIn:   result.ExpiresIn,
		User:        toUserPublicResponse(result.User, h.roleCodesForUser(r.Context(), result.User.ID, result.User.TenantID)),
	})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID, err := middleware.UserIDFromContext(r.Context())
	if err != nil {
		respond.Error(w, err)
		return
	}

	user, err := h.authService.GetCurrentUser(r.Context(), userID)
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, toUserMeResponse(user, h.roleCodesForUser(r.Context(), userID, user.TenantID)))
}

func (h *AuthHandler) roleCodesForUser(ctx context.Context, userID, tenantID uuid.UUID) []string {
	if h.roleService == nil {
		return nil
	}
	items, err := h.roleService.ListUserRoles(ctx, userID, tenantID)
	if err != nil {
		return nil
	}
	codes := make([]string, 0, len(items))
	for _, item := range items {
		if item.Code != "" {
			codes = append(codes, item.Code)
		}
	}
	return codes
}
