package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/freight-platform/identity-service/internal/domain"
	"github.com/freight-platform/identity-service/internal/http/middleware"
	apperrors "github.com/freight-platform/identity-service/internal/platform/errors"
	"github.com/freight-platform/identity-service/internal/platform/respond"
	"github.com/freight-platform/identity-service/internal/service"
)

type AuthHandler struct {
	authService *service.AuthService
}

func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
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
		User:        toUserPublicResponse(result.User),
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

	respond.JSON(w, http.StatusOK, toUserMeResponse(user))
}
