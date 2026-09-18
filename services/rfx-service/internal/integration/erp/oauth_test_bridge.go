//go:build integration

package erp

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/freight-platform/shared-go/clientip"
	"github.com/freight-platform/shared-go/integrationauth"
)

type oauthTokenHTTPHandler struct {
	verifier      *integrationauth.Verifier
	jwtService    *integrationauth.JWTService
	auditor       *integrationauth.AuditRecorder
	limiter       *integrationauth.PrincipalRateLimiter
	failedLimiter *integrationauth.PrincipalRateLimiter
	resolver      *clientip.Resolver
}

func newOAuthTokenHTTPHandler(
	verifier *integrationauth.Verifier,
	jwtService *integrationauth.JWTService,
	auditor *integrationauth.AuditRecorder,
	limiter, failedLimiter *integrationauth.PrincipalRateLimiter,
) http.Handler {
	return &oauthTokenHTTPHandler{
		verifier:      verifier,
		jwtService:    jwtService,
		auditor:       auditor,
		limiter:       limiter,
		failedLimiter: failedLimiter,
		resolver:      clientip.NewResolver(nil),
	}
}

func (h *oauthTokenHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	integrationauth.StripUntrustedIntegrationHeaders(r.Header)
	if err := r.ParseForm(); err != nil {
		writeOAuthTestError(w, http.StatusBadRequest, "invalid_request", "malformed form body")
		return
	}
	if strings.TrimSpace(r.FormValue("grant_type")) != "client_credentials" {
		writeOAuthTestError(w, http.StatusBadRequest, "unsupported_grant_type", "only client_credentials is supported")
		return
	}
	clientID := strings.TrimSpace(r.FormValue("client_id"))
	clientSecret := strings.TrimSpace(r.FormValue("client_secret"))
	if clientID == "" || clientSecret == "" {
		writeOAuthTestError(w, http.StatusBadRequest, "invalid_request", "client_id and client_secret are required")
		return
	}
	clientIP, err := h.resolver.ClientIP(r)
	if err != nil {
		writeOAuthTestError(w, http.StatusForbidden, "access_denied", "client ip not allowed")
		return
	}
	failKey := integrationauth.OAuthFailedAttemptKey(clientIP, clientID)
	if h.failedLimiter != nil {
		if limited, _ := h.failedLimiter.IsLimited(failKey); limited {
			writeOAuthTestError(w, http.StatusTooManyRequests, "rate_limit_exceeded", "too many token requests")
			return
		}
	}
	authCtx, err := h.verifier.AuthenticateOAuthClientCredentials(r.Context(), clientID, clientSecret, clientIP)
	if err != nil {
		if h.failedLimiter != nil {
			h.failedLimiter.RecordAttempt(failKey)
		}
		writeOAuthTestError(w, http.StatusUnauthorized, "invalid_client", "client authentication failed")
		return
	}
	key := h.limiter.Key(authCtx.TenantID.String(), authCtx.PrincipalID.String(), "oauth_token")
	if allowed, _ := h.limiter.Allow(key); !allowed {
		writeOAuthTestError(w, http.StatusTooManyRequests, "rate_limit_exceeded", "too many token requests")
		return
	}
	token, expiresIn, err := h.jwtService.CreateIntegrationToken(authCtx)
	if err != nil {
		writeOAuthTestError(w, http.StatusInternalServerError, "server_error", "token issue failed")
		return
	}
	_ = h.auditor.RecordAuthSuccess(r.Context(), authCtx, "")
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"access_token": token,
		"token_type":   "Bearer",
		"expires_in":   expiresIn,
	})
}

func writeOAuthTestError(w http.ResponseWriter, status int, code, description string) {
	if status == http.StatusTooManyRequests {
		w.Header().Set("Retry-After", strconv.Itoa(int(time.Minute.Seconds())))
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":             code,
		"error_description": description,
	})
}
