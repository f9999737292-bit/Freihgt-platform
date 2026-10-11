package handlers

import (
	"context"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/document-service/internal/domain"
	"github.com/freight-platform/document-service/internal/platform/respond"
)

type carrierDocumentUseCase interface {
	List(ctx context.Context, filter domain.CarrierDocumentFilter) ([]domain.Document, int, error)
	Get(ctx context.Context, tenantID, carrierCompanyID, documentID uuid.UUID) (*domain.Document, error)
	ListAttachments(ctx context.Context, tenantID, carrierCompanyID, documentID uuid.UUID) ([]domain.Attachment, error)
	GetAttachment(ctx context.Context, tenantID, carrierCompanyID, documentID, attachmentID uuid.UUID) (*domain.Attachment, error)
	OpenContent(ctx context.Context, tenantID, carrierCompanyID, documentID, attachmentID uuid.UUID) (*domain.Attachment, io.ReadCloser, error)
}

type CarrierDocumentHandler struct {
	service carrierDocumentUseCase
}

func NewCarrierDocumentHandler(service carrierDocumentUseCase) *CarrierDocumentHandler {
	return &CarrierDocumentHandler{service: service}
}

func (h *CarrierDocumentHandler) List(w http.ResponseWriter, r *http.Request) {
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
	carrierID, err := domain.ParseUUID(r.URL.Query().Get("carrier_company_id"), "carrier_company_id")
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
	filter := domain.CarrierDocumentFilter{
		TenantID: tenantID, CarrierCompanyID: carrierID, ShipmentID: shipmentID,
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

func (h *CarrierDocumentHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID, carrierID, documentID, err := carrierDocumentPath(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	doc, err := h.service.Get(r.Context(), tenantID, carrierID, documentID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toShipperDocumentResponse(doc))
}

func (h *CarrierDocumentHandler) ListAttachments(w http.ResponseWriter, r *http.Request) {
	tenantID, carrierID, documentID, err := carrierDocumentPath(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	items, err := h.service.ListAttachments(r.Context(), tenantID, carrierID, documentID)
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

func (h *CarrierDocumentHandler) GetAttachment(w http.ResponseWriter, r *http.Request) {
	tenantID, carrierID, documentID, attachmentID, err := carrierAttachmentPath(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	item, err := h.service.GetAttachment(r.Context(), tenantID, carrierID, documentID, attachmentID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toShipperAttachmentResponse(item))
}

func (h *CarrierDocumentHandler) Content(w http.ResponseWriter, r *http.Request) {
	tenantID, carrierID, documentID, attachmentID, err := carrierAttachmentPath(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	item, body, err := h.service.OpenContent(r.Context(), tenantID, carrierID, documentID, attachmentID)
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

func carrierDocumentPath(r *http.Request) (uuid.UUID, uuid.UUID, uuid.UUID, error) {
	tenantID, err := trustedTenantID(r)
	if err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, err
	}
	if err := reconcileTenant(tenantID, r.URL.Query().Get("tenant_id")); err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, err
	}
	carrierID, err := domain.ParseUUID(r.URL.Query().Get("carrier_company_id"), "carrier_company_id")
	if err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, err
	}
	documentID, err := domain.ParseUUID(chi.URLParam(r, "documentId"), "document_id")
	if err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, err
	}
	return tenantID, carrierID, documentID, nil
}

func carrierAttachmentPath(r *http.Request) (uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID, error) {
	tenantID, carrierID, documentID, err := carrierDocumentPath(r)
	if err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, uuid.Nil, err
	}
	attachmentID, err := domain.ParseUUID(chi.URLParam(r, "attachmentId"), "attachment_id")
	if err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, uuid.Nil, err
	}
	return tenantID, carrierID, documentID, attachmentID, nil
}
