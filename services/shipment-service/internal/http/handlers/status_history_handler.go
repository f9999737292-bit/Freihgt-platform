package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/shipment-service/internal/domain"
	"github.com/freight-platform/shipment-service/internal/platform/respond"
	"github.com/freight-platform/shipment-service/internal/service"
)

type StatusHistoryHandler struct {
	service *service.StatusHistoryService
}

func NewStatusHistoryHandler(svc *service.StatusHistoryService) *StatusHistoryHandler {
	return &StatusHistoryHandler{service: svc}
}

func (h *StatusHistoryHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}

	shipmentID, err := domain.ParseUUID(chi.URLParam(r, "shipmentId"), "shipmentId")
	if err != nil {
		respond.Error(w, err)
		return
	}

	page := parsePositiveIntDefault(r.URL.Query().Get("page"), 1)
	limit := parsePositiveIntDefault(r.URL.Query().Get("limit"), 50)
	order := strings.TrimSpace(r.URL.Query().Get("order"))

	filter, err := service.ParseStatusHistoryListFilter(tenantID, shipmentID, page, limit, order)
	if err != nil {
		respond.Error(w, err)
		return
	}

	result, err := h.service.List(r.Context(), filter)
	if err != nil {
		respond.Error(w, err)
		return
	}

	items := make([]map[string]any, 0, len(result.Items))
	for i := range result.Items {
		items = append(items, toStatusHistoryItemResponse(&result.Items[i]))
	}

	respond.JSON(w, http.StatusOK, map[string]any{
		"shipment": map[string]any{
			"id":            result.Shipment.ID.String(),
			"number":        result.Shipment.ShipmentNumber,
			"currentStatus": result.Shipment.Status,
		},
		"complete": result.Complete,
		"items":    items,
		"page":     result.Page,
		"limit":    result.Limit,
		"total":    result.Total,
		"hasNext":  result.HasNext,
		"warnings": result.Warnings,
	})
}

func toStatusHistoryItemResponse(item *domain.ShipmentStatusHistory) map[string]any {
	response := map[string]any{
		"id":              item.ID.String(),
		"shipmentId":      item.ShipmentID.String(),
		"shipmentVersion": item.ShipmentVersion,
		"toStatus":        item.ToStatus,
		"source":          item.Source,
		"actor": map[string]any{
			"type": string(item.ActorType),
		},
		"occurredAt": item.OccurredAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		"recordedAt": item.RecordedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	}
	if item.FromStatus != nil {
		response["fromStatus"] = *item.FromStatus
	}
	if item.ReasonCode != nil {
		response["reasonCode"] = *item.ReasonCode
	}
	if item.ActorID != nil {
		response["actor"] = map[string]any{
			"type": string(item.ActorType),
			"id":   item.ActorID.String(),
		}
	}
	if item.CorrelationID != nil {
		response["correlationId"] = *item.CorrelationID
	}
	return response
}

func parsePositiveIntDefault(raw string, fallback int) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
