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

type ETAHandler struct {
	query *service.ETAQueryService
}

func NewETAHandler(query *service.ETAQueryService) *ETAHandler {
	return &ETAHandler{query: query}
}

func (h *ETAHandler) GetCurrent(w http.ResponseWriter, r *http.Request) {
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
	planned := parsePlannedTimes(r)
	summary, err := h.query.GetShipmentETA(r.Context(), tenantID, shipmentID, planned)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, mapETASummary(summary))
}

func (h *ETAHandler) ListHistory(w http.ResponseWriter, r *http.Request) {
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
	targetType := r.URL.Query().Get("targetType")
	if targetType == "" {
		targetType = domain.TargetDelivery
	}
	from, to, err := parseTimeRange(r)
	if err != nil {
		respond.Error(w, errors.Validation("invalid time range", map[string]any{"error": err.Error()}))
		return
	}
	limit := parseETALimit(r)
	offset := parseOffset(r)
	items, total, err := h.query.ListETAHistory(r.Context(), tenantID, shipmentID, targetType, from, to, limit, offset)
	if err != nil {
		respond.Error(w, err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, mapETAObservation(item))
	}
	respond.JSON(w, http.StatusOK, map[string]any{
		"items":  out,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

type etaLookupRequest struct {
	ShipmentIDs []string                       `json:"shipmentIds"`
	Planned     map[string]plannedTimesPayload `json:"planned"`
}

type plannedTimesPayload struct {
	PlannedPickupAt   *string `json:"plannedPickupAt"`
	PlannedDeliveryAt *string `json:"plannedDeliveryAt"`
	ActualDeliveryAt  *string `json:"actualDeliveryAt"`
	ActualPickupAt    *string `json:"actualPickupAt"`
	ShipmentStatus    string  `json:"shipmentStatus"`
}

type ETAInternalHandler struct {
	*ETAHandler
	token string
}

func NewETAInternalHandler(query *service.ETAQueryService, token string) *ETAInternalHandler {
	return &ETAInternalHandler{ETAHandler: NewETAHandler(query), token: token}
}

func (h *ETAInternalHandler) LookupDelivery(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Internal-Service-Token") != h.token {
		respond.Error(w, errors.Unauthorized("internal authentication failed"))
		return
	}
	tenantID, err := tenantFromRequest(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req etaLookupRequest
	if err := decodeJSON(r, &req); err != nil {
		respond.Error(w, errors.Validation("invalid request body", nil))
		return
	}
	ids := make([]uuid.UUID, 0, len(req.ShipmentIDs))
	planned := make(map[uuid.UUID]service.PlannedTimes, len(req.ShipmentIDs))
	for _, raw := range req.ShipmentIDs {
		id, parseErr := repository.ParseUUID(raw)
		if parseErr != nil {
			continue
		}
		ids = append(ids, id)
		if p, ok := req.Planned[raw]; ok {
			planned[id] = parsePlannedPayload(p)
		}
	}
	states, err := h.query.LookupDeliveryETA(r.Context(), tenantID, ids, planned)
	if err != nil {
		respond.Error(w, err)
		return
	}
	items := make(map[string]any, len(states))
	for id, summary := range states {
		items[id.String()] = mapETATarget(summary)
	}
	respond.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func parsePlannedTimes(r *http.Request) service.PlannedTimes {
	return parsePlannedPayload(plannedTimesPayload{
		PlannedPickupAt:   optionalQueryTime(r, "plannedPickupAt"),
		PlannedDeliveryAt: optionalQueryTime(r, "plannedDeliveryAt"),
		ActualDeliveryAt:  optionalQueryTime(r, "actualDeliveryAt"),
		ActualPickupAt:    optionalQueryTime(r, "actualPickupAt"),
		ShipmentStatus:    r.URL.Query().Get("shipmentStatus"),
	})
}

func optionalQueryTime(r *http.Request, key string) *string {
	v := r.URL.Query().Get(key)
	if v == "" {
		return nil
	}
	return &v
}

func parsePlannedPayload(p plannedTimesPayload) service.PlannedTimes {
	out := service.PlannedTimes{ShipmentStatus: p.ShipmentStatus}
	out.PlannedPickupAt = parseOptionalRFC3339(p.PlannedPickupAt)
	out.PlannedDeliveryAt = parseOptionalRFC3339(p.PlannedDeliveryAt)
	out.ActualDeliveryAt = parseOptionalRFC3339(p.ActualDeliveryAt)
	out.ActualPickupAt = parseOptionalRFC3339(p.ActualPickupAt)
	return out
}

func parseOptionalRFC3339(raw *string) *time.Time {
	if raw == nil || *raw == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, *raw)
	if err != nil {
		return nil
	}
	utc := t.UTC()
	return &utc
}

func parseETALimit(r *http.Request) int {
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	if limit > domain.MaxETAHistoryLimit {
		limit = domain.MaxETAHistoryLimit
	}
	if limit <= 0 {
		limit = 50
	}
	return limit
}

func mapETASummary(summary domain.ShipmentETASummary) map[string]any {
	payload := map[string]any{"shipmentId": summary.ShipmentID.String()}
	if summary.Delivery != nil {
		payload["delivery"] = mapETATarget(*summary.Delivery)
	}
	if summary.Pickup != nil {
		payload["pickup"] = mapETATarget(*summary.Pickup)
	}
	return payload
}

func mapETATarget(t domain.ETATargetSummary) map[string]any {
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
	if t.Provider != nil {
		payload["provider"] = *t.Provider
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
	if t.ProviderConfidence != nil {
		payload["providerConfidence"] = *t.ProviderConfidence
	}
	return payload
}

func mapETAObservation(o domain.ETAObservation) map[string]any {
	payload := map[string]any{
		"id":                 o.ID.String(),
		"shipmentId":         o.ShipmentID.String(),
		"targetType":         o.TargetType,
		"estimatedArrivalAt": o.EstimatedArrivalAt.UTC().Format(time.RFC3339),
		"sourceType":         o.SourceType,
		"sourceObservedAt":   o.SourceObservedAt.UTC().Format(time.RFC3339),
		"receivedAt":         o.ReceivedAt.UTC().Format(time.RFC3339),
		"qualityStatus":      o.QualityStatus,
	}
	if o.ProviderCode != nil {
		payload["provider"] = *o.ProviderCode
	}
	if o.ProviderEventID != nil {
		payload["providerEventId"] = *o.ProviderEventID
	}
	if len(o.QualityReasons) > 0 {
		payload["qualityReasons"] = o.QualityReasons
	}
	if o.ProviderConfidence != nil {
		payload["providerConfidence"] = *o.ProviderConfidence
	}
	return payload
}
