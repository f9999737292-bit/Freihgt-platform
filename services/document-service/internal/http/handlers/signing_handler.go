package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/document-service/internal/domain"
	apperrors "github.com/freight-platform/document-service/internal/platform/errors"
	"github.com/freight-platform/document-service/internal/platform/respond"
	"github.com/freight-platform/document-service/internal/service"
)

type SigningHandler struct {
	service *service.SigningService
}

func NewSigningHandler(svc *service.SigningService) *SigningHandler {
	return &SigningHandler{service: svc}
}

type createSigningSessionRequest struct {
	TenantID             string  `json:"tenant_id"`
	RequiredSignersCount int     `json:"required_signers_count"`
	ExpiresAt            *string `json:"expires_at"`
}

type addSignatureRequest struct {
	TenantID               string  `json:"tenant_id"`
	SignerUserID           string  `json:"signer_user_id"`
	SignerCompanyID        string  `json:"signer_company_id"`
	SignatureType          string  `json:"signature_type"`
	SignaturePayloadPath   *string `json:"signature_payload_path"`
	CertificateFingerprint *string `json:"certificate_fingerprint"`
}

func (h *SigningHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
	documentID, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req createSigningSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}
	tenantID, err := domain.ParseUUID(req.TenantID, "tenant_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	expiresAt, err := domain.ParseDateTime(derefString(req.ExpiresAt), "expires_at")
	if err != nil {
		respond.Error(w, err)
		return
	}
	session, err := h.service.CreateSession(r.Context(), documentID, domain.CreateSigningSessionInput{
		TenantID: tenantID, RequiredSignersCount: req.RequiredSignersCount, ExpiresAt: expiresAt,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toSigningSessionResponse(session))
}

func (h *SigningHandler) GetSession(w http.ResponseWriter, r *http.Request) {
	id, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	session, err := h.service.GetSession(r.Context(), id)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toSigningSessionResponse(session))
}

func (h *SigningHandler) AddSignature(w http.ResponseWriter, r *http.Request) {
	sessionID, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req addSignatureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}
	tenantID, err := domain.ParseUUID(req.TenantID, "tenant_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	signerUserID, err := domain.ParseUUID(req.SignerUserID, "signer_user_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	signerCompanyID, err := domain.ParseUUID(req.SignerCompanyID, "signer_company_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	signature, session, doc, err := h.service.AddSignature(r.Context(), sessionID, domain.AddSignatureInput{
		TenantID: tenantID, SignerUserID: signerUserID, SignerCompanyID: signerCompanyID,
		SignatureType: req.SignatureType, SignaturePayloadPath: req.SignaturePayloadPath,
		CertificateFingerprint: req.CertificateFingerprint,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, map[string]any{
		"signature":        toSignatureResponse(signature),
		"signing_session":  toSigningSessionResponse(session),
		"document":         toDocumentResponse(doc),
	})
}

func toSigningSessionResponse(s *domain.SigningSession) map[string]any {
	return map[string]any{
		"id":                      s.ID.String(),
		"tenant_id":               s.TenantID.String(),
		"document_id":             s.DocumentID.String(),
		"status":                  s.Status,
		"required_signers_count":  s.RequiredSignersCount,
		"completed_signers_count": s.CompletedSignersCount,
		"created_at":              s.CreatedAt.UTC().Format(time.RFC3339),
		"expires_at":              formatDateTime(s.ExpiresAt),
	}
}

func toSignatureResponse(s *domain.Signature) map[string]any {
	return map[string]any{
		"id":                      s.ID.String(),
		"tenant_id":               s.TenantID.String(),
		"signing_session_id":      s.SigningSessionID.String(),
		"document_id":             s.DocumentID.String(),
		"signer_user_id":          optionalUUIDString(s.SignerUserID),
		"signer_company_id":       optionalUUIDString(s.SignerCompanyID),
		"signature_type":          s.SignatureType,
		"signature_payload_path":  s.SignaturePayloadPath,
		"certificate_fingerprint": s.CertificateFingerprint,
		"signed_at":               formatDateTime(s.SignedAt),
		"verification_status":     s.VerificationStatus,
		"created_at":              s.CreatedAt.UTC().Format(time.RFC3339),
	}
}
