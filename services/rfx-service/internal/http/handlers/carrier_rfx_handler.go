package handlers

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/platform/respond"
)

func (h *RfxHandler) ListCarrierInvitedEvents(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}

	filter := domain.ListCarrierInvitedEventsFilter{
		TenantID: actor.TenantID,
		Limit:    parseLimit(r),
		Offset:   parseOffset(r),
	}
	carrierCompanyID, err := parseCarrierCompanyIDQuery(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	filter.CarrierCompanyID = carrierCompanyID

	if raw := strings.TrimSpace(r.URL.Query().Get("status")); raw != "" {
		filter.Status = &raw
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("response_filter")); raw != "" {
		filter.ResponseFilter = &raw
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("search")); raw != "" {
		filter.Search = &raw
	}

	events, total, err := h.service.ListCarrierInvitedEvents(r.Context(), actor, filter)
	if err != nil {
		respond.Error(w, err)
		return
	}

	items := make([]map[string]any, 0, len(events))
	for i := range events {
		items = append(items, toCarrierInvitedEventResponse(&events[i]))
	}

	respond.JSON(w, http.StatusOK, map[string]any{
		"items":  items,
		"total":  total,
		"limit":  filter.Limit,
		"offset": filter.Offset,
	})
}

func (h *RfxHandler) GetOwnResponse(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}

	eventID, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	carrierCompanyID, err := parseCarrierCompanyIDQuery(r)
	if err != nil {
		respond.Error(w, err)
		return
	}

	response, err := h.service.GetOwnResponse(r.Context(), actor, eventID, carrierCompanyID)
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, toRfxResponseResponse(response))
}

func (h *RfxHandler) GetResponse(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}

	responseID, err := domain.ParseUUID(chi.URLParam(r, "response_id"), "response_id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	response, err := h.service.GetResponse(r.Context(), actor, responseID)
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, toRfxResponseResponse(response))
}

func (h *RfxHandler) ListLanes(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}

	lotID, err := domain.ParseUUID(chi.URLParam(r, "lot_id"), "lot_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	carrierCompanyID, err := parseOptionalCarrierCompanyIDQuery(r)
	if err != nil {
		respond.Error(w, err)
		return
	}

	lanes, err := h.service.ListLanes(r.Context(), actor, lotID, carrierCompanyID)
	if err != nil {
		respond.Error(w, err)
		return
	}

	items := make([]map[string]any, 0, len(lanes))
	for i := range lanes {
		items = append(items, toRfxLaneResponse(&lanes[i]))
	}

	respond.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *RfxHandler) GetCarrierParticipant(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}

	eventID, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	carrierCompanyID, err := parseCarrierCompanyIDQuery(r)
	if err != nil {
		respond.Error(w, err)
		return
	}

	participant, err := h.service.GetCarrierParticipant(r.Context(), actor, eventID, carrierCompanyID)
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, toRfxParticipantResponse(participant))
}

func parseCarrierCompanyIDQuery(r *http.Request) (uuid.UUID, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("carrier_company_id"))
	if raw == "" {
		return uuid.Nil, nil
	}
	return domain.ParseUUID(raw, "carrier_company_id")
}

func parseOptionalCarrierCompanyIDQuery(r *http.Request) (uuid.UUID, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("carrier_company_id"))
	if raw == "" {
		return uuid.Nil, nil
	}
	return domain.ParseUUID(raw, "carrier_company_id")
}

func toCarrierInvitedEventResponse(item *domain.CarrierInvitedRfxEvent) map[string]any {
	resp := toRfxEventResponse(&item.Event)
	resp["participant_status"] = item.ParticipantStatus
	resp["own_response_status"] = item.OwnResponseStatus
	resp["lot_count"] = item.LotCount
	resp["participant_company_id"] = item.ParticipantCompanyID.String()
	if item.OwnResponseID != nil {
		resp["own_response_id"] = item.OwnResponseID.String()
	}
	return resp
}

func parseCarrierCompanyIDRequired(r *http.Request) (uuid.UUID, error) {
	id, err := parseCarrierCompanyIDQuery(r)
	if err != nil {
		return uuid.Nil, err
	}
	if id == uuid.Nil {
		return uuid.Nil, apperrors.Validation("carrier_company_id is required when user belongs to multiple carrier companies", map[string]any{"field": "carrier_company_id"})
	}
	return id, nil
}
