package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	apperrors "github.com/freight-platform/transport-order-service/internal/platform/errors"
	"github.com/freight-platform/transport-order-service/internal/platform/respond"
	"github.com/freight-platform/transport-order-service/internal/repository"
)

type analyticsDimensionBatchGetter interface {
	BatchGetAnalyticsDimensions(ctx context.Context, tenantID uuid.UUID, transportOrderIDs []uuid.UUID) ([]repository.TransportOrderAnalyticsDimension, error)
}

type AnalyticsDimensionInternalHandler struct {
	svc analyticsDimensionBatchGetter
}

func NewAnalyticsDimensionInternalHandler(svc analyticsDimensionBatchGetter) *AnalyticsDimensionInternalHandler {
	return &AnalyticsDimensionInternalHandler{svc: svc}
}

type batchAnalyticsDimensionsRequest struct {
	TransportOrderIDs []string `json:"transport_order_ids"`
}

type batchAnalyticsDimensionItem struct {
	TransportOrderID   string  `json:"transport_order_id"`
	OrderNumber        *string `json:"order_number,omitempty"`
	OriginCountry      string  `json:"origin_country"`
	OriginCity         *string `json:"origin_city,omitempty"`
	DestinationCountry string  `json:"destination_country"`
	DestinationCity    *string `json:"destination_city,omitempty"`
	TransportMode      string  `json:"transport_mode"`
	EquipmentType      *string `json:"equipment_type,omitempty"`
}

func (h *AnalyticsDimensionInternalHandler) BatchGetAnalyticsDimensions(w http.ResponseWriter, r *http.Request) {
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req batchAnalyticsDimensionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid request body", map[string]any{"field": "body"}))
		return
	}
	if len(req.TransportOrderIDs) == 0 {
		respond.Error(w, apperrors.Validation("transport_order_ids is required", map[string]any{"field": "transport_order_ids"}))
		return
	}
	if len(req.TransportOrderIDs) > repository.MaxAnalyticsDimensionBatchSize {
		respond.Error(w, apperrors.Validation("transport_order_ids exceeds batch limit", map[string]any{
			"field": "transport_order_ids",
			"max":   repository.MaxAnalyticsDimensionBatchSize,
		}))
		return
	}
	ids := make([]uuid.UUID, 0, len(req.TransportOrderIDs))
	for _, raw := range req.TransportOrderIDs {
		id, err := uuid.Parse(raw)
		if err != nil || id == uuid.Nil {
			respond.Error(w, apperrors.Validation("invalid transport_order_id", map[string]any{"field": "transport_order_ids"}))
			return
		}
		ids = append(ids, id)
	}
	items, err := h.svc.BatchGetAnalyticsDimensions(r.Context(), tenantID, ids)
	if err != nil {
		respond.Error(w, err)
		return
	}
	responseItems := make([]batchAnalyticsDimensionItem, 0, len(items))
	for _, item := range items {
		responseItems = append(responseItems, batchAnalyticsDimensionItem{
			TransportOrderID:   item.TransportOrderID.String(),
			OrderNumber:        item.OrderNumber,
			OriginCountry:      item.OriginCountry,
			OriginCity:         item.OriginCity,
			DestinationCountry: item.DestinationCountry,
			DestinationCity:    item.DestinationCity,
			TransportMode:      item.TransportMode,
			EquipmentType:      item.EquipmentType,
		})
	}
	respond.JSON(w, http.StatusOK, map[string]any{"items": responseItems})
}
