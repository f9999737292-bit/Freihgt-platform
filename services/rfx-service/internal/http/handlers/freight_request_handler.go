package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/platform/respond"
	"github.com/freight-platform/rfx-service/internal/service"
)

type FreightRequestHandler struct {
	service *service.FreightRequestService
}

func NewFreightRequestHandler(svc *service.FreightRequestService) *FreightRequestHandler {
	return &FreightRequestHandler{service: svc}
}

type createFreightRequestFromOrderRequest struct {
	TenantID             string  `json:"tenant_id"`
	TransportOrderID     string  `json:"transport_order_id"`
	FreightRequestNumber string  `json:"freight_request_number"`
	RequestType          string  `json:"request_type"`
	ShipperCompanyID     string  `json:"shipper_company_id"`
	ResponseDeadline     *string `json:"response_deadline"`
	CurrencyCode         *string `json:"currency_code"`
}

func (h *FreightRequestHandler) CreateFromTransportOrder(w http.ResponseWriter, r *http.Request) {
	var req createFreightRequestFromOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}

	input, err := parseCreateFreightRequestFromOrderRequest(req)
	if err != nil {
		respond.Error(w, err)
		return
	}

	fr, err := h.service.CreateFromTransportOrder(r.Context(), input)
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusCreated, toFreightRequestResponse(fr))
}

func (h *FreightRequestHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	tenantID, err := domain.ParseUUID(r.URL.Query().Get("tenant_id"), "tenant_id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	fr, err := h.service.GetByID(r.Context(), id, tenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, toFreightRequestResponse(fr))
}

func (h *FreightRequestHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, err := domain.ParseUUID(r.URL.Query().Get("tenant_id"), "tenant_id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	filter := domain.ListFreightRequestsFilter{TenantID: tenantID, Limit: parseLimit(r), Offset: parseOffset(r)}
	if raw := strings.TrimSpace(r.URL.Query().Get("request_type")); raw != "" {
		filter.RequestType = &raw
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("status")); raw != "" {
		filter.Status = &raw
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("shipper_company_id")); raw != "" {
		parsed, err := domain.ParseUUID(raw, "shipper_company_id")
		if err != nil {
			respond.Error(w, err)
			return
		}
		filter.ShipperCompanyID = &parsed
	}

	requests, total, err := h.service.List(r.Context(), filter)
	if err != nil {
		respond.Error(w, err)
		return
	}

	items := make([]map[string]any, 0, len(requests))
	for i := range requests {
		items = append(items, toFreightRequestResponse(&requests[i]))
	}

	respond.JSON(w, http.StatusOK, map[string]any{
		"items":  items,
		"total":  total,
		"limit":  filter.Limit,
		"offset": filter.Offset,
	})
}

func (h *FreightRequestHandler) Publish(w http.ResponseWriter, r *http.Request) {
	id, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	tenantID, err := domain.ParseUUID(r.URL.Query().Get("tenant_id"), "tenant_id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	fr, err := h.service.Publish(r.Context(), id, tenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, map[string]any{
		"id":     fr.ID.String(),
		"status": fr.Status,
	})
}

func parseCreateFreightRequestFromOrderRequest(req createFreightRequestFromOrderRequest) (domain.CreateFreightRequestFromOrderInput, error) {
	tenantID, err := domain.ParseUUID(req.TenantID, "tenant_id")
	if err != nil {
		return domain.CreateFreightRequestFromOrderInput{}, err
	}
	transportOrderID, err := domain.ParseUUID(req.TransportOrderID, "transport_order_id")
	if err != nil {
		return domain.CreateFreightRequestFromOrderInput{}, err
	}
	shipperCompanyID, err := domain.ParseUUID(req.ShipperCompanyID, "shipper_company_id")
	if err != nil {
		return domain.CreateFreightRequestFromOrderInput{}, err
	}

	var responseDeadline *time.Time
	if req.ResponseDeadline != nil {
		responseDeadline, err = domain.ParseDateTime(*req.ResponseDeadline, "response_deadline")
		if err != nil {
			return domain.CreateFreightRequestFromOrderInput{}, err
		}
	}

	return domain.CreateFreightRequestFromOrderInput{
		TenantID:             tenantID,
		TransportOrderID:     transportOrderID,
		FreightRequestNumber: req.FreightRequestNumber,
		RequestType:          req.RequestType,
		ShipperCompanyID:     shipperCompanyID,
		ResponseDeadline:     responseDeadline,
		CurrencyCode:         req.CurrencyCode,
	}, nil
}

func toFreightRequestResponse(fr *domain.FreightRequest) map[string]any {
	resp := map[string]any{
		"id":                     fr.ID.String(),
		"tenant_id":              fr.TenantID.String(),
		"freight_request_number": fr.FreightRequestNumber,
		"request_type":           fr.RequestType,
		"shipper_company_id":     fr.ShipperCompanyID.String(),
		"status":                 fr.Status,
		"response_deadline":      formatDateTime(fr.ResponseDeadline),
		"currency_code":          fr.CurrencyCode,
		"created_at":             fr.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		"updated_at":             fr.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		"version":                fr.Version,
	}
	if fr.TransportOrderID != nil {
		resp["transport_order_id"] = fr.TransportOrderID.String()
	}
	return resp
}
