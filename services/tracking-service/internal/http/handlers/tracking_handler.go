package handlers

import (
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/shared-go/lowcode"
	"github.com/freight-platform/tracking-service/internal/domain"
	"github.com/freight-platform/tracking-service/internal/platform/errors"
	"github.com/freight-platform/tracking-service/internal/platform/respond"
	"github.com/freight-platform/tracking-service/internal/provider"
	"github.com/freight-platform/tracking-service/internal/repository"
	"github.com/freight-platform/tracking-service/internal/service"
)

type TrackingHandler struct {
	query *service.TrackingQueryService
}

func NewTrackingHandler(query *service.TrackingQueryService) *TrackingHandler {
	return &TrackingHandler{query: query}
}

func (h *TrackingHandler) GetCurrent(w http.ResponseWriter, r *http.Request) {
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
	summary, err := h.query.GetTrackingSummary(r.Context(), tenantID, shipmentID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, mapSummary(summary))
}

func (h *TrackingHandler) ListLocations(w http.ResponseWriter, r *http.Request) {
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
	from, to, err := parseTimeRange(r)
	if err != nil {
		respond.Error(w, errors.Validation("invalid time range", map[string]any{"error": err.Error()}))
		return
	}
	limit := parseLimit(r)
	offset := parseOffset(r)
	items, total, err := h.query.ListLocationHistory(r.Context(), tenantID, shipmentID, from, to, limit, offset)
	if err != nil {
		respond.Error(w, err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, mapLocation(item))
	}
	respond.JSON(w, http.StatusOK, map[string]any{
		"items":  out,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

type IngestHandler struct {
	ingest *service.IngestService
	secrets map[string]string
}

func NewIngestHandler(ingest *service.IngestService, secrets map[string]string) *IngestHandler {
	return &IngestHandler{ingest: ingest, secrets: secrets}
}

func (h *IngestHandler) Ingest(w http.ResponseWriter, r *http.Request) {
	providerCode := chi.URLParam(r, "provider")
	secret := r.Header.Get("X-Provider-Secret")
	expected, ok := h.secrets[providerCode]
	if !ok || expected == "" || secret == "" || secret != expected {
		respond.Error(w, errors.Unauthorized("provider authentication failed"))
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		respond.Error(w, errors.Validation("invalid request body", nil))
		return
	}
	result, err := h.ingest.IngestProviderLocations(r.Context(), providerCode, provider.ProviderPayload(body))
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, result)
}

type InternalHandler struct {
	query *service.TrackingQueryService
	token string
}

func NewInternalHandler(query *service.TrackingQueryService, token string) *InternalHandler {
	return &InternalHandler{query: query, token: token}
}

type lookupRequest struct {
	ShipmentIDs []string `json:"shipmentIds"`
}

func (h *InternalHandler) LookupStates(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Internal-Service-Token") != h.token {
		respond.Error(w, errors.Unauthorized("internal authentication failed"))
		return
	}
	tenantID, err := tenantFromRequest(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req lookupRequest
	if err := decodeJSON(r, &req); err != nil {
		respond.Error(w, errors.Validation("invalid request body", nil))
		return
	}
	ids := make([]uuid.UUID, 0, len(req.ShipmentIDs))
	for _, raw := range req.ShipmentIDs {
		id, parseErr := repository.ParseUUID(raw)
		if parseErr != nil {
			continue
		}
		ids = append(ids, id)
	}
	states, err := h.query.LookupTrackingStates(r.Context(), tenantID, ids)
	if err != nil {
		respond.Error(w, err)
		return
	}
	items := make(map[string]any, len(states))
	for id, summary := range states {
		items[id.String()] = mapSummary(summary)
	}
	respond.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func tenantFromRequest(r *http.Request) (uuid.UUID, error) {
	raw := lowcode.TenantIDFromHeader(r.Header)
	if raw == "" {
		return uuid.Nil, errors.Validation("tenant id is required", map[string]any{"header": lowcode.HeaderTenantID})
	}
	return repository.ParseUUID(raw)
}

func parseLimit(r *http.Request) int {
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	if limit > domain.MaxLocationHistoryLimit {
		limit = domain.MaxLocationHistoryLimit
	}
	if limit <= 0 {
		limit = 50
	}
	return limit
}

func parseOffset(r *http.Request) int {
	offset := 0
	if raw := r.URL.Query().Get("offset"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			offset = parsed
		}
	}
	if offset < 0 {
		offset = 0
	}
	return offset
}

func parseTimeRange(r *http.Request) (*time.Time, *time.Time, error) {
	var from *time.Time
	var to *time.Time
	if raw := r.URL.Query().Get("from"); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return nil, nil, err
		}
		from = &parsed
	}
	if raw := r.URL.Query().Get("to"); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return nil, nil, err
		}
		to = &parsed
	}
	return from, to, nil
}

func mapSummary(summary domain.TrackingSummary) map[string]any {
	payload := map[string]any{
		"shipmentId":     summary.ShipmentID.String(),
		"trackingStatus": summary.TrackingStatus,
		"freshness": map[string]any{
			"status": summary.Freshness.Status,
		},
		"quality": map[string]any{
			"status": summary.Quality.Status,
		},
	}
	if summary.Provider != nil {
		payload["provider"] = *summary.Provider
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

func mapLocation(item domain.LocationEvent) map[string]any {
	payload := map[string]any{
		"id":               item.ID.String(),
		"shipmentId":       item.ShipmentID.String(),
		"provider":         item.ProviderCode,
		"providerDeviceId": item.ProviderDeviceID,
		"latitude":         item.Latitude,
		"longitude":        item.Longitude,
		"recordedAt":       item.RecordedAt.UTC().Format(time.RFC3339),
		"receivedAt":       item.ReceivedAt.UTC().Format(time.RFC3339),
		"sourceType":       item.SourceType,
		"quality": map[string]any{
			"status": item.QualityStatus,
		},
	}
	if item.ProviderEventID != nil {
		payload["providerEventId"] = *item.ProviderEventID
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
	if item.QualityReason != nil {
		payload["quality"].(map[string]any)["reason"] = *item.QualityReason
	}
	return payload
}
