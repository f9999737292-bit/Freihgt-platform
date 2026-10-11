package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/tracking-service/internal/domain"
	"github.com/freight-platform/tracking-service/internal/platform/errors"
	"github.com/freight-platform/tracking-service/internal/platform/respond"
	"github.com/freight-platform/tracking-service/internal/repository"
	"github.com/freight-platform/tracking-service/internal/service"
)

type CarrierTrackingHandler struct {
	source *service.CarrierTrackingSource
}

func NewCarrierTrackingHandler(source *service.CarrierTrackingSource) *CarrierTrackingHandler {
	return &CarrierTrackingHandler{source: source}
}

func (h *CarrierTrackingHandler) GetTracking(w http.ResponseWriter, r *http.Request) {
	tenantID, shipmentID, carrierID, ok := carrierTrackingIdentity(w, r)
	if !ok {
		return
	}
	summary, err := h.source.TrackingSummary(r.Context(), tenantID, shipmentID, carrierID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, mapCustomerTrackingSummary(summary))
}

func (h *CarrierTrackingHandler) ListLocations(w http.ResponseWriter, r *http.Request) {
	tenantID, shipmentID, carrierID, ok := carrierTrackingIdentity(w, r)
	if !ok {
		return
	}
	from, to, err := parseTimeRange(r)
	if err != nil {
		respond.Error(w, errors.Validation("invalid time range", map[string]any{"error": err.Error()}))
		return
	}
	limit := parseLimit(r)
	offset := parseOffset(r)
	items, total, err := h.source.LocationHistory(r.Context(), tenantID, shipmentID, carrierID, from, to, limit, offset)
	if err != nil {
		respond.Error(w, err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, mapCustomerLocation(item))
	}
	respond.JSON(w, http.StatusOK, map[string]any{"items": out, "total": total, "limit": limit, "offset": offset})
}

func (h *CarrierTrackingHandler) GetETA(w http.ResponseWriter, r *http.Request) {
	if rejectedCustomerFactQuery(w, r, customerETAFactKeys) {
		return
	}
	tenantID, shipmentID, carrierID, ok := carrierTrackingIdentity(w, r)
	if !ok {
		return
	}
	summary, err := h.source.ETA(r.Context(), tenantID, shipmentID, carrierID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, mapCustomerETASummary(summary))
}

func (h *CarrierTrackingHandler) ListETAHistory(w http.ResponseWriter, r *http.Request) {
	tenantID, shipmentID, carrierID, ok := carrierTrackingIdentity(w, r)
	if !ok {
		return
	}
	targetType := r.URL.Query().Get("targetType")
	if targetType == "" {
		targetType = domain.TargetDelivery
	}
	if targetType != domain.TargetPickup && targetType != domain.TargetDelivery {
		respond.Error(w, errors.Validation("invalid target type", map[string]any{"field": "targetType"}))
		return
	}
	from, to, err := parseTimeRange(r)
	if err != nil {
		respond.Error(w, errors.Validation("invalid time range", map[string]any{"error": err.Error()}))
		return
	}
	limit := parseETALimit(r)
	offset := parseOffset(r)
	items, total, err := h.source.ETAHistory(r.Context(), tenantID, shipmentID, carrierID, targetType, from, to, limit, offset)
	if err != nil {
		respond.Error(w, err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, mapCustomerETAObservation(item))
	}
	respond.JSON(w, http.StatusOK, map[string]any{"items": out, "total": total, "limit": limit, "offset": offset})
}

func (h *CarrierTrackingHandler) GetSlots(w http.ResponseWriter, r *http.Request) {
	if rejectedCustomerFactQuery(w, r, customerSlotFactKeys) {
		return
	}
	tenantID, shipmentID, carrierID, ok := carrierTrackingIdentity(w, r)
	if !ok {
		return
	}
	summary, err := h.source.Slots(r.Context(), tenantID, shipmentID, carrierID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, mapCustomerSlotSummary(summary))
}

func (h *CarrierTrackingHandler) ListSlotHistory(w http.ResponseWriter, r *http.Request) {
	tenantID, shipmentID, carrierID, ok := carrierTrackingIdentity(w, r)
	if !ok {
		return
	}
	slotType := r.URL.Query().Get("slotType")
	if slotType == "" {
		slotType = domain.SlotTypeDelivery
	}
	if slotType != domain.SlotTypePickup && slotType != domain.SlotTypeDelivery {
		respond.Error(w, errors.Validation("invalid slot type", map[string]any{"field": "slotType"}))
		return
	}
	from, to, err := parseTimeRange(r)
	if err != nil {
		respond.Error(w, errors.Validation("invalid time range", map[string]any{"error": err.Error()}))
		return
	}
	limit := parseSlotLimit(r)
	offset := parseOffset(r)
	items, total, err := h.source.SlotHistory(r.Context(), tenantID, shipmentID, carrierID, slotType, from, to, limit, offset)
	if err != nil {
		respond.Error(w, err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, mapCustomerSlotRevision(item))
	}
	respond.JSON(w, http.StatusOK, map[string]any{"items": out, "total": total, "limit": limit, "offset": offset})
}

func carrierTrackingIdentity(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, uuid.UUID, bool) {
	tenantID, err := tenantFromRequest(r)
	if err != nil {
		respond.Error(w, err)
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	shipmentID, err := repository.ParseUUID(chi.URLParam(r, "shipmentId"))
	if err != nil {
		respond.Error(w, errors.Validation("invalid shipment id", nil))
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	raw := r.URL.Query().Get("carrier_company_id")
	if raw == "" {
		respond.Error(w, errors.Validation("carrier company is required", map[string]any{"field": "carrier_company_id"}))
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	carrierID, err := repository.ParseUUID(raw)
	if err != nil || carrierID == uuid.Nil {
		respond.Error(w, errors.Validation("invalid carrier company", map[string]any{"field": "carrier_company_id"}))
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	return tenantID, shipmentID, carrierID, true
}
