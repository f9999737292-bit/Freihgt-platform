package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/freight-platform/identity-service/internal/service"
	"github.com/freight-platform/shared-go/clientip"
	"github.com/freight-platform/shared-go/integrationauth"
	"github.com/freight-platform/shared-go/lowcode"
)

type IntegrationOAuthHandler struct {
	oauthService     *service.IntegrationOAuthService
	clientIPResolver *clientip.Resolver
}

func NewIntegrationOAuthHandler(oauthService *service.IntegrationOAuthService, clientIPResolver *clientip.Resolver) *IntegrationOAuthHandler {
	if clientIPResolver == nil {
		clientIPResolver = clientip.NewResolver(nil)
	}
	return &IntegrationOAuthHandler{oauthService: oauthService, clientIPResolver: clientIPResolver}
}

func (h *IntegrationOAuthHandler) Token(w http.ResponseWriter, r *http.Request) {
	integrationauth.StripUntrustedIntegrationHeaders(r.Header)
	if err := r.ParseForm(); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "malformed form body")
		return
	}
	grantType := strings.TrimSpace(r.FormValue("grant_type"))
	if grantType != "client_credentials" {
		writeOAuthError(w, http.StatusBadRequest, "unsupported_grant_type", "only client_credentials is supported")
		return
	}
	clientID := strings.TrimSpace(r.FormValue("client_id"))
	clientSecret := strings.TrimSpace(r.FormValue("client_secret"))
	if clientID == "" || clientSecret == "" {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "client_id and client_secret are required")
		return
	}
	clientIP, err := h.clientIPResolver.ClientIP(r)
	if err != nil {
		writeOAuthError(w, http.StatusForbidden, "access_denied", "client ip not allowed")
		return
	}
	requestID := strings.TrimSpace(r.Header.Get(lowcode.HeaderRequestID))
	result, err := h.oauthService.IssueClientCredentialsToken(r.Context(), clientID, clientSecret, clientIP, requestID)
	if err != nil {
		writeOAuthAuthError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"access_token": result.AccessToken,
		"token_type":   result.TokenType,
		"expires_in":   result.ExpiresIn,
		"scope":        result.Scope,
	})
}

func writeOAuthAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, integrationauth.ErrInvalidClient):
		writeOAuthError(w, http.StatusUnauthorized, "invalid_client", "client authentication failed")
	case errors.Is(err, integrationauth.ErrCIDRDenied):
		writeOAuthError(w, http.StatusForbidden, "access_denied", "client ip not allowed")
	case errors.Is(err, integrationauth.ErrRateLimited):
		w.Header().Set("Retry-After", strconv.Itoa(int(time.Minute.Seconds())))
		writeOAuthError(w, http.StatusTooManyRequests, "rate_limit_exceeded", "too many token requests")
	default:
		writeOAuthError(w, http.StatusUnauthorized, "invalid_client", "client authentication failed")
	}
}

func writeOAuthError(w http.ResponseWriter, status int, code, description string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":             code,
		"error_description": description,
	})
}
