package handlers

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/document-service/internal/domain"
	apperrors "github.com/freight-platform/document-service/internal/platform/errors"
	"github.com/freight-platform/document-service/internal/platform/respond"
)

type shipperDocumentUseCase interface {
	List(ctx context.Context, filter domain.ShipperDocumentFilter) ([]domain.Document, int, error)
	Get(ctx context.Context, tenantID, shipperCompanyID, documentID uuid.UUID) (*domain.Document, error)
	ListAttachments(ctx context.Context, tenantID, shipperCompanyID, documentID uuid.UUID) ([]domain.Attachment, error)
	GetAttachment(ctx context.Context, tenantID, shipperCompanyID, documentID, attachmentID uuid.UUID) (*domain.Attachment, error)
	OpenContent(ctx context.Context, tenantID, shipperCompanyID, documentID, attachmentID uuid.UUID) (*domain.Attachment, io.ReadCloser, error)
}

type ShipperDocumentHandler struct {
	service shipperDocumentUseCase
}

func NewShipperDocumentHandler(service shipperDocumentUseCase) *ShipperDocumentHandler {
	return &ShipperDocumentHandler{service: service}
}

func (h *ShipperDocumentHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, err := trustedTenantID(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if err := reconcileTenant(tenantID, r.URL.Query().Get("tenant_id")); err != nil {
		respond.Error(w, err)
		return
	}
	if err := rejectRelatedEntityOverride(r); err != nil {
		respond.Error(w, err)
		return
	}
	shipperID, err := domain.ParseUUID(r.URL.Query().Get("shipper_company_id"), "shipper_company_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	shipmentID, err := requiredQueryUUID(r, "shipment_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	limit, err := parseShipperLimit(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	offset, err := parseShipperOffset(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	filter := domain.ShipperDocumentFilter{
		TenantID: tenantID, ShipperCompanyID: shipperID, ShipmentID: shipmentID,
		Limit: limit, Offset: offset,
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("document_type")); raw != "" {
		filter.DocumentType = &raw
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("document_status")); raw != "" {
		filter.DocumentStatus = &raw
	}
	docs, total, err := h.service.List(r.Context(), filter)
	if err != nil {
		respond.Error(w, err)
		return
	}
	items := make([]map[string]any, 0, len(docs))
	for i := range docs {
		items = append(items, toShipperDocumentResponse(&docs[i]))
	}
	respond.JSON(w, http.StatusOK, map[string]any{
		"items": items, "total": total, "limit": limit, "offset": offset,
	})
}

func (h *ShipperDocumentHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, shipperID, documentID, err := shipperDocumentPath(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	doc, err := h.service.Get(r.Context(), tenantID, shipperID, documentID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toShipperDocumentResponse(doc))
}

func (h *ShipperDocumentHandler) ListAttachments(w http.ResponseWriter, r *http.Request) {
	tenantID, shipperID, documentID, err := shipperDocumentPath(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	items, err := h.service.ListAttachments(r.Context(), tenantID, shipperID, documentID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for i := range items {
		out = append(out, toShipperAttachmentResponse(&items[i]))
	}
	respond.JSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *ShipperDocumentHandler) GetAttachment(w http.ResponseWriter, r *http.Request) {
	tenantID, shipperID, documentID, attachmentID, err := shipperAttachmentPath(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	item, err := h.service.GetAttachment(r.Context(), tenantID, shipperID, documentID, attachmentID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toShipperAttachmentResponse(item))
}

func (h *ShipperDocumentHandler) Content(w http.ResponseWriter, r *http.Request) {
	tenantID, shipperID, documentID, attachmentID, err := shipperAttachmentPath(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	item, body, err := h.service.OpenContent(r.Context(), tenantID, shipperID, documentID, attachmentID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	defer body.Close()
	w.Header().Set("Content-Type", item.MediaType)
	w.Header().Set("X-Content-SHA256", item.SHA256)
	w.Header().Set("Content-Disposition", contentDisposition(item.FileName))
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, body)
}

func shipperDocumentPath(r *http.Request) (uuid.UUID, uuid.UUID, uuid.UUID, error) {
	tenantID, err := trustedTenantID(r)
	if err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, err
	}
	if err := reconcileTenant(tenantID, r.URL.Query().Get("tenant_id")); err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, err
	}
	shipperID, err := domain.ParseUUID(r.URL.Query().Get("shipper_company_id"), "shipper_company_id")
	if err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, err
	}
	documentID, err := domain.ParseUUID(chi.URLParam(r, "documentId"), "document_id")
	if err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, err
	}
	return tenantID, shipperID, documentID, nil
}

func shipperAttachmentPath(r *http.Request) (uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID, error) {
	tenantID, shipperID, documentID, err := shipperDocumentPath(r)
	if err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, uuid.Nil, err
	}
	attachmentID, err := domain.ParseUUID(chi.URLParam(r, "attachmentId"), "attachment_id")
	if err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, uuid.Nil, err
	}
	return tenantID, shipperID, documentID, attachmentID, nil
}

func requiredQueryUUID(r *http.Request, name string) (uuid.UUID, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return uuid.Nil, apperrors.Validation(name+" is required", map[string]any{"field": name})
	}
	return domain.ParseUUID(raw, name)
}

func rejectRelatedEntityOverride(r *http.Request) error {
	raw := strings.TrimSpace(r.URL.Query().Get("related_entity_type"))
	if raw != "" && raw != domain.RelatedEntityTypeShipment {
		return apperrors.Validation("related_entity_type cannot be overridden", map[string]any{"field": "related_entity_type"})
	}
	return nil
}

func parseShipperLimit(r *http.Request) (int, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("limit"))
	if raw == "" {
		return 20, nil
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return 0, apperrors.Validation("invalid limit", map[string]any{"field": "limit"})
	}
	return parsed, nil
}

func parseShipperOffset(r *http.Request) (int, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("offset"))
	if raw == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return 0, apperrors.Validation("invalid offset", map[string]any{"field": "offset"})
	}
	return parsed, nil
}

func toShipperDocumentResponse(d *domain.Document) map[string]any {
	return map[string]any{
		"id":                  d.ID.String(),
		"document_number":     d.DocumentNumber,
		"document_type":       d.DocumentType,
		"document_status":     d.DocumentStatus,
		"owner_company_id":    d.OwnerCompanyID.String(),
		"related_entity_type": d.RelatedEntityType,
		"related_entity_id":   optionalUUIDString(d.RelatedEntityID),
		"legal_language":      d.LegalLanguage,
		"created_at":          d.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		"updated_at":          d.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		"version":             d.Version,
	}
}

func toShipperAttachmentResponse(item *domain.Attachment) map[string]any {
	return map[string]any{
		"id":                  item.ID.String(),
		"document_id":         item.DocumentID.String(),
		"document_version_id": optionalUUIDString(item.DocumentVersionID),
		"file_name":           item.FileName,
		"media_type":          item.MediaType,
		"size_bytes":          item.SizeBytes,
		"sha256":              item.SHA256,
		"status":              item.Status,
		"created_at":          item.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		"finalized_at":        formatDateTime(item.FinalizedAt),
	}
}

func contentDisposition(fileName string) string {
	safe, err := domain.SafeFileName(fileName)
	if err != nil || strings.ContainsAny(safe, "\"\r\n") {
		safe = "document"
	}
	return `attachment; filename="` + safe + `"`
}
