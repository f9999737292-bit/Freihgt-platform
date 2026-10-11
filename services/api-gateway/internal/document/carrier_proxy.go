package document

import (
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apperrors "github.com/freight-platform/api-gateway/internal/platform/errors"
	"github.com/freight-platform/api-gateway/internal/platform/respond"
)

// CarrierHandler proxies customer carrier document reads to the
// participant-safe document-service source. Membership is enforced before
// this handler. Shipment participation stays in document-service.
type CarrierHandler struct {
	shipper *ShipperHandler
}

func NewCarrierHandler(shipper *ShipperHandler) *CarrierHandler {
	return &CarrierHandler{shipper: shipper}
}

func (h *CarrierHandler) List(w http.ResponseWriter, r *http.Request) {
	h.proxy(w, r, true, false)
}

func (h *CarrierHandler) Get(w http.ResponseWriter, r *http.Request) {
	h.proxy(w, r, false, false)
}

func (h *CarrierHandler) ListAttachments(w http.ResponseWriter, r *http.Request) {
	h.proxy(w, r, false, false)
}

func (h *CarrierHandler) GetAttachment(w http.ResponseWriter, r *http.Request) {
	h.proxy(w, r, false, false)
}

func (h *CarrierHandler) GetAttachmentContent(w http.ResponseWriter, r *http.Request) {
	h.proxy(w, r, false, true)
}

func (h *CarrierHandler) proxy(w http.ResponseWriter, r *http.Request, requireShipment, stream bool) {
	query, err := carrierDocumentQuery(r, requireShipment)
	if err != nil {
		respond.Error(w, err)
		return
	}
	path, err := carrierDocumentSourcePath(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	tenantID, userID, err := h.shipper.verifiedIdentity(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	endpoint := h.shipper.baseURL + path
	if query != "" {
		endpoint += "?" + query
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, endpoint, nil)
	if err != nil {
		writeDocumentUnavailable(w)
		return
	}
	req.Header.Set("X-Tenant-ID", tenantID)
	if userID != "" {
		req.Header.Set("X-User-ID", userID)
	}
	if companyID := strings.TrimSpace(r.Header.Get("X-Company-ID")); companyID != "" {
		req.Header.Set("X-Company-ID", companyID)
	}
	if actor := strings.TrimSpace(r.Header.Get("X-Actor-Kind")); actor != "" {
		req.Header.Set("X-Actor-Kind", actor)
	}
	resp, err := h.shipper.httpClient.Do(req)
	if err != nil {
		writeDocumentUnavailable(w)
		return
	}
	defer resp.Body.Close()
	if stream {
		copyAttachmentContent(w, resp)
		return
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		writeDocumentUnavailable(w)
		return
	}
	if contentType := resp.Header.Get("Content-Type"); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	} else {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(body)
}

func carrierDocumentQuery(r *http.Request, requireShipment bool) (string, error) {
	incoming := r.URL.Query()
	if _, present := incoming["tenant_id"]; present {
		return "", apperrors.Validation("tenant_id is not accepted", map[string]any{"field": "tenant_id"})
	}
	if values, present := incoming["related_entity_type"]; present {
		for _, value := range values {
			if strings.TrimSpace(value) != "SHIPMENT" {
				return "", apperrors.Validation("related_entity_type is server owned", map[string]any{"field": "related_entity_type"})
			}
		}
	}
	company, err := uuid.Parse(strings.TrimSpace(r.Header.Get("X-Company-ID")))
	if err != nil {
		return "", apperrors.Unauthorized("verified company context is required")
	}
	out := url.Values{}
	out.Set("carrier_company_id", company.String())
	if !requireShipment {
		return out.Encode(), nil
	}
	raw := strings.TrimSpace(incoming.Get("shipment_id"))
	if raw == "" {
		return "", apperrors.Validation("shipment_id is required", map[string]any{"field": "shipment_id"})
	}
	shipmentID, err := uuid.Parse(raw)
	if err != nil {
		return "", apperrors.Validation("invalid shipment_id", map[string]any{"field": "shipment_id"})
	}
	out.Set("shipment_id", shipmentID.String())
	for _, key := range []string{"document_type", "document_status", "limit", "offset"} {
		for _, value := range incoming[key] {
			out.Add(key, value)
		}
	}
	return out.Encode(), nil
}

func carrierDocumentSourcePath(r *http.Request) (string, error) {
	path := "/v1/carrier/documents"
	if documentID := strings.TrimSpace(chi.URLParam(r, "documentId")); documentID != "" {
		parsed, err := uuid.Parse(documentID)
		if err != nil {
			return "", apperrors.Validation("invalid documentId", map[string]any{"field": "documentId"})
		}
		path += "/" + parsed.String()
	}
	if !strings.Contains(r.URL.Path, "/attachments") {
		return path, nil
	}
	path += "/attachments"
	if attachmentID := strings.TrimSpace(chi.URLParam(r, "attachmentId")); attachmentID != "" {
		parsed, err := uuid.Parse(attachmentID)
		if err != nil {
			return "", apperrors.Validation("invalid attachmentId", map[string]any{"field": "attachmentId"})
		}
		path += "/" + parsed.String()
	}
	if strings.HasSuffix(r.URL.Path, "/content") {
		path += "/content"
	}
	return path, nil
}

func writeDocumentUnavailable(w http.ResponseWriter) {
	respond.JSON(w, http.StatusServiceUnavailable, map[string]any{
		"error": map[string]any{
			"code":    "SERVICE_UNAVAILABLE",
			"message": "document service is temporarily unavailable",
			"details": map[string]any{"service": "document-service"},
		},
	})
}
