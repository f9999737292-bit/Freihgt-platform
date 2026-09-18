package handlers

import (
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/platform/respond"
	"github.com/freight-platform/rfx-service/internal/service"
	"github.com/freight-platform/rfx-service/internal/erpjson"
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

func readERPJSONBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
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
