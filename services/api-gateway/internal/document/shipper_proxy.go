package document

import (
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/api-gateway/internal/config"
	gwmiddleware "github.com/freight-platform/api-gateway/internal/http/middleware"
	apperrors "github.com/freight-platform/api-gateway/internal/platform/errors"
	"github.com/freight-platform/api-gateway/internal/platform/respond"
)

// ShipperHandler proxies customer document reads to the participant-safe
// document-service source. It does not decide document type, status, or
// shipment participation.
type ShipperHandler struct {
	httpClient  *http.Client
	baseURL     string
	authEnabled bool
	devTenantID string
}

func NewShipperHandler(cfg config.Config) *ShipperHandler {
	return &ShipperHandler{
		httpClient:  &http.Client{Timeout: time.Duration(cfg.ProxyTimeoutSeconds) * time.Second},
		baseURL:     strings.TrimRight(cfg.Services.Document, "/"),
		authEnabled: cfg.AuthEnabled,
		devTenantID: strings.TrimSpace(cfg.DevTenantID),
	}
}

func (h *ShipperHandler) List(w http.ResponseWriter, r *http.Request) {
	h.proxy(w, r, true, false)
}

func (h *ShipperHandler) Get(w http.ResponseWriter, r *http.Request) {
	h.proxy(w, r, false, false)
}

func (h *ShipperHandler) ListAttachments(w http.ResponseWriter, r *http.Request) {
	h.proxy(w, r, false, false)
}

func (h *ShipperHandler) GetAttachment(w http.ResponseWriter, r *http.Request) {
	h.proxy(w, r, false, false)
}

func (h *ShipperHandler) GetAttachmentContent(w http.ResponseWriter, r *http.Request) {
	h.proxy(w, r, false, true)
}

func (h *ShipperHandler) proxy(w http.ResponseWriter, r *http.Request, requireShipment, stream bool) {
	if err := sanitizeShipperDocumentQuery(r, requireShipment); err != nil {
		respond.Error(w, err)
		return
	}
	tenantID, userID, err := h.verifiedIdentity(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	path := shipperDocumentSourcePath(r)
	endpoint := h.baseURL + path
	if raw := r.URL.RawQuery; raw != "" {
		endpoint += "?" + raw
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, endpoint, nil)
	if err != nil {
		respond.Error(w, apperrors.ServiceUnavailable("document service is temporarily unavailable", "document-service"))
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
	resp, err := h.httpClient.Do(req)
	if err != nil {
		respond.Error(w, apperrors.ServiceUnavailable("document service is temporarily unavailable", "document-service"))
		return
	}
	defer resp.Body.Close()
	if stream {
		copyAttachmentContent(w, resp)
		return
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		respond.Error(w, apperrors.ServiceUnavailable("document service is temporarily unavailable", "document-service"))
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

func sanitizeShipperDocumentQuery(r *http.Request, requireShipment bool) error {
	query := r.URL.Query()
	if _, present := query["tenant_id"]; present {
		return apperrors.Validation("tenant_id is not accepted", map[string]any{"field": "tenant_id"})
	}
	if values, present := query["related_entity_type"]; present {
		for _, value := range values {
			if strings.TrimSpace(value) != "SHIPMENT" {
				return apperrors.Validation("related_entity_type is server owned", map[string]any{"field": "related_entity_type"})
			}
		}
		query.Del("related_entity_type")
	}
	if requireShipment {
		raw := strings.TrimSpace(query.Get("shipment_id"))
		if raw == "" {
			return apperrors.Validation("shipment_id is required", map[string]any{"field": "shipment_id"})
		}
		parsed, err := uuid.Parse(raw)
		if err != nil {
			return apperrors.Validation("invalid shipment_id", map[string]any{"field": "shipment_id"})
		}
		query.Set("shipment_id", parsed.String())
	}
	r.URL.RawQuery = query.Encode()
	return nil
}

func shipperDocumentSourcePath(r *http.Request) string {
	path := "/v1/shipper/documents"
	if documentID := strings.TrimSpace(chi.URLParam(r, "documentId")); documentID != "" {
		path += "/" + documentID
	}
	if !strings.Contains(r.URL.Path, "/attachments") {
		return path
	}
	path += "/attachments"
	if attachmentID := strings.TrimSpace(chi.URLParam(r, "attachmentId")); attachmentID != "" {
		path += "/" + attachmentID
	}
	if strings.HasSuffix(r.URL.Path, "/content") {
		path += "/content"
	}
	return path
}

func (h *ShipperHandler) verifiedIdentity(r *http.Request) (string, string, error) {
	if h.authEnabled {
		ac, err := gwmiddleware.MustAuthContext(r.Context())
		if err != nil {
			return "", "", apperrors.Unauthorized("verified tenant context is required")
		}
		return ac.TenantID, ac.UserID, nil
	}
	if h.devTenantID == "" {
		return "", "", apperrors.Unauthorized("development tenant context is not configured")
	}
	return h.devTenantID, "", nil
}

func copyAttachmentContent(w http.ResponseWriter, resp *http.Response) {
	for _, key := range []string{"Content-Type", "Content-Disposition", "X-Content-SHA256", "Content-Length"} {
		if value := resp.Header.Get(key); value != "" {
			w.Header().Set(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}
