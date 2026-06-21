package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/transport-order-service/internal/domain"
	apperrors "github.com/freight-platform/transport-order-service/internal/platform/errors"
	"github.com/freight-platform/transport-order-service/internal/platform/respond"
	"github.com/freight-platform/transport-order-service/internal/service"
)

type Handler struct {
	service *service.TransportOrderService
}

func NewHandler(svc *service.TransportOrderService) *Handler {
	return &Handler{service: svc}
}

type healthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

func (h *Handler) Health(w http.ResponseWriter, _ *http.Request) {
	respond.JSON(w, http.StatusOK, healthResponse{
		Status:  "ok",
		Service: "transport-order-service",
	})
}

type createLocationRequest struct {
	TenantID     string   `json:"tenant_id"`
	CompanyID    *string  `json:"company_id"`
	LocationType string   `json:"location_type"`
	Name         string   `json:"name"`
	CountryCode  string   `json:"country_code"`
	Region       *string  `json:"region"`
	City         *string  `json:"city"`
	AddressLine  *string  `json:"address_line"`
	PostalCode   *string  `json:"postal_code"`
	Lat          *float64 `json:"lat"`
	Lon          *float64 `json:"lon"`
	Timezone     string   `json:"timezone"`
}

func (h *Handler) CreateLocation(w http.ResponseWriter, r *http.Request) {
	var req createLocationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}

	tenantID, err := domain.ParseUUID(req.TenantID, "tenant_id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	var companyID *uuid.UUID
	if req.CompanyID != nil && strings.TrimSpace(*req.CompanyID) != "" {
		parsed, err := domain.ParseUUID(*req.CompanyID, "company_id")
		if err != nil {
			respond.Error(w, err)
			return
		}
		companyID = &parsed
	}

	location, err := h.service.CreateLocation(r.Context(), domain.CreateLocationInput{
		TenantID:     tenantID,
		CompanyID:    companyID,
		LocationType: req.LocationType,
		Name:         req.Name,
		CountryCode:  req.CountryCode,
		Region:       req.Region,
		City:         req.City,
		AddressLine:  req.AddressLine,
		PostalCode:   req.PostalCode,
		Lat:          req.Lat,
		Lon:          req.Lon,
		Timezone:     req.Timezone,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusCreated, toLocationResponse(location))
}

func (h *Handler) GetLocation(w http.ResponseWriter, r *http.Request) {
	id, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	location, err := h.service.GetLocation(r.Context(), id)
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, toLocationResponse(location))
}

func (h *Handler) ListLocations(w http.ResponseWriter, r *http.Request) {
	tenantID, err := domain.ParseUUID(r.URL.Query().Get("tenant_id"), "tenant_id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	filter := domain.ListLocationsFilter{TenantID: tenantID, Limit: parseLimit(r), Offset: parseOffset(r)}
	if raw := strings.TrimSpace(r.URL.Query().Get("company_id")); raw != "" {
		parsed, err := domain.ParseUUID(raw, "company_id")
		if err != nil {
			respond.Error(w, err)
			return
		}
		filter.CompanyID = &parsed
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("location_type")); raw != "" {
		filter.LocationType = &raw
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("search")); raw != "" {
		filter.Search = &raw
	}

	locations, total, err := h.service.ListLocations(r.Context(), filter)
	if err != nil {
		respond.Error(w, err)
		return
	}

	items := make([]map[string]any, 0, len(locations))
	for i := range locations {
		items = append(items, toLocationResponse(&locations[i]))
	}

	respond.JSON(w, http.StatusOK, map[string]any{
		"items":  items,
		"total":  total,
		"limit":  filter.Limit,
		"offset": filter.Offset,
	})
}

type createCargoItemRequest struct {
	SKU         *string  `json:"sku"`
	Name        string   `json:"name"`
	Quantity    float64  `json:"quantity"`
	Unit        string   `json:"unit"`
	Weight      *float64 `json:"weight"`
	Volume      *float64 `json:"volume"`
	PackageType *string  `json:"package_type"`
	HazardClass *string  `json:"hazard_class"`
}

type createCargoRequest struct {
	TenantID           string                   `json:"tenant_id"`
	CargoType          string                   `json:"cargo_type"`
	Description        *string                  `json:"description"`
	GrossWeight        *float64                 `json:"gross_weight"`
	NetWeight          *float64                 `json:"net_weight"`
	Volume             *float64                 `json:"volume"`
	TemperatureMin     *float64                 `json:"temperature_min"`
	TemperatureMax     *float64                 `json:"temperature_max"`
	DangerousGoodsFlag bool                     `json:"dangerous_goods_flag"`
	CustomsRequired    bool                     `json:"customs_required"`
	Items              []createCargoItemRequest `json:"items"`
}

func (h *Handler) CreateCargo(w http.ResponseWriter, r *http.Request) {
	var req createCargoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}

	tenantID, err := domain.ParseUUID(req.TenantID, "tenant_id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	items := make([]domain.CreateCargoItemInput, 0, len(req.Items))
	for _, item := range req.Items {
		items = append(items, domain.CreateCargoItemInput{
			SKU:         item.SKU,
			Name:        item.Name,
			Quantity:    item.Quantity,
			Unit:        item.Unit,
			Weight:      item.Weight,
			Volume:      item.Volume,
			PackageType: item.PackageType,
			HazardClass: item.HazardClass,
		})
	}

	cargo, err := h.service.CreateCargo(r.Context(), domain.CreateCargoInput{
		TenantID:           tenantID,
		CargoType:          req.CargoType,
		Description:        req.Description,
		GrossWeight:        req.GrossWeight,
		NetWeight:          req.NetWeight,
		Volume:             req.Volume,
		TemperatureMin:     req.TemperatureMin,
		TemperatureMax:     req.TemperatureMax,
		DangerousGoodsFlag: req.DangerousGoodsFlag,
		CustomsRequired:    req.CustomsRequired,
		Items:              items,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusCreated, toCargoResponse(cargo))
}

func (h *Handler) GetCargo(w http.ResponseWriter, r *http.Request) {
	id, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	cargo, err := h.service.GetCargo(r.Context(), id)
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, toCargoResponse(cargo))
}

type createTransportOrderRequest struct {
	TenantID              string  `json:"tenant_id"`
	OrderNumber           string  `json:"order_number"`
	ShipperCompanyID      string  `json:"shipper_company_id"`
	ConsigneeCompanyID    string  `json:"consignee_company_id"`
	OriginLocationID      string  `json:"origin_location_id"`
	DestinationLocationID string  `json:"destination_location_id"`
	CargoID               string  `json:"cargo_id"`
	RequestedPickupDate   *string `json:"requested_pickup_date"`
	RequestedDeliveryDate *string `json:"requested_delivery_date"`
	TransportMode         string  `json:"transport_mode"`
	EquipmentType         *string `json:"equipment_type"`
	SourceSystem          *string `json:"source_system"`
	ExternalReference     *string `json:"external_reference"`
}

type updateTransportOrderRequest struct {
	RequestedPickupDate   *string `json:"requested_pickup_date"`
	RequestedDeliveryDate *string `json:"requested_delivery_date"`
	EquipmentType         *string `json:"equipment_type"`
	TransportMode         *string `json:"transport_mode"`
}

func (h *Handler) CreateTransportOrder(w http.ResponseWriter, r *http.Request) {
	var req createTransportOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}

	input, err := parseCreateTransportOrderRequest(req)
	if err != nil {
		respond.Error(w, err)
		return
	}

	order, err := h.service.CreateTransportOrder(r.Context(), input)
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusCreated, toTransportOrderResponse(order))
}

func (h *Handler) GetTransportOrder(w http.ResponseWriter, r *http.Request) {
	id, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	order, err := h.service.GetTransportOrder(r.Context(), id)
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, toTransportOrderResponse(order))
}

func (h *Handler) ListTransportOrders(w http.ResponseWriter, r *http.Request) {
	tenantID, err := domain.ParseUUID(r.URL.Query().Get("tenant_id"), "tenant_id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	filter := domain.ListTransportOrdersFilter{TenantID: tenantID, Limit: parseLimit(r), Offset: parseOffset(r)}
	if raw := strings.TrimSpace(r.URL.Query().Get("shipper_company_id")); raw != "" {
		parsed, err := domain.ParseUUID(raw, "shipper_company_id")
		if err != nil {
			respond.Error(w, err)
			return
		}
		filter.ShipperCompanyID = &parsed
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("consignee_company_id")); raw != "" {
		parsed, err := domain.ParseUUID(raw, "consignee_company_id")
		if err != nil {
			respond.Error(w, err)
			return
		}
		filter.ConsigneeCompanyID = &parsed
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("status")); raw != "" {
		filter.Status = &raw
	}

	orders, total, err := h.service.ListTransportOrders(r.Context(), filter)
	if err != nil {
		respond.Error(w, err)
		return
	}

	items := make([]map[string]any, 0, len(orders))
	for i := range orders {
		items = append(items, toTransportOrderResponse(&orders[i]))
	}

	respond.JSON(w, http.StatusOK, map[string]any{
		"items":  items,
		"total":  total,
		"limit":  filter.Limit,
		"offset": filter.Offset,
	})
}

func (h *Handler) UpdateTransportOrder(w http.ResponseWriter, r *http.Request) {
	id, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	var req updateTransportOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}

	input := domain.UpdateTransportOrderInput{EquipmentType: req.EquipmentType, TransportMode: req.TransportMode}
	if req.RequestedPickupDate != nil {
		pickup, err := domain.ParseOptionalDate(*req.RequestedPickupDate, "requested_pickup_date")
		if err != nil {
			respond.Error(w, err)
			return
		}
		input.RequestedPickupDate = pickup
	}
	if req.RequestedDeliveryDate != nil {
		delivery, err := domain.ParseOptionalDate(*req.RequestedDeliveryDate, "requested_delivery_date")
		if err != nil {
			respond.Error(w, err)
			return
		}
		input.RequestedDeliveryDate = delivery
	}

	order, err := h.service.UpdateTransportOrder(r.Context(), id, input)
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, toTransportOrderResponse(order))
}

func (h *Handler) SubmitTransportOrder(w http.ResponseWriter, r *http.Request) {
	id, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	order, err := h.service.SubmitTransportOrder(r.Context(), id)
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, map[string]any{
		"id":     order.ID.String(),
		"status": order.Status,
	})
}

func (h *Handler) CancelTransportOrder(w http.ResponseWriter, r *http.Request) {
	id, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	order, err := h.service.CancelTransportOrder(r.Context(), id)
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, map[string]any{
		"id":     order.ID.String(),
		"status": order.Status,
	})
}

func parseCreateTransportOrderRequest(req createTransportOrderRequest) (domain.CreateTransportOrderInput, error) {
	tenantID, err := domain.ParseUUID(req.TenantID, "tenant_id")
	if err != nil {
		return domain.CreateTransportOrderInput{}, err
	}
	shipperID, err := domain.ParseUUID(req.ShipperCompanyID, "shipper_company_id")
	if err != nil {
		return domain.CreateTransportOrderInput{}, err
	}
	consigneeID, err := domain.ParseUUID(req.ConsigneeCompanyID, "consignee_company_id")
	if err != nil {
		return domain.CreateTransportOrderInput{}, err
	}
	originID, err := domain.ParseUUID(req.OriginLocationID, "origin_location_id")
	if err != nil {
		return domain.CreateTransportOrderInput{}, err
	}
	destinationID, err := domain.ParseUUID(req.DestinationLocationID, "destination_location_id")
	if err != nil {
		return domain.CreateTransportOrderInput{}, err
	}
	cargoID, err := domain.ParseUUID(req.CargoID, "cargo_id")
	if err != nil {
		return domain.CreateTransportOrderInput{}, err
	}

	var pickup, delivery *time.Time
	if req.RequestedPickupDate != nil {
		pickup, err = domain.ParseOptionalDate(*req.RequestedPickupDate, "requested_pickup_date")
		if err != nil {
			return domain.CreateTransportOrderInput{}, err
		}
	}
	if req.RequestedDeliveryDate != nil {
		delivery, err = domain.ParseOptionalDate(*req.RequestedDeliveryDate, "requested_delivery_date")
		if err != nil {
			return domain.CreateTransportOrderInput{}, err
		}
	}

	return domain.CreateTransportOrderInput{
		TenantID:              tenantID,
		OrderNumber:           req.OrderNumber,
		ShipperCompanyID:      shipperID,
		ConsigneeCompanyID:    consigneeID,
		OriginLocationID:      originID,
		DestinationLocationID: destinationID,
		CargoID:               cargoID,
		RequestedPickupDate:   pickup,
		RequestedDeliveryDate: delivery,
		TransportMode:         req.TransportMode,
		EquipmentType:         req.EquipmentType,
		SourceSystem:          req.SourceSystem,
		ExternalReference:     req.ExternalReference,
	}, nil
}

func parseLimit(r *http.Request) int {
	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			return limit
		}
		limit = parsed
	}
	return limit
}

func parseOffset(r *http.Request) int {
	offset := 0
	if raw := r.URL.Query().Get("offset"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			return offset
		}
		offset = parsed
	}
	return offset
}

func toLocationResponse(location *domain.Location) map[string]any {
	resp := map[string]any{
		"id":            location.ID.String(),
		"tenant_id":     location.TenantID.String(),
		"location_type": location.LocationType,
		"name":          location.Name,
		"country_code":  location.CountryCode,
		"region":        location.Region,
		"city":          location.City,
		"address_line":  location.AddressLine,
		"postal_code":   location.PostalCode,
		"lat":           location.Lat,
		"lon":           location.Lon,
		"timezone":      location.Timezone,
		"status":        location.Status,
		"created_at":    location.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		"updated_at":    location.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		"version":       location.Version,
	}
	if location.CompanyID != nil {
		resp["company_id"] = location.CompanyID.String()
	}
	return resp
}

func toCargoResponse(cargo *domain.Cargo) map[string]any {
	items := make([]map[string]any, 0, len(cargo.Items))
	for _, item := range cargo.Items {
		items = append(items, map[string]any{
			"id":           item.ID.String(),
			"sku":          item.SKU,
			"name":         item.Name,
			"quantity":     item.Quantity,
			"unit":         item.Unit,
			"weight":       item.Weight,
			"volume":       item.Volume,
			"package_type": item.PackageType,
			"hazard_class": item.HazardClass,
		})
	}
	return map[string]any{
		"id":                   cargo.ID.String(),
		"tenant_id":            cargo.TenantID.String(),
		"cargo_type":           cargo.CargoType,
		"description":          cargo.Description,
		"gross_weight":         cargo.GrossWeight,
		"net_weight":           cargo.NetWeight,
		"volume":               cargo.Volume,
		"temperature_min":      cargo.TemperatureMin,
		"temperature_max":      cargo.TemperatureMax,
		"dangerous_goods_flag": cargo.DangerousGoodsFlag,
		"customs_required":     cargo.CustomsRequired,
		"items":                items,
		"created_at":           cargo.CreatedAt,
		"updated_at":           cargo.UpdatedAt,
		"version":              cargo.Version,
	}
}

func toTransportOrderResponse(order *domain.TransportOrder) map[string]any {
	resp := map[string]any{
		"id":                      order.ID.String(),
		"tenant_id":               order.TenantID.String(),
		"order_number":            order.OrderNumber,
		"shipper_company_id":      order.ShipperCompanyID.String(),
		"consignee_company_id":    order.ConsigneeCompanyID.String(),
		"origin_location_id":      order.OriginLocationID.String(),
		"destination_location_id": order.DestinationLocationID.String(),
		"requested_pickup_date":   domain.FormatDate(order.RequestedPickupDate),
		"requested_delivery_date": domain.FormatDate(order.RequestedDeliveryDate),
		"transport_mode":          order.TransportMode,
		"equipment_type":          order.EquipmentType,
		"status":                  order.Status,
		"source_system":           order.SourceSystem,
		"external_reference":      order.ExternalReference,
		"created_at":              order.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		"updated_at":              order.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		"version":                 order.Version,
	}
	if order.CargoID != nil {
		resp["cargo_id"] = order.CargoID.String()
	}
	return resp
}
