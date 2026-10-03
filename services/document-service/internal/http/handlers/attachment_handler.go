package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/document-service/internal/domain"
	apperrors "github.com/freight-platform/document-service/internal/platform/errors"
	"github.com/freight-platform/document-service/internal/platform/respond"
	"github.com/freight-platform/document-service/internal/service"
)

type AttachmentHandler struct {
	service *service.AttachmentService
}

func NewAttachmentHandler(service *service.AttachmentService) *AttachmentHandler {
	return &AttachmentHandler{service: service}
}

func (h *AttachmentHandler) Create(w http.ResponseWriter, r *http.Request) {
	documentID, tenantID, err := documentAndTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	versionID, err := optionalHeaderUUID(r, "X-Document-Version-ID")
	if err != nil {
		respond.Error(w, err)
		return
	}
	if q := strings.TrimSpace(r.URL.Query().Get("tenant_id")); q != "" && q != tenantID.String() {
		respond.Error(w, apperrors.Unauthorized("tenant query does not match trusted tenant"))
		return
	}
	item, err := h.service.Create(r.Context(), documentID, service.CreateAttachmentInput{
		TenantID: tenantID, DocumentVersionID: versionID,
		FileName: r.Header.Get("X-File-Name"), MediaType: r.Header.Get("Content-Type"),
		ClientSHA256: r.Header.Get("X-Content-SHA256"), IdempotencyKey: r.Header.Get("Idempotency-Key"),
		ActorUserID: optionalActor(r), Body: r.Body,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, attachmentJSON(item))
}

func (h *AttachmentHandler) Get(w http.ResponseWriter, r *http.Request) {
	documentID, attachmentID, tenantID, err := attachmentPath(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	item, err := h.service.Get(r.Context(), tenantID, documentID, attachmentID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, attachmentJSON(item))
}

func (h *AttachmentHandler) Content(w http.ResponseWriter, r *http.Request) {
	documentID, attachmentID, tenantID, err := attachmentPath(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	item, body, err := h.service.Open(r.Context(), tenantID, documentID, attachmentID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	defer body.Close()
	w.Header().Set("Content-Type", item.MediaType)
	w.Header().Set("X-Content-SHA256", item.SHA256)
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, body)
}

func (h *AttachmentHandler) Finalize(w http.ResponseWriter, r *http.Request) {
	documentID, attachmentID, tenantID, err := attachmentPath(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	item, err := h.service.Finalize(r.Context(), tenantID, documentID, attachmentID, optionalActor(r))
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, attachmentJSON(item))
}

func (h *AttachmentHandler) AttachSignature(w http.ResponseWriter, r *http.Request) {
	documentID, attachmentID, tenantID, err := attachmentPath(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req signatureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}
	if req.TenantID != "" && req.TenantID != tenantID.String() {
		respond.Error(w, apperrors.Unauthorized("tenant header does not match body tenant"))
		return
	}
	var signingTime *time.Time
	if req.SigningTime != "" {
		parsed, err := time.Parse(time.RFC3339, req.SigningTime)
		if err != nil {
			respond.Error(w, apperrors.Validation("signing_time must be RFC3339", map[string]any{"field": "signing_time"}))
			return
		}
		signingTime = &parsed
	}
	sig, err := h.service.AttachSignature(r.Context(), tenantID, documentID, attachmentID, service.AttachSignatureInput{
		Format: req.SignatureFormat, Reference: req.SignatureReference,
		Subject: req.CertificateSubject, Issuer: req.CertificateIssuer, Serial: req.CertificateSerial,
		Thumbprint: req.CertificateThumbprint, SigningTime: signingTime,
		IdempotencyKey: r.Header.Get("Idempotency-Key"), ActorUserID: optionalActor(r),
	})
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, signatureJSON(sig))
}

func (h *AttachmentHandler) GetSignature(w http.ResponseWriter, r *http.Request) {
	documentID, attachmentID, tenantID, err := attachmentPath(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	signatureID, err := domain.ParseUUID(chi.URLParam(r, "signatureId"), "signature_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	sig, err := h.service.GetSignature(r.Context(), tenantID, documentID, attachmentID, signatureID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, signatureJSON(sig))
}

type signatureRequest struct {
	TenantID              string  `json:"tenant_id"`
	SignatureFormat       string  `json:"signature_format"`
	SignatureReference    string  `json:"signature_reference"`
	CertificateSubject    *string `json:"certificate_subject"`
	CertificateIssuer     *string `json:"certificate_issuer"`
	CertificateSerial     *string `json:"certificate_serial"`
	CertificateThumbprint *string `json:"certificate_thumbprint"`
	SigningTime           string  `json:"signing_time"`
}

func documentAndTenant(r *http.Request) (uuid.UUID, uuid.UUID, error) {
	documentID, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	tenantID, err := trustedTenantID(r)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	if q := strings.TrimSpace(r.URL.Query().Get("tenant_id")); q != "" && q != tenantID.String() {
		return uuid.Nil, uuid.Nil, apperrors.Unauthorized("tenant query does not match trusted tenant")
	}
	return documentID, tenantID, nil
}

func attachmentPath(r *http.Request) (uuid.UUID, uuid.UUID, uuid.UUID, error) {
	documentID, tenantID, err := documentAndTenant(r)
	if err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, err
	}
	attachmentID, err := domain.ParseUUID(chi.URLParam(r, "attachmentId"), "attachment_id")
	if err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, err
	}
	return documentID, attachmentID, tenantID, nil
}

func optionalHeaderUUID(r *http.Request, name string) (*uuid.UUID, error) {
	raw := strings.TrimSpace(r.Header.Get(name))
	if raw == "" {
		return nil, nil
	}
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil {
		return nil, apperrors.Validation(name+" is invalid", map[string]any{"field": name})
	}
	return &id, nil
}

func optionalActor(r *http.Request) *uuid.UUID {
	id, err := optionalHeaderUUID(r, "X-User-ID")
	if err != nil {
		return nil
	}
	return id
}

func attachmentJSON(item *domain.Attachment) map[string]any {
	return map[string]any{
		"attachment_id":       item.ID.String(),
		"document_id":         item.DocumentID.String(),
		"tenant_id":           item.TenantID.String(),
		"document_version_id": optionalUUIDString(item.DocumentVersionID),
		"file_name":           item.FileName,
		"media_type":          item.MediaType,
		"size_bytes":          item.SizeBytes,
		"sha256":              item.SHA256,
		"status":              item.Status,
		"created_at":          item.CreatedAt.UTC().Format(time.RFC3339),
		"finalized_at":        formatTime(item.FinalizedAt),
	}
}

func signatureJSON(item *domain.AttachmentSignature) map[string]any {
	return map[string]any{
		"signature_id":           item.ID.String(),
		"attachment_id":          item.AttachmentID.String(),
		"tenant_id":              item.TenantID.String(),
		"signature_format":       item.SignatureFormat,
		"signature_reference":    item.SignatureReference,
		"certificate_subject":    item.CertificateSubject,
		"certificate_issuer":     item.CertificateIssuer,
		"certificate_serial":     item.CertificateSerial,
		"certificate_thumbprint": item.CertificateThumbprint,
		"signing_time":           formatTime(item.SigningTime),
		"verification_status":    item.VerificationStatus,
		"verification_time":      formatTime(item.VerificationTime),
		"verification_error":     item.VerificationError,
		"created_at":             item.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func formatTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC().Format(time.RFC3339)
}
