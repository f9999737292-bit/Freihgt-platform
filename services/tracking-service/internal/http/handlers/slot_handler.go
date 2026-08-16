package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/tracking-service/internal/domain"
	"github.com/freight-platform/tracking-service/internal/platform/errors"
	"github.com/freight-platform/tracking-service/internal/platform/respond"
	"github.com/freight-platform/tracking-service/internal/repository"
	"github.com/freight-platform/tracking-service/internal/service"
)

type SlotHandler struct {
	query *service.SlotQueryService
}

func NewSlotHandler(query *service.SlotQueryService) *SlotHandler {
	return &SlotHandler{query: query}
}

func (h *SlotHandler) GetCurrent(w http.ResponseWriter, r *http.Request) {
	tenantID, err := tenantFromRequest(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	shipmentID, err := repository.ParseUUID(chi.URLParam(r, "shipmentId"))
	if err != nil {
		respond.Error(w, errors.Validation("invalid shipment id", nil))
		return
	}
	milestone := parseSlotMilestoneContext(r)
	summary, err := h.query.GetShipmentSlots(r.Context(), tenantID, shipmentID, milestone)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, mapSlotSummary(summary))
}

func (h *SlotHandler) ListHistory(w http.ResponseWriter, r *http.Request) {
	tenantID, err := tenantFromRequest(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	shipmentID, err := repository.ParseUUID(chi.URLParam(r, "shipmentId"))
	if err != nil {
		respond.Error(w, errors.Validation("invalid shipment id", nil))
		return
	}
	slotType := r.URL.Query().Get("slotType")
	if slotType == "" {
		slotType = domain.SlotTypeDelivery
	}
	from, to, err := parseTimeRange(r)
	if err != nil {
		respond.Error(w, errors.Validation("invalid time range", map[string]any{"error": err.Error()}))
		return
	}
	limit := parseSlotLimit(r)
	offset := parseOffset(r)
	items, total, err := h.query.ListSlotHistory(r.Context(), tenantID, shipmentID, slotType, from, to, limit, offset)
	if err != nil {
		respond.Error(w, err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, mapSlotRevision(item))
	}
	respond.JSON(w, http.StatusOK, map[string]any{
		"items":  out,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

type slotLookupRequest struct {
	ShipmentIDs []string                         `json:"shipmentIds"`
	Context     map[string]slotMilestonePayload  `json:"context"`
}

type slotMilestonePayload struct {
	ShipmentStatus   string           `json:"shipmentStatus"`
	ActualPickupAt   *string          `json:"actualPickupAt"`
	ActualDeliveryAt *string          `json:"actualDeliveryAt"`
	PickupETA        *etaSnapshotDTO  `json:"pickupEta"`
	DeliveryETA      *etaSnapshotDTO  `json:"deliveryEta"`
}

type etaSnapshotDTO struct {
	HasUsableETA       bool    `json:"hasUsableEta"`
	Status             string  `json:"status"`
	FreshnessStatus    string  `json:"freshnessStatus"`
	QualityStatus      string  `json:"qualityStatus"`
	EstimatedArrivalAt *string `json:"estimatedArrivalAt"`
}

type SlotInternalHandler struct {
	*SlotHandler
	token string
}

func NewSlotInternalHandler(query *service.SlotQueryService, token string) *SlotInternalHandler {
	return &SlotInternalHandler{SlotHandler: NewSlotHandler(query), token: token}
}

func (h *SlotInternalHandler) Lookup(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Internal-Service-Token") != h.token {
		respond.Error(w, errors.Unauthorized("internal authentication failed"))
		return
	}
	tenantID, err := tenantFromRequest(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req slotLookupRequest
	if err := decodeJSON(r, &req); err != nil {
		respond.Error(w, errors.Validation("invalid request body", nil))
		return
	}
	ids := make([]uuid.UUID, 0, len(req.ShipmentIDs))
	contextByShipment := make(map[uuid.UUID]service.SlotMilestoneContext, len(req.ShipmentIDs))
	for _, raw := range req.ShipmentIDs {
		id, parseErr := repository.ParseUUID(raw)
		if parseErr != nil {
			continue
		}
		ids = append(ids, id)
		if payload, ok := req.Context[raw]; ok {
			contextByShipment[id] = parseSlotMilestonePayload(payload)
		}
	}
	summaries, err := h.query.LookupSlotSummaries(r.Context(), tenantID, ids, contextByShipment)
	if err != nil {
		respond.Error(w, err)
		return
	}
	items := make(map[string]any, len(summaries))
	for id, summary := range summaries {
		items[id.String()] = mapSlotSummary(summary)
	}
	respond.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func parseSlotMilestoneContext(r *http.Request) service.SlotMilestoneContext {
	return parseSlotMilestonePayload(slotMilestonePayload{
		ShipmentStatus:   r.URL.Query().Get("shipmentStatus"),
		ActualPickupAt:   optionalQueryTime(r, "actualPickupAt"),
		ActualDeliveryAt: optionalQueryTime(r, "actualDeliveryAt"),
		PickupETA:        parseETASnapshotQuery(r, "pickup"),
		DeliveryETA:      parseETASnapshotQuery(r, "delivery"),
	})
}

func parseETASnapshotQuery(r *http.Request, prefix string) *etaSnapshotDTO {
	status := r.URL.Query().Get(prefix + "EtaStatus")
	if status == "" {
		return nil
	}
	dto := &etaSnapshotDTO{
		Status:          status,
		FreshnessStatus: r.URL.Query().Get(prefix + "EtaFreshness"),
		QualityStatus:   r.URL.Query().Get(prefix + "EtaQuality"),
	}
	if v := r.URL.Query().Get(prefix + "EstimatedArrivalAt"); v != "" {
		dto.EstimatedArrivalAt = &v
	}
	dto.HasUsableETA = status == domain.ETAStatusAvailable || status == domain.ETAStatusStale
	return dto
}

func parseSlotMilestonePayload(p slotMilestonePayload) service.SlotMilestoneContext {
	out := service.SlotMilestoneContext{ShipmentStatus: p.ShipmentStatus}
	out.ActualPickupAt = parseOptionalRFC3339(p.ActualPickupAt)
	out.ActualDeliveryAt = parseOptionalRFC3339(p.ActualDeliveryAt)
	if p.PickupETA != nil {
		out.PickupETA = mapETASnapshotDTO(*p.PickupETA)
	}
	if p.DeliveryETA != nil {
		out.DeliveryETA = mapETASnapshotDTO(*p.DeliveryETA)
	}
	return out
}

func mapETASnapshotDTO(d etaSnapshotDTO) domain.ETASnapshot {
	s := domain.ETASnapshot{
		HasUsableETA:    d.HasUsableETA,
		Status:          d.Status,
		FreshnessStatus: d.FreshnessStatus,
		QualityStatus:   d.QualityStatus,
	}
	s.EstimatedArrivalAt = parseOptionalRFC3339(d.EstimatedArrivalAt)
	if !d.HasUsableETA && s.EstimatedArrivalAt != nil {
		s.HasUsableETA = d.Status == domain.ETAStatusAvailable || d.Status == domain.ETAStatusStale
	}
	return s
}

func parseSlotLimit(r *http.Request) int {
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	if limit > domain.MaxSlotHistoryLimit {
		limit = domain.MaxSlotHistoryLimit
	}
	if limit <= 0 {
		limit = 50
	}
	return limit
}

func mapSlotSummary(summary domain.ShipmentSlotSummary) map[string]any {
	payload := map[string]any{"shipmentId": summary.ShipmentID.String()}
	if summary.Pickup != nil {
		payload["pickup"] = mapSlotTarget(*summary.Pickup)
	}
	if summary.Delivery != nil {
		payload["delivery"] = mapSlotTarget(*summary.Delivery)
	}
	return payload
}

func mapSlotTarget(t domain.SlotTargetSummary) map[string]any {
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
	if t.Provider != nil {
		payload["provider"] = *t.Provider
	}
	if t.ProviderSlotID != nil {
		payload["providerSlotId"] = *t.ProviderSlotID
	}
	if t.SourceObservedAt != nil {
		payload["sourceObservedAt"] = t.SourceObservedAt.UTC().Format(time.RFC3339)
	}
	if t.BookedAt != nil {
		payload["bookedAt"] = t.BookedAt.UTC().Format(time.RFC3339)
	}
	if t.ConfirmedAt != nil {
		payload["confirmedAt"] = t.ConfirmedAt.UTC().Format(time.RFC3339)
	}
	if t.ProjectedLateBySeconds != nil {
		payload["projectedLateBySeconds"] = *t.ProjectedLateBySeconds
	}
	if t.EarlyBySeconds != nil {
		payload["earlyBySeconds"] = *t.EarlyBySeconds
	}
	if t.MarginSeconds != nil {
		payload["marginSeconds"] = *t.MarginSeconds
	}
	return payload
}

func mapSlotRevision(rev domain.SlotRevision) map[string]any {
	payload := map[string]any{
		"id":               rev.ID.String(),
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
	if rev.ProviderCode != nil {
		payload["provider"] = *rev.ProviderCode
	}
	if rev.ProviderSlotID != nil {
		payload["providerSlotId"] = *rev.ProviderSlotID
	}
	if len(rev.QualityReasons) > 0 {
		payload["qualityReasons"] = rev.QualityReasons
	}
	return payload
}
