package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/platform/respond"
	"github.com/freight-platform/rfx-service/internal/service"
)

type TemplateLibraryHandler struct {
	service *service.TemplateLibraryService
}

func NewTemplateLibraryHandler(svc *service.TemplateLibraryService) *TemplateLibraryHandler {
	return &TemplateLibraryHandler{service: svc}
}

func (h *TemplateLibraryHandler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	filter := domain.TemplateListFilter{Limit: parseLimit(r), Offset: parseOffset(r)}
	if raw := r.URL.Query().Get("status"); raw != "" {
		filter.Status = &raw
	}
	if raw := r.URL.Query().Get("owner_company_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			respond.Error(w, apperrors.Validation("invalid owner_company_id", nil))
			return
		}
		filter.OwnerCompanyID = &id
	}
	if raw := r.URL.Query().Get("rfx_type"); raw != "" {
		filter.RfxType = &raw
	}
	if raw := r.URL.Query().Get("search"); raw != "" {
		filter.Search = &raw
	}
	items, total, err := h.service.ListTemplates(r.Context(), actor, filter)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respItems := make([]map[string]any, 0, len(items))
	for i := range items {
		respItems = append(respItems, toRfxTemplateResponse(&items[i]))
	}
	respond.JSON(w, http.StatusOK, map[string]any{"items": respItems, "total": total, "limit": filter.Limit, "offset": filter.Offset})
}

func (h *TemplateLibraryHandler) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	var in domain.CreateTemplateInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	detail, err := h.service.CreateTemplate(r.Context(), actor, in)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toTemplateDetailResponse(detail))
}

func (h *TemplateLibraryHandler) GetTemplate(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	templateID, ok := parseTemplateID(w, r)
	if !ok {
		return
	}
	detail, err := h.service.GetTemplate(r.Context(), actor, templateID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toTemplateDetailResponse(detail))
}

func (h *TemplateLibraryHandler) UpdateTemplate(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	templateID, ok := parseTemplateID(w, r)
	if !ok {
		return
	}
	var in domain.UpdateTemplateInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	tmpl, err := h.service.UpdateTemplate(r.Context(), actor, templateID, in)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toRfxTemplateResponse(tmpl))
}

func (h *TemplateLibraryHandler) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	templateID, ok := parseTemplateID(w, r)
	if !ok {
		return
	}
	if err := h.service.DeleteTemplate(r.Context(), actor, templateID); err != nil {
		respond.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *TemplateLibraryHandler) ArchiveTemplate(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	templateID, ok := parseTemplateID(w, r)
	if !ok {
		return
	}
	tmpl, err := h.service.ArchiveTemplate(r.Context(), actor, templateID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toRfxTemplateResponse(tmpl))
}

func (h *TemplateLibraryHandler) PublishTemplateVersion(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	templateID, ok := parseTemplateID(w, r)
	if !ok {
		return
	}
	var in domain.PublishTemplateVersionInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	version, err := h.service.PublishTemplateVersion(r.Context(), actor, templateID, r.Header.Get("Idempotency-Key"), in)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toRfxTemplateVersionResponse(version))
}

func (h *TemplateLibraryHandler) ForkDraftFromPublished(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	templateID, ok := parseTemplateID(w, r)
	if !ok {
		return
	}
	version, err := h.service.ForkDraftFromPublished(r.Context(), actor, templateID, r.Header.Get("Idempotency-Key"))
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toRfxTemplateVersionResponse(version))
}

func parseTemplateID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid template id", nil))
		return uuid.Nil, false
	}
	return id, true
}
