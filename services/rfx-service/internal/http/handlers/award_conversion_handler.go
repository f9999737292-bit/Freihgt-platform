package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	"github.com/freight-platform/rfx-service/internal/platform/respond"
)

func (h *RfxHandler) ConvertAwardToTransportOrders(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	result, err := h.service.ConvertAwardToTransportOrders(r.Context(), actor, eventID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	status := http.StatusOK
	if result.Created {
		status = http.StatusCreated
	}
	respond.JSON(w, status, toAwardTransportOrdersResponse(result))
}

func (h *RfxHandler) ListAwardTransportOrders(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	eventID, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	items, err := h.service.ListAwardTransportOrders(r.Context(), actor, eventID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{
		"items": toAwardTransportOrderItems(items),
	})
}

func toAwardTransportOrdersResponse(result *domain.ConvertAwardTransportOrdersResult) map[string]any {
	return map[string]any{
		"created": result.Created,
		"items":   toAwardTransportOrderItems(result.Items),
	}
}

func toAwardTransportOrderItems(items []domain.RfxAwardTransportOrder) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for i := range items {
		out = append(out, toAwardTransportOrderItem(&items[i]))
	}
	return out
}

func toAwardTransportOrderItem(item *domain.RfxAwardTransportOrder) map[string]any {
	resp := map[string]any{
		"id":                   item.ID.String(),
		"rfx_event_id":         item.RfxEventID.String(),
		"rfx_award_id":         item.RfxAwardID.String(),
		"rfx_response_id":      item.RfxResponseID.String(),
		"transport_order_id":   item.TransportOrderID.String(),
		"transport_order_status": item.OrderStatus,
		"order_number":         item.OrderNumber,
		"carrier_company_id":   item.CarrierCompanyID.String(),
		"buyer_company_id":     item.BuyerCompanyID.String(),
		"amount":               item.Amount,
		"currency_code":        item.CurrencyCode,
		"converted_at":         item.ConvertedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
	if item.RfxLotID != uuid.Nil {
		resp["rfx_lot_id"] = item.RfxLotID.String()
	}
	if item.RfxLaneID != uuid.Nil {
		resp["rfx_lane_id"] = item.RfxLaneID.String()
	}
	if item.ConvertedBy != nil {
		resp["converted_by"] = item.ConvertedBy.String()
	}
	return resp
}
