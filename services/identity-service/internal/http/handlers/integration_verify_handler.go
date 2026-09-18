package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/freight-platform/shared-go/clientip"
	"github.com/freight-platform/shared-go/integrationauth"
	"github.com/freight-platform/shared-go/internalauth"
	"github.com/freight-platform/shared-go/lowcode"
)

type IntegrationVerifyHandler struct {
	verifier         *integrationauth.Verifier
	auditor          *integrationauth.AuditRecorder
	internalAuth     internalauth.Config
	clientIPResolver *clientip.Resolver
}

func NewIntegrationVerifyHandler(verifier *integrationauth.Verifier, auditor *integrationauth.AuditRecorder, internalAuth internalauth.Config, clientIPResolver *clientip.Resolver) *IntegrationVerifyHandler {
	if clientIPResolver == nil {
		clientIPResolver = clientip.NewResolver(nil)
	}
	return &IntegrationVerifyHandler{verifier: verifier, auditor: auditor, internalAuth: internalAuth, clientIPResolver: clientIPResolver}
}

type verifyBearerRequest struct {
	Bearer string `json:"bearer"`
}

type verifyBearerResponse struct {
	PrincipalID string   `json:"principal_id"`
	TenantID    string   `json:"tenant_id"`
	CompanyID   string   `json:"company_id"`
	AuthScheme  string   `json:"auth_scheme"`
	Scopes      []string `json:"scopes"`
}

func (h *IntegrationVerifyHandler) VerifyBearer(w http.ResponseWriter, r *http.Request) {
	if !h.internalAuth.Authorize(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var req verifyBearerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	bearer := strings.TrimSpace(req.Bearer)
	if bearer == "" {
		http.Error(w, "bearer required", http.StatusBadRequest)
		return
	}
	requestID := strings.TrimSpace(r.Header.Get(lowcode.HeaderRequestID))
	clientIP, ipErr := h.clientIPResolver.ClientIP(r)
	if ipErr != nil {
		http.Error(w, "access_denied", http.StatusForbidden)
		return
	}
	authCtx, err := h.verifier.AuthenticateAPIKeyBearer(r.Context(), bearer, clientIP)
	if err != nil {
		_ = h.auditor.RecordAuthFailure(r.Context(), nil, nil, mapVerifyError(err), integrationauth.AuthSchemeAPIKey, requestID, clientIP)
		writeVerifyError(w, err)
		return
	}
	_ = h.auditor.RecordAuthSuccess(r.Context(), authCtx, requestID)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(verifyBearerResponse{
		PrincipalID: authCtx.PrincipalID.String(),
		TenantID:    authCtx.TenantID.String(),
		CompanyID:   authCtx.CompanyID.String(),
		AuthScheme:  authCtx.AuthScheme,
		Scopes:      authCtx.Scopes,
	})
}

func mapVerifyError(err error) string {
	switch {
	case errors.Is(err, integrationauth.ErrAuthSchemeDenied):
		return "auth_scheme_denied"
	case errors.Is(err, integrationauth.ErrCIDRDenied):
		return "cidr_denied"
	default:
		return "invalid_credentials"
	}
}

func writeVerifyError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, integrationauth.ErrAuthSchemeDenied):
		http.Error(w, "auth_scheme_denied", http.StatusUnauthorized)
	case errors.Is(err, integrationauth.ErrCIDRDenied):
		http.Error(w, "access_denied", http.StatusForbidden)
	default:
		http.Error(w, "invalid_credentials", http.StatusUnauthorized)
	}
}
