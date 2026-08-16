package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/document-service/internal/platform/respond"
	"github.com/freight-platform/document-service/internal/service"
	apperrors "github.com/freight-platform/document-service/internal/platform/errors"
)

type PODUploadHandler struct {
	pod *service.PODUploadService
}

func NewPODUploadHandler(pod *service.PODUploadService) *PODUploadHandler {
	return &PODUploadHandler{pod: pod}
}

func (h *PODUploadHandler) CreateIntent(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(strings.TrimSpace(r.Header.Get("X-Tenant-ID")))
	if err != nil {
		respond.Error(w, apperrors.Validation("X-Tenant-ID required", nil))
		return
	}
	var body struct {
		ShipmentID     string `json:"shipmentId"`
		DriverID       string `json:"driverId"`
		OwnerCompanyID string `json:"ownerCompanyId"`
		MimeType       string `json:"mimeType"`
		FileName       string `json:"fileName"`
		IdempotencyKey string `json:"idempotencyKey"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, apperrors.Validation("invalid body", nil))
		return
	}
	shipmentID, err := uuid.Parse(strings.TrimSpace(body.ShipmentID))
	if err != nil {
		respond.Error(w, apperrors.Validation("shipmentId required", nil))
		return
	}
	driverID, err := uuid.Parse(strings.TrimSpace(body.DriverID))
	if err != nil {
		respond.Error(w, apperrors.Validation("driverId required", nil))
		return
	}
	ownerID, err := uuid.Parse(strings.TrimSpace(body.OwnerCompanyID))
	if err != nil {
		respond.Error(w, apperrors.Validation("ownerCompanyId required", nil))
		return
	}
	intent, err := h.pod.CreateUploadIntent(r.Context(), service.CreatePODUploadInput{
		TenantID: tenantID, ShipmentID: shipmentID, DriverID: driverID,
		OwnerCompanyID: ownerID, MimeType: body.MimeType, FileName: body.FileName,
		IdempotencyKey: body.IdempotencyKey,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, intent)
}

func (h *PODUploadHandler) UploadContent(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(strings.TrimSpace(r.Header.Get("X-Tenant-ID")))
	if err != nil {
		respond.Error(w, apperrors.Validation("X-Tenant-ID required", nil))
		return
	}
	uploadID, err := uuid.Parse(chi.URLParam(r, "uploadId"))
	if err != nil {
		respond.Error(w, apperrors.Validation("uploadId required", nil))
		return
	}
	token := strings.TrimSpace(r.Header.Get("X-Upload-Token"))
	if err := h.pod.UploadContent(r.Context(), tenantID, uploadID, token, io.LimitReader(r.Body, 11<<20)); err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]string{"status": "uploaded"})
}

func (h *PODUploadHandler) Complete(w http.ResponseWriter, r *http.Request) {
	tenantID, err := uuid.Parse(strings.TrimSpace(r.Header.Get("X-Tenant-ID")))
	if err != nil {
		respond.Error(w, apperrors.Validation("X-Tenant-ID required", nil))
		return
	}
	uploadID, err := uuid.Parse(chi.URLParam(r, "uploadId"))
	if err != nil {
		respond.Error(w, apperrors.Validation("uploadId required", nil))
		return
	}
	var body struct {
		DriverID       string `json:"driverId"`
		ChecksumSHA256 string `json:"checksumSha256"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respond.Error(w, apperrors.Validation("invalid body", nil))
		return
	}
	driverID, err := uuid.Parse(strings.TrimSpace(body.DriverID))
	if err != nil {
		respond.Error(w, apperrors.Validation("driverId required", nil))
		return
	}
	result, err := h.pod.CompleteUpload(r.Context(), service.CompletePODUploadInput{
		TenantID: tenantID, UploadID: uploadID, DriverID: driverID, ChecksumSHA256: body.ChecksumSHA256,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, result)
}
