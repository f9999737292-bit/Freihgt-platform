package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/billing-register-service/internal/domain"
	apperrors "github.com/freight-platform/billing-register-service/internal/platform/errors"
	"github.com/freight-platform/billing-register-service/internal/platform/respond"
	"github.com/freight-platform/billing-register-service/internal/repository"
	"github.com/freight-platform/billing-register-service/internal/service"
)

type BillingRegisterHandler struct {
	registers *service.BillingRegisterService
}

func NewBillingRegisterHandler(registers *service.BillingRegisterService) *BillingRegisterHandler {
	return &BillingRegisterHandler{registers: registers}
}

func (h *BillingRegisterHandler) Health(w http.ResponseWriter, _ *http.Request) {
	respond.JSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "billing-register-service"})
}

type createRegisterRequest struct {
	TenantID            string   `json:"tenant_id"`
	RegisterNumber      string   `json:"register_number"`
	CustomerCompanyID   string   `json:"customer_company_id"`
	ContractorCompanyID string   `json:"contractor_company_id"`
	ContractID          *string  `json:"contract_id"`
	PeriodFrom          string   `json:"period_from"`
	PeriodTo            string   `json:"period_to"`
	CurrencyCode        string   `json:"currency_code"`
	VATRate             *float64 `json:"vat_rate"`
}

type tenantRequest struct {
	TenantID string `json:"tenant_id"`
}

type approveRequest struct {
	TenantID   string `json:"tenant_id"`
	ApprovedBy string `json:"approved_by"`
}

type createItemRequest struct {
	TenantID           string   `json:"tenant_id"`
	ShipmentID         string   `json:"shipment_id"`
	TransportOrderID   *string  `json:"transport_order_id"`
	RouteDescription   *string  `json:"route_description"`
	PickupDate         *string  `json:"pickup_date"`
	DeliveryDate       *string  `json:"delivery_date"`
	ShipperCompanyID   *string  `json:"shipper_company_id"`
	ConsigneeCompanyID *string  `json:"consignee_company_id"`
	CarrierCompanyID   *string  `json:"carrier_company_id"`
	BaseAmount         float64  `json:"base_amount"`
	ExtraCharges       float64  `json:"extra_charges"`
	Penalties          float64  `json:"penalties"`
	VATRate            *float64 `json:"vat_rate"`
}

func (h *BillingRegisterHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createRegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}
	input, err := parseCreateRegisterRequest(req)
	if err != nil {
		respond.Error(w, err)
		return
	}
	reg, err := h.registers.Create(r.Context(), input)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toRegisterResponse(reg))
}

func (h *BillingRegisterHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	detail, err := h.registers.GetByID(r.Context(), id)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toRegisterDetailResponse(detail))
}

func (h *BillingRegisterHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, err := domain.ParseUUID(r.URL.Query().Get("tenant_id"), "tenant_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	filter := domain.ListBillingRegistersFilter{TenantID: tenantID, Limit: parseLimit(r), Offset: parseOffset(r)}
	if raw := strings.TrimSpace(r.URL.Query().Get("customer_company_id")); raw != "" {
		id, err := domain.ParseUUID(raw, "customer_company_id")
		if err != nil {
			respond.Error(w, err)
			return
		}
		filter.CustomerCompanyID = &id
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("contractor_company_id")); raw != "" {
		id, err := domain.ParseUUID(raw, "contractor_company_id")
		if err != nil {
			respond.Error(w, err)
			return
		}
		filter.ContractorCompanyID = &id
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("status")); raw != "" {
		filter.Status = &raw
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("period_from")); raw != "" {
		d, err := domain.ParseDate(raw, "period_from")
		if err != nil {
			respond.Error(w, err)
			return
		}
		filter.PeriodFrom = &d
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("period_to")); raw != "" {
		d, err := domain.ParseDate(raw, "period_to")
		if err != nil {
			respond.Error(w, err)
			return
		}
		filter.PeriodTo = &d
	}
	regs, total, err := h.registers.List(r.Context(), filter)
	if err != nil {
		respond.Error(w, err)
		return
	}
	items := make([]map[string]any, 0, len(regs))
	for i := range regs {
		items = append(items, toRegisterResponse(&regs[i]))
	}
	respond.JSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

func (h *BillingRegisterHandler) AddItem(w http.ResponseWriter, r *http.Request) {
	registerID, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req createItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}
	input, err := parseCreateItemRequest(req)
	if err != nil {
		respond.Error(w, err)
		return
	}
	item, err := h.registers.AddItem(r.Context(), registerID, input)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toItemResponse(item))
}

func (h *BillingRegisterHandler) ListItems(w http.ResponseWriter, r *http.Request) {
	registerID, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	tenantID, err := domain.ParseUUID(r.URL.Query().Get("tenant_id"), "tenant_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	items, err := h.registers.ListItems(r.Context(), registerID, tenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	result := make([]map[string]any, 0, len(items))
	for i := range items {
		result = append(result, toItemResponse(&items[i]))
	}
	respond.JSON(w, http.StatusOK, map[string]any{"items": result})
}

func (h *BillingRegisterHandler) DeleteItem(w http.ResponseWriter, r *http.Request) {
	registerID, err := domain.ParseUUID(chi.URLParam(r, "register_id"), "register_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	itemID, err := domain.ParseUUID(chi.URLParam(r, "item_id"), "item_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	tenantID, err := domain.ParseUUID(r.URL.Query().Get("tenant_id"), "tenant_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	reg, err := h.registers.DeleteItem(r.Context(), registerID, itemID, tenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toRegisterResponse(reg))
}

func (h *BillingRegisterHandler) Calculate(w http.ResponseWriter, r *http.Request) {
	h.tenantAction(w, r, func(id uuid.UUID, in domain.TenantActionInput) (any, error) {
		reg, err := h.registers.Calculate(r.Context(), id, in)
		if err != nil {
			return nil, err
		}
		return toRegisterResponse(reg), nil
	})
}

func (h *BillingRegisterHandler) Approve(w http.ResponseWriter, r *http.Request) {
	id, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req approveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}
	tenantID, err := domain.ParseUUID(req.TenantID, "tenant_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	approvedBy, err := domain.ParseUUID(req.ApprovedBy, "approved_by")
	if err != nil {
		respond.Error(w, err)
		return
	}
	reg, err := h.registers.Approve(r.Context(), id, domain.ApproveRegisterInput{TenantID: tenantID, ApprovedBy: approvedBy})
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toRegisterResponse(reg))
}

func (h *BillingRegisterHandler) MarkSentToEDO(w http.ResponseWriter, r *http.Request) {
	h.tenantAction(w, r, func(id uuid.UUID, in domain.TenantActionInput) (any, error) {
		reg, err := h.registers.MarkSentToEDO(r.Context(), id, in)
		if err != nil {
			return nil, err
		}
		return toRegisterResponse(reg), nil
	})
}

func (h *BillingRegisterHandler) MarkSigned(w http.ResponseWriter, r *http.Request) {
	h.tenantAction(w, r, func(id uuid.UUID, in domain.TenantActionInput) (any, error) {
		reg, err := h.registers.MarkSigned(r.Context(), id, in)
		if err != nil {
			return nil, err
		}
		return toRegisterResponse(reg), nil
	})
}

func (h *BillingRegisterHandler) MarkPaid(w http.ResponseWriter, r *http.Request) {
	h.tenantAction(w, r, func(id uuid.UUID, in domain.TenantActionInput) (any, error) {
		reg, err := h.registers.MarkPaid(r.Context(), id, in)
		if err != nil {
			return nil, err
		}
		return toRegisterResponse(reg), nil
	})
}

func (h *BillingRegisterHandler) Close(w http.ResponseWriter, r *http.Request) {
	h.tenantAction(w, r, func(id uuid.UUID, in domain.TenantActionInput) (any, error) {
		reg, err := h.registers.Close(r.Context(), id, in)
		if err != nil {
			return nil, err
		}
		return toRegisterResponse(reg), nil
	})
}

func (h *BillingRegisterHandler) tenantAction(w http.ResponseWriter, r *http.Request, fn func(uuid.UUID, domain.TenantActionInput) (any, error)) {
	id, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req tenantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}
	tenantID, err := domain.ParseUUID(req.TenantID, "tenant_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	result, err := fn(id, domain.TenantActionInput{TenantID: tenantID})
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, result)
}

func parseCreateRegisterRequest(req createRegisterRequest) (domain.CreateBillingRegisterInput, error) {
	tenantID, err := domain.ParseUUID(req.TenantID, "tenant_id")
	if err != nil {
		return domain.CreateBillingRegisterInput{}, err
	}
	customerID, err := domain.ParseUUID(req.CustomerCompanyID, "customer_company_id")
	if err != nil {
		return domain.CreateBillingRegisterInput{}, err
	}
	contractorID, err := domain.ParseUUID(req.ContractorCompanyID, "contractor_company_id")
	if err != nil {
		return domain.CreateBillingRegisterInput{}, err
	}
	contractID, err := domain.ParseOptionalUUID(deref(req.ContractID), "contract_id")
	if err != nil {
		return domain.CreateBillingRegisterInput{}, err
	}
	periodFrom, err := domain.ParseDate(req.PeriodFrom, "period_from")
	if err != nil {
		return domain.CreateBillingRegisterInput{}, err
	}
	periodTo, err := domain.ParseDate(req.PeriodTo, "period_to")
	if err != nil {
		return domain.CreateBillingRegisterInput{}, err
	}
	return domain.CreateBillingRegisterInput{
		TenantID: tenantID, RegisterNumber: req.RegisterNumber, CustomerCompanyID: customerID,
		ContractorCompanyID: contractorID, ContractID: contractID, PeriodFrom: periodFrom, PeriodTo: periodTo,
		CurrencyCode: req.CurrencyCode, VATRate: req.VATRate,
	}, nil
}

func parseCreateItemRequest(req createItemRequest) (domain.CreateBillingRegisterItemInput, error) {
	tenantID, err := domain.ParseUUID(req.TenantID, "tenant_id")
	if err != nil {
		return domain.CreateBillingRegisterItemInput{}, err
	}
	shipmentID, err := domain.ParseUUID(req.ShipmentID, "shipment_id")
	if err != nil {
		return domain.CreateBillingRegisterItemInput{}, err
	}
	transportOrderID, err := domain.ParseOptionalUUID(deref(req.TransportOrderID), "transport_order_id")
	if err != nil {
		return domain.CreateBillingRegisterItemInput{}, err
	}
	pickupDate, err := domain.ParseOptionalDate(deref(req.PickupDate), "pickup_date")
	if err != nil {
		return domain.CreateBillingRegisterItemInput{}, err
	}
	deliveryDate, err := domain.ParseOptionalDate(deref(req.DeliveryDate), "delivery_date")
	if err != nil {
		return domain.CreateBillingRegisterItemInput{}, err
	}
	shipperID, err := domain.ParseOptionalUUID(deref(req.ShipperCompanyID), "shipper_company_id")
	if err != nil {
		return domain.CreateBillingRegisterItemInput{}, err
	}
	consigneeID, err := domain.ParseOptionalUUID(deref(req.ConsigneeCompanyID), "consignee_company_id")
	if err != nil {
		return domain.CreateBillingRegisterItemInput{}, err
	}
	carrierID, err := domain.ParseOptionalUUID(deref(req.CarrierCompanyID), "carrier_company_id")
	if err != nil {
		return domain.CreateBillingRegisterItemInput{}, err
	}
	return domain.CreateBillingRegisterItemInput{
		TenantID: tenantID, ShipmentID: shipmentID, TransportOrderID: transportOrderID,
		RouteDescription: req.RouteDescription, PickupDate: pickupDate, DeliveryDate: deliveryDate,
		ShipperCompanyID: shipperID, ConsigneeCompanyID: consigneeID, CarrierCompanyID: carrierID,
		BaseAmount: req.BaseAmount, ExtraCharges: req.ExtraCharges, Penalties: req.Penalties, VATRate: req.VATRate,
	}, nil
}

func toRegisterResponse(reg *domain.BillingRegister) map[string]any {
	return map[string]any{
		"id": reg.ID.String(), "tenant_id": reg.TenantID.String(), "register_number": reg.RegisterNumber,
		"customer_company_id": reg.CustomerCompanyID.String(), "contractor_company_id": reg.ContractorCompanyID.String(),
		"contract_id": optionalUUIDString(reg.ContractID),
		"period_from": domain.FormatDate(reg.PeriodFrom), "period_to": domain.FormatDate(reg.PeriodTo),
		"currency_code": reg.CurrencyCode, "vat_rate": reg.VATRate, "status": reg.Status,
		"total_without_vat": reg.TotalWithoutVAT, "vat_amount": reg.VATAmount, "total_with_vat": reg.TotalWithVAT,
		"created_at": reg.CreatedAt.UTC().Format(time.RFC3339), "approved_at": formatDateTime(reg.ApprovedAt),
		"approved_by": optionalUUIDString(reg.ApprovedBy), "updated_at": reg.UpdatedAt.UTC().Format(time.RFC3339),
		"version": reg.Version,
	}
}

func toRegisterDetailResponse(detail *repository.RegisterDetail) map[string]any {
	resp := toRegisterResponse(detail.Register)
	items := make([]map[string]any, 0, len(detail.Items))
	for i := range detail.Items {
		items = append(items, toItemResponse(&detail.Items[i]))
	}
	resp["items"] = items
	resp["closing_document_packages"] = mapPackages(detail.ClosingDocumentPackages)
	resp["invoices"] = mapInvoices(detail.Invoices)
	resp["acts"] = mapActs(detail.Acts)
	resp["vat_invoices"] = mapVATInvoices(detail.VATInvoices)
	resp["upd_documents"] = mapUPDs(detail.UPDDocuments)
	return resp
}

func toItemResponse(item *domain.BillingRegisterItem) map[string]any {
	return map[string]any{
		"id": item.ID.String(), "register_id": item.RegisterID.String(), "shipment_id": item.ShipmentID.String(),
		"transport_order_id": optionalUUIDString(item.TransportOrderID), "route_description": item.RouteDescription,
		"pickup_date": domain.FormatOptionalDate(item.PickupDate), "delivery_date": domain.FormatOptionalDate(item.DeliveryDate),
		"shipper_company_id": optionalUUIDString(item.ShipperCompanyID), "consignee_company_id": optionalUUIDString(item.ConsigneeCompanyID),
		"carrier_company_id": optionalUUIDString(item.CarrierCompanyID),
		"base_amount": item.BaseAmount, "extra_charges": item.ExtraCharges, "penalties": item.Penalties,
		"amount_without_vat": item.AmountWithoutVAT, "vat_rate": item.VATRate, "vat_amount": item.VATAmount,
		"amount_with_vat": item.AmountWithVAT, "status": item.Status, "created_at": item.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func deref(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
