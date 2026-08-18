package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
	"github.com/freight-platform/shipment-service/internal/platform/respond"
	"github.com/freight-platform/shipment-service/internal/repository"
	"github.com/freight-platform/shipment-service/internal/service"
)

type OrderExecutionHandler struct {
	service *service.OrderExecutionService
}

func NewOrderExecutionHandler(svc *service.OrderExecutionService) *OrderExecutionHandler {
	return &OrderExecutionHandler{service: svc}
}

type executeTransportOrderRequest struct {
	ShipmentNumber    string  `json:"shipment_number"`
	PlannedPickupAt   *string `json:"planned_pickup_at"`
	PlannedDeliveryAt *string `json:"planned_delivery_at"`
}

func (h *OrderExecutionHandler) Execute(w http.ResponseWriter, r *http.Request) {
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	transition, err := resolveUserStatusTransitionContext(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	orderID, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	carrierCompanyID, err := parseCarrierCompanyIDQuery(r)
	if err != nil {
		respond.Error(w, err)
		return
	}

	var req executeTransportOrderRequest
	if err := decodeStrictJSON(r, &req); err != nil {
		respond.Error(w, err)
		return
	}
	input, err := parseExecuteTransportOrderRequest(req)
	if err != nil {
		respond.Error(w, err)
		return
	}

	result, err := h.service.ExecuteAwardOrder(r.Context(), tenantID, orderID, carrierCompanyID, input, transition)
	if err != nil {
		respond.Error(w, err)
		return
	}
	status := http.StatusCreated
	if !result.Created {
		status = http.StatusOK
	}
	respond.JSON(w, status, toExecuteTransportOrderResponse(result))
}

func (h *OrderExecutionHandler) GetExecution(w http.ResponseWriter, r *http.Request) {
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	orderID, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	companyID, actorKind, err := parseExecutionActor(r)
	if err != nil {
		respond.Error(w, err)
		return
	}

	view, err := h.service.GetExecution(r.Context(), tenantID, orderID, companyID, actorKind)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toOrderExecutionViewResponse(view))
}

func (h *OrderExecutionHandler) ListCarrierOrders(w http.ResponseWriter, r *http.Request) {
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	carrierCompanyID, err := parseCarrierCompanyIDQuery(r)
	if err != nil {
		respond.Error(w, err)
		return
	}

	limit := parseLimit(r)
	offset := parseOffset(r)
	items, total, err := h.service.ListCarrierTransportOrders(r.Context(), domain.ListCarrierTransportOrdersFilter{
		TenantID:         tenantID,
		CarrierCompanyID: carrierCompanyID,
		Limit:            limit,
		Offset:           offset,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}

	responseItems := make([]map[string]any, 0, len(items))
	for i := range items {
		responseItems = append(responseItems, toCarrierTransportOrderItemResponse(&items[i]))
	}
	respond.JSON(w, http.StatusOK, map[string]any{
		"items":  responseItems,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (h *OrderExecutionHandler) ListBuyerOrders(w http.ResponseWriter, r *http.Request) {
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	buyerCompanyID, err := parseBuyerCompanyIDQuery(r)
	if err != nil {
		respond.Error(w, err)
		return
	}

	limit := parseLimit(r)
	offset := parseOffset(r)
	items, total, err := h.service.ListBuyerTransportOrders(r.Context(), domain.ListBuyerTransportOrdersFilter{
		TenantID:       tenantID,
		BuyerCompanyID: buyerCompanyID,
		Limit:          limit,
		Offset:         offset,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}

	responseItems := make([]map[string]any, 0, len(items))
	for i := range items {
		responseItems = append(responseItems, toBuyerTransportOrderItemResponse(&items[i]))
	}
	respond.JSON(w, http.StatusOK, map[string]any{
		"items":  responseItems,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func parseBuyerCompanyIDQuery(r *http.Request) (uuid.UUID, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("buyer_company_id"))
	if raw == "" {
		return uuid.Nil, apperrors.Validation("buyer_company_id is required", map[string]any{"field": "buyer_company_id"})
	}
	return domain.ParseUUID(raw, "buyer_company_id")
}

func toBuyerTransportOrderItemResponse(item *domain.BuyerTransportOrderListItem) map[string]any {
	payload := map[string]any{
		"transport_order_id":     item.Link.TransportOrderID.String(),
		"transport_order_number": item.OrderNumber,
		"transport_order_status": item.OrderStatus,
		"rfx_event_id":           item.Link.RfxEventID.String(),
		"carrier_company_id":     item.Link.CarrierCompanyID.String(),
		"buyer_company_id":       item.Link.BuyerCompanyID.String(),
		"amount":                 item.Link.Amount,
		"currency_code":          item.Link.CurrencyCode,
		"converted_at":           item.Link.ConvertedAt.Format(time.RFC3339),
	}
	if item.Link.RfxLotID != nil {
		payload["rfx_lot_id"] = item.Link.RfxLotID.String()
	}
	if item.ShipmentID != nil {
		payload["shipment_id"] = item.ShipmentID.String()
	}
	if item.ShipmentStatus != nil {
		payload["shipment_status"] = *item.ShipmentStatus
	}
	return payload
}

func (h *OrderExecutionHandler) StartExecution(w http.ResponseWriter, r *http.Request) {
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	transition, err := resolveUserStatusTransitionContext(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	orderID, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	carrierCompanyID, err := parseCarrierCompanyIDQuery(r)
	if err != nil {
		respond.Error(w, err)
		return
	}

	shipment, err := h.service.StartExecution(r.Context(), tenantID, orderID, carrierCompanyID, transition)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toShipmentResponse(shipment))
}

func parseCarrierCompanyIDQuery(r *http.Request) (uuid.UUID, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("carrier_company_id"))
	if raw == "" {
		return uuid.Nil, apperrors.Validation("carrier_company_id is required", map[string]any{"field": "carrier_company_id"})
	}
	return domain.ParseUUID(raw, "carrier_company_id")
}

func parseExecutionActor(r *http.Request) (companyID uuid.UUID, actorKind string, err error) {
	actorKind = strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("actor")))
	companyRaw := strings.TrimSpace(r.URL.Query().Get("company_id"))
	if companyRaw == "" {
		return uuid.Nil, "", apperrors.Validation("company_id is required", map[string]any{"field": "company_id"})
	}
	companyID, err = domain.ParseUUID(companyRaw, "company_id")
	if err != nil {
		return uuid.Nil, "", err
	}
	switch actorKind {
	case domain.ExecutionActorCarrier, domain.ExecutionActorBuyer:
		return companyID, actorKind, nil
	default:
		return uuid.Nil, "", apperrors.Validation("actor must be CARRIER or BUYER", map[string]any{"field": "actor"})
	}
}

func parseExecuteTransportOrderRequest(req executeTransportOrderRequest) (domain.ExecuteTransportOrderInput, error) {
	pickup, err := domain.ParseDateTime(derefString(req.PlannedPickupAt), "planned_pickup_at")
	if err != nil {
		return domain.ExecuteTransportOrderInput{}, err
	}
	delivery, err := domain.ParseDateTime(derefString(req.PlannedDeliveryAt), "planned_delivery_at")
	if err != nil {
		return domain.ExecuteTransportOrderInput{}, err
	}
	return domain.ExecuteTransportOrderInput{
		ShipmentNumber:    req.ShipmentNumber,
		PlannedPickupAt:   pickup,
		PlannedDeliveryAt: delivery,
	}, nil
}

func toExecuteTransportOrderResponse(result *repository.ExecuteAwardOrderResult) map[string]any {
	if result == nil {
		return map[string]any{}
	}
	payload := map[string]any{
		"created":                result.Created,
		"transport_order_id":     result.AwardLink.TransportOrderID.String(),
		"transport_order_number": result.OrderNumber,
		"transport_order_status": result.OrderStatus,
		"rfx_event_id":           result.AwardLink.RfxEventID.String(),
		"rfx_award_id":           result.AwardLink.RfxAwardID.String(),
		"rfx_response_id":        result.AwardLink.RfxResponseID.String(),
		"carrier_company_id":     result.AwardLink.CarrierCompanyID.String(),
		"buyer_company_id":       result.AwardLink.BuyerCompanyID.String(),
		"amount":                 result.AwardLink.Amount,
		"currency_code":          result.AwardLink.CurrencyCode,
	}
	if result.AwardLink.RfxLotID != nil {
		payload["rfx_lot_id"] = result.AwardLink.RfxLotID.String()
	}
	if result.Shipment != nil {
		payload["shipment"] = toShipmentResponse(result.Shipment)
	}
	return payload
}

func toOrderExecutionViewResponse(view *domain.OrderExecutionView) map[string]any {
	if view == nil {
		return map[string]any{}
	}
	payload := map[string]any{
		"transport_order_id":     view.Link.TransportOrderID.String(),
		"transport_order_number": view.OrderNumber,
		"transport_order_status": view.OrderStatus,
		"readiness": map[string]any{
			"carrier_accepted":     view.Readiness.CarrierAccepted,
			"driver_assigned":      view.Readiness.DriverAssigned,
			"vehicle_assigned":     view.Readiness.VehicleAssigned,
			"ready_to_start":       view.Readiness.ReadyToStart,
			"missing_requirements": view.Readiness.MissingRequirements,
		},
		"provenance": map[string]any{
			"rfx_event_id":    view.Provenance.RfxEventID.String(),
			"rfx_award_id":    view.Provenance.RfxAwardID.String(),
			"rfx_response_id": view.Provenance.RfxResponseID.String(),
			"amount":          view.Provenance.Amount,
			"currency_code":   view.Provenance.CurrencyCode,
		},
		"carrier_company_id": view.Link.CarrierCompanyID.String(),
		"buyer_company_id":   view.Link.BuyerCompanyID.String(),
	}
	if view.Link.RfxLotID != nil {
		payload["rfx_lot_id"] = view.Link.RfxLotID.String()
	}
	if view.Provenance.RfxLotID != nil {
		payload["provenance"].(map[string]any)["rfx_lot_id"] = view.Provenance.RfxLotID.String()
	}
	if view.Shipment != nil {
		payload["shipment"] = toShipmentResponse(view.Shipment)
	}
	if len(view.Milestones) > 0 {
		payload["milestones"] = toStatusHistoryItems(view.Milestones)
	}
	if len(view.SLASignals) > 0 {
		payload["sla_signals"] = view.SLASignals
	}
	if len(view.PODDocuments) > 0 {
		payload["pod_documents"] = view.PODDocuments
	}
	if len(view.AllowedActions) > 0 {
		payload["allowed_actions"] = view.AllowedActions
	}
	return payload
}

func toStatusHistoryItems(items []domain.ShipmentStatusHistory) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		entry := map[string]any{
			"id":               item.ID.String(),
			"shipment_version": item.ShipmentVersion,
			"to_status":        item.ToStatus,
			"source":           item.Source,
			"actor_type":       item.ActorType,
			"occurred_at":      item.OccurredAt.Format(time.RFC3339),
			"recorded_at":      item.RecordedAt.Format(time.RFC3339),
		}
		if item.FromStatus != nil {
			entry["from_status"] = *item.FromStatus
		}
		if item.ReasonCode != nil {
			entry["reason_code"] = *item.ReasonCode
		}
		out = append(out, entry)
	}
	return out
}

func toCarrierTransportOrderItemResponse(item *domain.CarrierTransportOrderListItem) map[string]any {
	payload := map[string]any{
		"transport_order_id":     item.Link.TransportOrderID.String(),
		"transport_order_number": item.OrderNumber,
		"transport_order_status": item.OrderStatus,
		"rfx_event_id":           item.Link.RfxEventID.String(),
		"carrier_company_id":     item.Link.CarrierCompanyID.String(),
		"buyer_company_id":       item.Link.BuyerCompanyID.String(),
		"amount":                 item.Link.Amount,
		"currency_code":          item.Link.CurrencyCode,
		"converted_at":           item.Link.ConvertedAt.Format(time.RFC3339),
	}
	if item.Link.RfxLotID != nil {
		payload["rfx_lot_id"] = item.Link.RfxLotID.String()
	}
	if item.ShipmentID != nil {
		payload["shipment_id"] = item.ShipmentID.String()
	}
	if item.ShipmentStatus != nil {
		payload["shipment_status"] = *item.ShipmentStatus
	}
	return payload
}
