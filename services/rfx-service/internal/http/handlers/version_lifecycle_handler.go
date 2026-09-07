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

type VersionLifecycleHandler struct {
	service *service.VersionLifecycleService
}

func NewVersionLifecycleHandler(svc *service.VersionLifecycleService) *VersionLifecycleHandler {
	return &VersionLifecycleHandler{service: svc}
}

func (h *VersionLifecycleHandler) ListVersions(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	versions, err := h.service.ListVersions(r.Context(), actor, eventID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	items := make([]map[string]any, 0, len(versions))
	for i := range versions {
		items = append(items, toRfxVersionResponse(&versions[i]))
	}
	respond.JSON(w, http.StatusOK, map[string]any{"versions": items})
}

func (h *VersionLifecycleHandler) GetVersion(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	versionID, ok := parseVersionID(w, r)
	if !ok {
		return
	}
	view, err := h.service.GetVersion(r.Context(), actor, eventID, versionID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{
		"version":       toRfxVersionResponse(&view.Version),
		"questionnaire": toQuestionnaireDefinitionResponse(&view.Questionnaire),
	})
}

func (h *VersionLifecycleHandler) PublishQuestionnaire(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	var input domain.PublishQuestionnaireInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", nil))
		return
	}
	version, err := h.service.PublishQuestionnaire(r.Context(), actor, eventID, r.Header.Get("Idempotency-Key"), input)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toRfxVersionResponse(version))
}

func (h *VersionLifecycleHandler) ForkDraftFromPublished(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, ok := parseEventID(w, r)
	if !ok {
		return
	}
	version, err := h.service.ForkDraftFromPublished(r.Context(), actor, eventID, r.Header.Get("Idempotency-Key"))
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toRfxVersionResponse(version))
}

func parseVersionID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "version_id"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid version_id", nil))
		return uuid.Nil, false
	}
	return id, true
}
