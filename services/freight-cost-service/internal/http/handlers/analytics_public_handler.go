package handlers

import (
	"net/http"

	"github.com/freight-platform/freight-cost-service/internal/platform/respond"
	"github.com/freight-platform/freight-cost-service/internal/security"
	"github.com/freight-platform/freight-cost-service/internal/service"
)

type AnalyticsPublicHandler struct {
	analytics *service.AnalyticsPublicService
}

func NewAnalyticsPublicHandler(analytics *service.AnalyticsPublicService) *AnalyticsPublicHandler {
	return &AnalyticsPublicHandler{analytics: analytics}
}

func (h *AnalyticsPublicHandler) Overview(w http.ResponseWriter, r *http.Request) {
	actor, err := security.ParseTrustedActor(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if _, err := ParseTrustedTenant(r); err != nil {
		respond.Error(w, err)
		return
	}
	result, err := h.analytics.Overview(r.Context(), actor, r.URL.Query())
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toOverviewResponse(result))
}

func (h *AnalyticsPublicHandler) ListLanes(w http.ResponseWriter, r *http.Request) {
	actor, err := security.ParseTrustedActor(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if _, err := ParseTrustedTenant(r); err != nil {
		respond.Error(w, err)
		return
	}
	result, err := h.analytics.ListLanes(r.Context(), actor, r.URL.Query())
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toListEnvelope(result))
}

func (h *AnalyticsPublicHandler) ListCarriers(w http.ResponseWriter, r *http.Request) {
	actor, err := security.ParseTrustedActor(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if _, err := ParseTrustedTenant(r); err != nil {
		respond.Error(w, err)
		return
	}
	result, err := h.analytics.ListCarriers(r.Context(), actor, r.URL.Query())
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toListEnvelope(result))
}

func (h *AnalyticsPublicHandler) ListAccessorials(w http.ResponseWriter, r *http.Request) {
	actor, err := security.ParseTrustedActor(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if _, err := ParseTrustedTenant(r); err != nil {
		respond.Error(w, err)
		return
	}
	result, err := h.analytics.ListAccessorials(r.Context(), actor, r.URL.Query())
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toListEnvelope(result))
}

func (h *AnalyticsPublicHandler) ListOpportunities(w http.ResponseWriter, r *http.Request) {
	actor, err := security.ParseTrustedActor(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	if _, err := ParseTrustedTenant(r); err != nil {
		respond.Error(w, err)
		return
	}
	result, err := h.analytics.ListOpportunities(r.Context(), actor, r.URL.Query())
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toListEnvelope(result))
}
