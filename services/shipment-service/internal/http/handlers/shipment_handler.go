package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/shipment-service/internal/domain"
	"github.com/freight-platform/shipment-service/internal/platform/respond"
	"github.com/freight-platform/shipment-service/internal/service"
)

type ShipmentHandler struct {
	service *service.ShipmentService
}

func NewShipmentHandler(svc *service.ShipmentService) *ShipmentHandler {
	return &ShipmentHandler{service: svc}
}

func (h *ShipmentHandler) Health(w http.ResponseWriter, _ *http.Request) {
	respond.JSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "shipment-service",
	})
}

type createShipmentFromOrderRequest struct {
	ShipmentNumber     string  `json:"shipment_number"`
	TransportOrderID   string  `json:"transport_order_id"`
	CarrierCompanyID   string  `json:"carrier_company_id"`
	ForwarderCompanyID *string `json:"forwarder_company_id"`
	PlannedPickupAt    *string `json:"planned_pickup_at"`
	PlannedDeliveryAt  *string `json:"planned_delivery_at"`
}

type createShipmentFromBidRequest struct {
	ShipmentNumber    string  `json:"shipment_number"`
	BidID             string  `json:"bid_id"`
	TransportOrderID  string  `json:"transport_order_id"`
	PlannedPickupAt   *string `json:"planned_pickup_at"`
	PlannedDeliveryAt *string `json:"planned_delivery_at"`
}

type assignDriverRequest struct {
	DriverID string `json:"driver_id"`
}

type assignVehicleRequest struct {
	VehicleID string `json:"vehicle_id"`
}

type updateShipmentStatusRequest struct {
	Status     string  `json:"status"`
	ActualTime *string `json:"actual_time"`
}

type cancelShipmentRequest struct {
	Reason string `json:"reason"`
}

func (h *ShipmentHandler) CreateFromTransportOrder(w http.ResponseWriter, r *http.Request) {
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
	var req createShipmentFromOrderRequest
	if err := decodeStrictJSON(r, &req); err != nil {
		respond.Error(w, err)
		return
	}
	input, err := parseCreateShipmentFromOrderRequest(req)
	if err != nil {
		respond.Error(w, err)
		return
	}
	shipment, err := h.service.CreateFromTransportOrder(r.Context(), tenantID, input, transition)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toShipmentResponse(shipment))
}

func (h *ShipmentHandler) CreateFromBid(w http.ResponseWriter, r *http.Request) {
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
	var req createShipmentFromBidRequest
	if err := decodeStrictJSON(r, &req); err != nil {
		respond.Error(w, err)
		return
	}
	input, err := parseCreateShipmentFromBidRequest(req)
	if err != nil {
		respond.Error(w, err)
		return
	}
	shipment, err := h.service.CreateFromBid(r.Context(), tenantID, input, transition)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toShipmentResponse(shipment))
}

func (h *ShipmentHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	shipment, err := h.service.GetByIDAndTenant(r.Context(), tenantID, id)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toShipmentResponse(shipment))
}

func (h *ShipmentHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, err := domain.ParseUUID(r.URL.Query().Get("tenant_id"), "tenant_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	filter := domain.ListShipmentsFilter{
		TenantID: tenantID,
		Limit:    parseLimit(r),
		Offset:   parseOffset(r),
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("shipper_company_id")); raw != "" {
		id, err := domain.ParseUUID(raw, "shipper_company_id")
		if err != nil {
			respond.Error(w, err)
			return
		}
		filter.ShipperCompanyID = &id
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("consignee_company_id")); raw != "" {
		id, err := domain.ParseUUID(raw, "consignee_company_id")
		if err != nil {
			respond.Error(w, err)
			return
		}
		filter.ConsigneeCompanyID = &id
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("carrier_company_id")); raw != "" {
		id, err := domain.ParseUUID(raw, "carrier_company_id")
		if err != nil {
			respond.Error(w, err)
			return
		}
		filter.CarrierCompanyID = &id
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("status")); raw != "" {
		filter.Status = &raw
	}

	shipments, total, err := h.service.List(r.Context(), filter)
	if err != nil {
		respond.Error(w, err)
		return
	}
	items := make([]map[string]any, 0, len(shipments))
	for i := range shipments {
		items = append(items, toShipmentResponse(&shipments[i]))
	}
	respond.JSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

func (h *ShipmentHandler) AssignDriver(w http.ResponseWriter, r *http.Request) {
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
	shipmentID, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req assignDriverRequest
	if err := decodeStrictJSON(r, &req); err != nil {
		respond.Error(w, err)
		return
	}
	driverID, err := domain.ParseUUID(req.DriverID, "driver_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	shipment, err := h.service.AssignDriver(r.Context(), tenantID, shipmentID, driverID, transition)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toShipmentResponse(shipment))
}

func (h *ShipmentHandler) AssignVehicle(w http.ResponseWriter, r *http.Request) {
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
	shipmentID, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req assignVehicleRequest
	if err := decodeStrictJSON(r, &req); err != nil {
		respond.Error(w, err)
		return
	}
	vehicleID, err := domain.ParseUUID(req.VehicleID, "vehicle_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	shipment, err := h.service.AssignVehicle(r.Context(), tenantID, shipmentID, vehicleID, transition)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toShipmentResponse(shipment))
}

func (h *ShipmentHandler) Accept(w http.ResponseWriter, r *http.Request) {
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
	shipmentID, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	if r.ContentLength > 0 {
		var discard struct{}
		if err := decodeStrictJSON(r, &discard); err != nil {
			respond.Error(w, err)
			return
		}
	}
	shipment, err := h.service.Accept(r.Context(), tenantID, shipmentID, transition)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toShipmentResponse(shipment))
}

func (h *ShipmentHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
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
	shipmentID, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req updateShipmentStatusRequest
	if err := decodeStrictJSON(r, &req); err != nil {
		respond.Error(w, err)
		return
	}
	actualTime, err := domain.ParseDateTime(derefString(req.ActualTime), "actual_time")
	if err != nil {
		respond.Error(w, err)
		return
	}
	shipment, err := h.service.UpdateStatus(r.Context(), tenantID, shipmentID, domain.UpdateShipmentStatusInput{
		Status: strings.TrimSpace(req.Status), ActualTime: actualTime,
	}, transition)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toShipmentResponse(shipment))
}

func (h *ShipmentHandler) Cancel(w http.ResponseWriter, r *http.Request) {
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
	shipmentID, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req cancelShipmentRequest
	if err := decodeStrictJSON(r, &req); err != nil {
		respond.Error(w, err)
		return
	}
	shipment, err := h.service.Cancel(r.Context(), tenantID, shipmentID, domain.CancelShipmentInput{
		Reason: req.Reason,
	}, transition)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toShipmentResponse(shipment))
}

func parseCreateShipmentFromOrderRequest(req createShipmentFromOrderRequest) (domain.CreateShipmentFromOrderInput, error) {
	transportOrderID, err := domain.ParseUUID(req.TransportOrderID, "transport_order_id")
	if err != nil {
		return domain.CreateShipmentFromOrderInput{}, err
	}
	carrierCompanyID, err := domain.ParseUUID(req.CarrierCompanyID, "carrier_company_id")
	if err != nil {
		return domain.CreateShipmentFromOrderInput{}, err
	}
	forwarderCompanyID, err := domain.ParseOptionalUUID(derefString(req.ForwarderCompanyID), "forwarder_company_id")
	if err != nil {
		return domain.CreateShipmentFromOrderInput{}, err
	}
	plannedPickup, err := domain.ParseDateTime(derefString(req.PlannedPickupAt), "planned_pickup_at")
	if err != nil {
		return domain.CreateShipmentFromOrderInput{}, err
	}
	plannedDelivery, err := domain.ParseDateTime(derefString(req.PlannedDeliveryAt), "planned_delivery_at")
	if err != nil {
		return domain.CreateShipmentFromOrderInput{}, err
	}
	return domain.CreateShipmentFromOrderInput{
		ShipmentNumber:     req.ShipmentNumber,
		TransportOrderID:   transportOrderID,
		CarrierCompanyID:   carrierCompanyID,
		ForwarderCompanyID: forwarderCompanyID,
		PlannedPickupAt:    plannedPickup,
		PlannedDeliveryAt:  plannedDelivery,
	}, nil
}

func parseCreateShipmentFromBidRequest(req createShipmentFromBidRequest) (domain.CreateShipmentFromBidInput, error) {
	bidID, err := domain.ParseUUID(req.BidID, "bid_id")
	if err != nil {
		return domain.CreateShipmentFromBidInput{}, err
	}
	transportOrderID, err := domain.ParseUUID(req.TransportOrderID, "transport_order_id")
	if err != nil {
		return domain.CreateShipmentFromBidInput{}, err
	}
	plannedPickup, err := domain.ParseDateTime(derefString(req.PlannedPickupAt), "planned_pickup_at")
	if err != nil {
		return domain.CreateShipmentFromBidInput{}, err
	}
	plannedDelivery, err := domain.ParseDateTime(derefString(req.PlannedDeliveryAt), "planned_delivery_at")
	if err != nil {
		return domain.CreateShipmentFromBidInput{}, err
	}
	return domain.CreateShipmentFromBidInput{
		ShipmentNumber:    req.ShipmentNumber,
		BidID:             bidID,
		TransportOrderID:  transportOrderID,
		PlannedPickupAt:   plannedPickup,
		PlannedDeliveryAt: plannedDelivery,
	}, nil
}

func toShipmentResponse(s *domain.Shipment) map[string]any {
	return map[string]any{
		"id":                      s.ID.String(),
		"tenant_id":               s.TenantID.String(),
		"shipment_number":         s.ShipmentNumber,
		"transport_order_id":      optionalUUIDString(s.TransportOrderID),
		"shipper_company_id":      s.ShipperCompanyID.String(),
		"consignee_company_id":    s.ConsigneeCompanyID.String(),
		"carrier_company_id":      optionalUUIDString(s.CarrierCompanyID),
		"forwarder_company_id":    optionalUUIDString(s.ForwarderCompanyID),
		"driver_id":               optionalUUIDString(s.DriverID),
		"vehicle_id":              optionalUUIDString(s.VehicleID),
		"origin_location_id":      s.OriginLocationID.String(),
		"destination_location_id": s.DestinationLocationID.String(),
		"cargo_id":                optionalUUIDString(s.CargoID),
		"transport_mode":          s.TransportMode,
		"status":                  s.Status,
		"planned_pickup_at":       formatDateTime(s.PlannedPickupAt),
		"planned_delivery_at":     formatDateTime(s.PlannedDeliveryAt),
		"actual_pickup_at":        formatDateTime(s.ActualPickupAt),
		"actual_delivery_at":      formatDateTime(s.ActualDeliveryAt),
		"created_at":              s.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at":              s.UpdatedAt.UTC().Format(time.RFC3339),
		"version":                 s.Version,
	}
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
