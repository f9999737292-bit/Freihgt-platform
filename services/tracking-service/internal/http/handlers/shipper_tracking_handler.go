package handlers

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/tracking-service/internal/domain"
	"github.com/freight-platform/tracking-service/internal/platform/errors"
	"github.com/freight-platform/tracking-service/internal/platform/respond"
	"github.com/freight-platform/tracking-service/internal/repository"
	"github.com/freight-platform/tracking-service/internal/service"
)

type ShipperTrackingHandler struct {
	source *service.ShipperTrackingSource
}

func NewShipperTrackingHandler(source *service.ShipperTrackingSource) *ShipperTrackingHandler {
	return &ShipperTrackingHandler{source: source}
}

func (h *ShipperTrackingHandler) GetTracking(w http.ResponseWriter, r *http.Request) {
	tenantID, shipmentID, shipperID, ok := shipperTrackingIdentity(w, r)
	if !ok {
		return
	}
	summary, err := h.source.TrackingSummary(r.Context(), tenantID, shipmentID, shipperID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, mapCustomerTrackingSummary(summary))
}

func (h *ShipperTrackingHandler) ListLocations(w http.ResponseWriter, r *http.Request) {
	tenantID, shipmentID, shipperID, ok := shipperTrackingIdentity(w, r)
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
	items, total, err := h.source.LocationHistory(r.Context(), tenantID, shipmentID, shipperID, from, to, limit, offset)
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

func (h *ShipperTrackingHandler) GetETA(w http.ResponseWriter, r *http.Request) {
	if rejectedCustomerFactQuery(w, r, customerETAFactKeys) {
		return
	}
	tenantID, shipmentID, shipperID, ok := shipperTrackingIdentity(w, r)
	if !ok {
		return
	}
	summary, err := h.source.ETA(r.Context(), tenantID, shipmentID, shipperID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, mapCustomerETASummary(summary))
}

func (h *ShipperTrackingHandler) ListETAHistory(w http.ResponseWriter, r *http.Request) {
	tenantID, shipmentID, shipperID, ok := shipperTrackingIdentity(w, r)
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
	items, total, err := h.source.ETAHistory(r.Context(), tenantID, shipmentID, shipperID, targetType, from, to, limit, offset)
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

func (h *ShipperTrackingHandler) GetSlots(w http.ResponseWriter, r *http.Request) {
	if rejectedCustomerFactQuery(w, r, customerSlotFactKeys) {
		return
	}
	tenantID, shipmentID, shipperID, ok := shipperTrackingIdentity(w, r)
	if !ok {
		return
	}
	summary, err := h.source.Slots(r.Context(), tenantID, shipmentID, shipperID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, mapCustomerSlotSummary(summary))
}

func (h *ShipperTrackingHandler) ListSlotHistory(w http.ResponseWriter, r *http.Request) {
	tenantID, shipmentID, shipperID, ok := shipperTrackingIdentity(w, r)
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
	items, total, err := h.source.SlotHistory(r.Context(), tenantID, shipmentID, shipperID, slotType, from, to, limit, offset)
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

func shipperTrackingIdentity(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, uuid.UUID, bool) {
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
	raw := r.URL.Query().Get("shipper_company_id")
	if raw == "" {
		respond.Error(w, errors.Validation("shipper company is required", map[string]any{"field": "shipper_company_id"}))
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	shipperID, err := repository.ParseUUID(raw)
	if err != nil || shipperID == uuid.Nil {
		respond.Error(w, errors.Validation("invalid shipper company", map[string]any{"field": "shipper_company_id"}))
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	return tenantID, shipmentID, shipperID, true
}

var customerETAFactKeys = []string{
	"plannedPickupAt", "plannedDeliveryAt", "actualPickupAt", "actualDeliveryAt", "shipmentStatus",
}

var customerSlotFactKeys = []string{
	"shipmentStatus", "actualPickupAt", "actualDeliveryAt",
	"pickupEtaStatus", "deliveryEtaStatus",
	"pickupEstimatedArrivalAt", "deliveryEstimatedArrivalAt",
	"pickupEtaFreshness", "deliveryEtaFreshness",
	"pickupEtaQuality", "deliveryEtaQuality",
}

func rejectedCustomerFactQuery(w http.ResponseWriter, r *http.Request, keys []string) bool {
	for _, key := range keys {
		if _, present := r.URL.Query()[key]; present {
			respond.Error(w, errors.Validation("shipment facts are server owned", map[string]any{"field": key}))
			return true
		}
	}
	return false
}

func mapCustomerTrackingSummary(summary domain.TrackingSummary) map[string]any {
	payload := map[string]any{
		"shipmentId":     summary.ShipmentID.String(),
		"trackingStatus": summary.TrackingStatus,
		"freshness":      map[string]any{"status": summary.Freshness.Status},
		"quality":        map[string]any{"status": summary.Quality.Status},
	}
	if summary.Freshness.AgeSeconds != nil {
		payload["freshness"].(map[string]any)["ageSeconds"] = *summary.Freshness.AgeSeconds
	}
	if summary.LastKnownPosition != nil {
		payload["lastKnownPosition"] = map[string]any{
			"latitude":   summary.LastKnownPosition.Latitude,
			"longitude":  summary.LastKnownPosition.Longitude,
			"recordedAt": summary.LastKnownPosition.RecordedAt.UTC().Format(time.RFC3339),
			"ageSeconds": summary.LastKnownPosition.AgeSeconds,
		}
	}
	if summary.LastRecordedAt != nil {
		payload["lastRecordedAt"] = summary.LastRecordedAt.UTC().Format(time.RFC3339)
	}
	if summary.LastReceivedAt != nil {
		payload["lastReceivedAt"] = summary.LastReceivedAt.UTC().Format(time.RFC3339)
	}
	if summary.SpeedKph != nil {
		payload["speedKph"] = *summary.SpeedKph
	}
	if summary.HeadingDegrees != nil {
		payload["headingDegrees"] = *summary.HeadingDegrees
	}
	if summary.DeliveryDelaySeconds != nil {
		payload["deliveryDelaySeconds"] = *summary.DeliveryDelaySeconds
	}
	return payload
}

func mapCustomerLocation(item domain.LocationEvent) map[string]any {
	payload := map[string]any{
		"shipmentId": item.ShipmentID.String(),
		"latitude":   item.Latitude,
		"longitude":  item.Longitude,
		"recordedAt": item.RecordedAt.UTC().Format(time.RFC3339),
		"receivedAt": item.ReceivedAt.UTC().Format(time.RFC3339),
		"sourceType": item.SourceType,
		"quality":    map[string]any{"status": item.QualityStatus},
	}
	if item.SpeedKph != nil {
		payload["speedKph"] = *item.SpeedKph
	}
	if item.HeadingDegrees != nil {
		payload["headingDegrees"] = *item.HeadingDegrees
	}
	if item.AccuracyMeters != nil {
		payload["accuracyMeters"] = *item.AccuracyMeters
	}
	return payload
}

func mapCustomerETAObservation(o domain.ETAObservation) map[string]any {
	payload := map[string]any{
		"shipmentId":         o.ShipmentID.String(),
		"targetType":         o.TargetType,
		"estimatedArrivalAt": o.EstimatedArrivalAt.UTC().Format(time.RFC3339),
		"sourceType":         o.SourceType,
		"sourceObservedAt":   o.SourceObservedAt.UTC().Format(time.RFC3339),
		"receivedAt":         o.ReceivedAt.UTC().Format(time.RFC3339),
		"qualityStatus":      o.QualityStatus,
	}
	if len(o.QualityReasons) > 0 {
		payload["qualityReasons"] = o.QualityReasons
	}
	return payload
}

func mapCustomerETASummary(summary domain.ShipmentETASummary) map[string]any {
	payload := map[string]any{"shipmentId": summary.ShipmentID.String()}
	if summary.Pickup != nil {
		payload["pickup"] = mapCustomerETATarget(*summary.Pickup)
	}
	if summary.Delivery != nil {
		payload["delivery"] = mapCustomerETATarget(*summary.Delivery)
	}
	return payload
}

func mapCustomerETATarget(t domain.ETATargetSummary) map[string]any {
	payload := map[string]any{
		"status":            t.Status,
		"freshnessStatus":   t.FreshnessStatus,
		"qualityStatus":     t.QualityStatus,
		"arrivalProjection": t.ArrivalProjection,
	}
	if t.EstimatedArrivalAt != nil {
		payload["estimatedArrivalAt"] = t.EstimatedArrivalAt.UTC().Format(time.RFC3339)
	}
	if t.SourceType != nil {
		payload["sourceType"] = *t.SourceType
	}
	if t.SourceObservedAt != nil {
		payload["sourceObservedAt"] = t.SourceObservedAt.UTC().Format(time.RFC3339)
	}
	if t.ReceivedAt != nil {
		payload["receivedAt"] = t.ReceivedAt.UTC().Format(time.RFC3339)
	}
	if t.AgeSeconds != nil {
		payload["ageSeconds"] = *t.AgeSeconds
	}
	if t.DeliveryLagSeconds != nil {
		payload["deliveryLagSeconds"] = *t.DeliveryLagSeconds
	}
	if t.PlannedArrivalAt != nil {
		payload["plannedArrivalAt"] = t.PlannedArrivalAt.UTC().Format(time.RFC3339)
	}
	if t.ProjectedDeviationSeconds != nil {
		payload["projectedDeviationSeconds"] = *t.ProjectedDeviationSeconds
	}
	if len(t.QualityReasons) > 0 {
		payload["qualityReasons"] = t.QualityReasons
	}
	return payload
}

func mapCustomerSlotSummary(summary domain.ShipmentSlotSummary) map[string]any {
	payload := map[string]any{"shipmentId": summary.ShipmentID.String()}
	if summary.Pickup != nil {
		payload["pickup"] = mapCustomerSlotTarget(*summary.Pickup)
	}
	if summary.Delivery != nil {
		payload["delivery"] = mapCustomerSlotTarget(*summary.Delivery)
	}
	return payload
}

func mapCustomerSlotTarget(t domain.SlotTargetSummary) map[string]any {
	payload := map[string]any{
		"windowStatus":      t.WindowStatus,
		"qualityStatus":     t.QualityStatus,
		"arrivalProjection": t.ArrivalProjection,
		"etaRelation":       t.ETARelation,
	}
	if t.SlotStatus != nil {
		payload["slotStatus"] = *t.SlotStatus
	}
	if t.WindowStart != nil {
		payload["windowStart"] = t.WindowStart.UTC().Format(time.RFC3339)
	}
	if t.WindowEnd != nil {
		payload["windowEnd"] = t.WindowEnd.UTC().Format(time.RFC3339)
	}
	if t.Timezone != nil {
		payload["timezone"] = *t.Timezone
	}
	if t.SourceType != nil {
		payload["sourceType"] = *t.SourceType
	}
	if t.BookedAt != nil {
		payload["bookedAt"] = t.BookedAt.UTC().Format(time.RFC3339)
	}
	if t.ConfirmedAt != nil {
		payload["confirmedAt"] = t.ConfirmedAt.UTC().Format(time.RFC3339)
	}
	return payload
}

func mapCustomerSlotRevision(rev domain.SlotRevision) map[string]any {
	payload := map[string]any{
		"shipmentId":       rev.ShipmentID.String(),
		"slotType":         rev.SlotType,
		"windowStart":      rev.WindowStart.UTC().Format(time.RFC3339),
		"windowEnd":        rev.WindowEnd.UTC().Format(time.RFC3339),
		"slotStatus":       rev.SlotStatus,
		"sourceType":       rev.SourceType,
		"sourceObservedAt": rev.SourceObservedAt.UTC().Format(time.RFC3339),
		"receivedAt":       rev.ReceivedAt.UTC().Format(time.RFC3339),
		"qualityStatus":    rev.QualityStatus,
	}
	if rev.Timezone != nil {
		payload["timezone"] = *rev.Timezone
	}
	if len(rev.QualityReasons) > 0 {
		payload["qualityReasons"] = rev.QualityReasons
	}
	return payload
}
