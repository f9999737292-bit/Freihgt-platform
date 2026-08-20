package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/contract-rate-service/internal/domain"
	"github.com/freight-platform/contract-rate-service/internal/platform/respond"
	"github.com/freight-platform/contract-rate-service/internal/service"
)

type RateLineHandler struct {
	svc    *service.RateLineService
	actors *ActorResolver
}

func NewRateLineHandler(svc *service.RateLineService, actors *ActorResolver) *RateLineHandler {
	return &RateLineHandler{svc: svc, actors: actors}
}

func (h *RateLineHandler) Create(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actors.FromRequest(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	versionID, err := parsePathUUID(chi.URLParam(r, "versionId"), "version_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req struct {
		OriginLocationID      uuid.UUID `json:"origin_location_id"`
		DestinationLocationID uuid.UUID `json:"destination_location_id"`
		EquipmentType         string    `json:"equipment_type"`
		TransportMode         string    `json:"transport_mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, domainValidation("invalid request body"))
		return
	}
	created, err := h.svc.Create(r.Context(), domain.CreateRateLineInput{
		TenantID: actor.TenantID, RateCardVersionID: versionID,
		OriginLocationID: req.OriginLocationID, DestinationLocationID: req.DestinationLocationID,
		EquipmentType: req.EquipmentType, TransportMode: req.TransportMode, Actor: actor,
	}, CorrelationID(r))
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, mapRateLine(created))
}

func (h *RateLineHandler) List(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actors.FromRequest(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	versionID, err := parsePathUUID(chi.URLParam(r, "versionId"), "version_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	items, err := h.svc.ListByVersion(r.Context(), actor.TenantID, versionID, actor)
	if err != nil {
		respond.Error(w, err)
		return
	}
	out := make([]any, 0, len(items))
	for i := range items {
		out = append(out, mapRateLine(&items[i]))
	}
	respond.JSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *RateLineHandler) Get(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actors.FromRequest(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	lineID, err := parsePathUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	item, err := h.svc.Get(r.Context(), actor.TenantID, lineID, actor)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, mapRateLine(item))
}

func (h *RateLineHandler) Patch(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actors.FromRequest(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	lineID, err := parsePathUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req struct {
		OriginLocationID      *uuid.UUID `json:"origin_location_id"`
		DestinationLocationID *uuid.UUID `json:"destination_location_id"`
		EquipmentType         *string    `json:"equipment_type"`
		TransportMode         *string    `json:"transport_mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, domainValidation("invalid request body"))
		return
	}
	updated, err := h.svc.Update(r.Context(), actor.TenantID, lineID, domain.UpdateRateLineInput{
		OriginLocationID: req.OriginLocationID, DestinationLocationID: req.DestinationLocationID,
		EquipmentType: req.EquipmentType, TransportMode: req.TransportMode, Actor: actor,
	}, CorrelationID(r))
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, mapRateLine(updated))
}

func (h *RateLineHandler) Delete(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actors.FromRequest(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	lineID, err := parsePathUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	if err := h.svc.Delete(r.Context(), actor.TenantID, lineID, actor, CorrelationID(r)); err != nil {
		respond.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func mapRateLine(line *domain.RateLine) map[string]any {
	return map[string]any{
		"id": line.ID, "tenant_id": line.TenantID, "rate_card_version_id": line.RateCardVersionID,
		"origin_location_id": line.OriginLocationID, "destination_location_id": line.DestinationLocationID,
		"equipment_type": line.EquipmentType, "transport_mode": line.TransportMode,
		"created_at": line.CreatedAt, "updated_at": line.UpdatedAt,
	}
}
