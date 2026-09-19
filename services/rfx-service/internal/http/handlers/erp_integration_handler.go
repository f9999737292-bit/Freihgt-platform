package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/rfx-service/internal/erpjson"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/platform/respond"
	"github.com/freight-platform/rfx-service/internal/service"
)

type ErpIntegrationHandler struct {
	service *service.ErpIntegrationService
}

func NewErpIntegrationHandler(svc *service.ErpIntegrationService) *ErpIntegrationHandler {
	return &ErpIntegrationHandler{service: svc}
}

func (h *ErpIntegrationHandler) PreviewCreateDraft(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireIntegrationActor(w, r, domain.ScopeDraftPreview, domain.ScopeDraftCreate)
	if !ok {
		return
	}
	raw, err := readERPJSONBody(w, r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	preview, err := h.service.PreviewCreateDraft(r.Context(), actor, raw)
	if err != nil {
		respond.Error(w, err)
		return
	}
	writePreviewResponse(w, preview)
}

func (h *ErpIntegrationHandler) PreviewUpdateDraft(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireIntegrationActor(w, r, domain.ScopeDraftPreview, domain.ScopeDraftRead)
	if !ok {
		return
	}
	eventID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid event id", map[string]any{"field": "id"}))
		return
	}
	raw, err := readERPJSONBody(w, r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	preview, err := h.service.PreviewUpdateDraft(r.Context(), actor, eventID, raw)
	if err != nil {
		respond.Error(w, err)
		return
	}
	writePreviewResponse(w, preview)
}

func (h *ErpIntegrationHandler) CommitCreateDraft(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireIntegrationActor(w, r, domain.ScopeDraftCommit, domain.ScopeDraftCreate)
	if !ok {
		return
	}
	in, err := readERPCreateCommitBody(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	result, err := h.service.CommitCreateDraft(r.Context(), actor, in, r.Header.Get("Idempotency-Key"))
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, result)
}

func readERPCreateCommitBody(r *http.Request) (domain.ErpCreateCommitInput, error) {
	raw, err := readERPJSONBody(nil, r)
	if err != nil {
		return domain.ErpCreateCommitInput{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var in domain.ErpCreateCommitInput
	if err := decoder.Decode(&in); err != nil {
		return domain.ErpCreateCommitInput{}, apperrors.Validation("invalid request body", map[string]any{"field": "body"})
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return domain.ErpCreateCommitInput{}, apperrors.Validation("invalid request body", map[string]any{"field": "body"})
	}
	return in, nil
}

func requireERPJSONContentType(r *http.Request) error {
	raw := strings.TrimSpace(r.Header.Get("Content-Type"))
	if raw == "" {
		return unsupportedERPMediaType()
	}
	mediaType, params, err := mime.ParseMediaType(raw)
	if err != nil {
		return unsupportedERPMediaType()
	}
	if !strings.EqualFold(mediaType, "application/json") {
		return unsupportedERPMediaType()
	}
	if charset, ok := params["charset"]; ok && !strings.EqualFold(strings.TrimSpace(charset), "utf-8") {
		return unsupportedERPMediaType()
	}
	return nil
}

func unsupportedERPMediaType() error {
	return apperrors.Validation("unsupported media type", map[string]any{
		"field":        "content_type",
		"machine_code": erpjson.MachineCodeUnsupportedMediaType,
		"message_key":  "rfx.erp.unsupported_media_type",
	})
}

func readERPJSONBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	if err := requireERPJSONContentType(r); err != nil {
		return nil, err
	}
	defer r.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(r.Body, erpjson.MaxBodyBytes+1))
	if err != nil {
		return nil, apperrors.Validation("invalid request body", map[string]any{"field": "body"})
	}
	if len(raw) > erpjson.MaxBodyBytes {
		return nil, apperrors.RequestBodyTooLarge("request body too large")
	}
	if len(raw) == 0 {
		return nil, apperrors.Validation("request body is required", map[string]any{"field": "body"})
	}
	return raw, nil
}

func writePreviewResponse(w http.ResponseWriter, preview *erpjson.PreviewResponse) {
	if preview == nil {
		respond.Error(w, apperrors.Internal("preview response missing", nil))
		return
	}
	if erpjson.HasErrors(preview.Errors) {
		respond.JSON(w, http.StatusUnprocessableEntity, preview)
		return
	}
	respond.JSON(w, http.StatusOK, preview)
}
