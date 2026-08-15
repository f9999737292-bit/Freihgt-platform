package driver

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/api-gateway/internal/document"
	apperrors "github.com/freight-platform/api-gateway/internal/platform/errors"
	"github.com/freight-platform/api-gateway/internal/platform/respond"
)

func (h *Handler) InitiatePODUpload(w http.ResponseWriter, r *http.Request) {
	shipmentID := strings.TrimSpace(chi.URLParam(r, "shipmentId"))
	reqCtx, me, err := h.requireAssignedDriver(r, shipmentID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		respond.Error(w, err)
		return
	}
	var payload struct {
		MimeType       string `json:"mimeType"`
		FileName       string `json:"fileName"`
		IdempotencyKey string `json:"idempotencyKey"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", map[string]any{"field": "body"}))
		return
	}
	idem := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idem == "" {
		idem = strings.TrimSpace(payload.IdempotencyKey)
	}
	raw, status, err := h.documents.CreatePODUpload(r.Context(), document.CreatePODUploadRequest{
		TenantID: reqCtx.TenantID, ShipmentID: shipmentID, DriverID: me.DriverID,
		OwnerCompanyID: me.CompanyID, MimeType: payload.MimeType, FileName: payload.FileName,
		IdempotencyKey: idem,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSONRaw(w, status, raw)
}

func (h *Handler) UploadPODContent(w http.ResponseWriter, r *http.Request) {
	shipmentID := strings.TrimSpace(chi.URLParam(r, "shipmentId"))
	uploadID := strings.TrimSpace(chi.URLParam(r, "uploadId"))
	reqCtx, _, err := h.requireAssignedDriver(r, shipmentID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	token := strings.TrimSpace(r.Header.Get("X-Upload-Token"))
	if token == "" {
		respond.Error(w, apperrors.Validation("X-Upload-Token is required", map[string]any{"field": "X-Upload-Token"}))
		return
	}
	content, err := io.ReadAll(io.LimitReader(r.Body, 11<<20))
	if err != nil {
		respond.Error(w, err)
		return
	}
	status, err := h.documents.UploadPODContent(r.Context(), reqCtx.TenantID, uploadID, token, content, r.Header.Get("Content-Type"))
	if err != nil {
		respond.Error(w, err)
		return
	}
	if status >= 400 {
		respond.Error(w, apperrors.Validation("upload failed", map[string]any{"status": status}))
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"status": "uploaded"})
}

func (h *Handler) CompletePODUpload(w http.ResponseWriter, r *http.Request) {
	shipmentID := strings.TrimSpace(chi.URLParam(r, "shipmentId"))
	uploadID := strings.TrimSpace(chi.URLParam(r, "uploadId"))
	reqCtx, me, err := h.requireAssignedDriver(r, shipmentID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		respond.Error(w, err)
		return
	}
	var payload struct {
		ChecksumSHA256 string `json:"checksumSha256"`
	}
	_ = json.Unmarshal(body, &payload)
	raw, status, err := h.documents.CompletePODUpload(r.Context(), reqCtx.TenantID, uploadID, me.DriverID, payload.ChecksumSHA256)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSONRaw(w, status, raw)
}

type driverIdentity struct {
	DriverID  string
	CompanyID string
}

func (h *Handler) requireAssignedDriver(r *http.Request, shipmentID string) (RequestContext, driverIdentity, error) {
	reqCtx, err := h.buildRequestContext(r)
	if err != nil {
		return RequestContext{}, driverIdentity{}, err
	}
	if err := h.ensureDriverAccess(r, reqCtx); err != nil {
		return RequestContext{}, driverIdentity{}, err
	}
	if _, status, err := h.client.GetShipment(r.Context(), reqCtx, shipmentID); err != nil || status == http.StatusNotFound {
		return RequestContext{}, driverIdentity{}, apperrors.NotFound("shipment not found")
	} else if status >= 400 {
		return RequestContext{}, driverIdentity{}, apperrors.ServiceUnavailable("shipment service is temporarily unavailable", "shipment-service")
	}
	meRaw, status, err := h.client.GetMe(r.Context(), reqCtx)
	if err != nil || status >= 400 {
		return RequestContext{}, driverIdentity{}, apperrors.Unauthorized("driver identity is not configured")
	}
	var me struct {
		ID        string `json:"id"`
		CompanyID string `json:"companyId"`
	}
	if err := json.Unmarshal(meRaw, &me); err != nil || strings.TrimSpace(me.ID) == "" || strings.TrimSpace(me.CompanyID) == "" {
		return RequestContext{}, driverIdentity{}, apperrors.Unauthorized("driver identity is not configured")
	}
	return reqCtx, driverIdentity{DriverID: me.ID, CompanyID: me.CompanyID}, nil
}
