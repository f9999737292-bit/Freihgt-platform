package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/billing-register-service/internal/domain"
	apperrors "github.com/freight-platform/billing-register-service/internal/platform/errors"
	"github.com/freight-platform/billing-register-service/internal/platform/respond"
	"github.com/freight-platform/billing-register-service/internal/service"
)

type ClosingDocumentHandler struct {
	closing *service.ClosingDocumentService
}

func NewClosingDocumentHandler(closing *service.ClosingDocumentService) *ClosingDocumentHandler {
	return &ClosingDocumentHandler{closing: closing}
}

type createPackageRequest struct {
	TenantID      string `json:"tenant_id"`
	PackageNumber string `json:"package_number"`
	PackageType   string `json:"package_type"`
}

type createInvoiceRequest struct {
	TenantID        string `json:"tenant_id"`
	InvoiceNumber   string `json:"invoice_number"`
	InvoiceDate     string `json:"invoice_date"`
	SellerCompanyID string `json:"seller_company_id"`
	BuyerCompanyID  string `json:"buyer_company_id"`
}

type createActRequest struct {
	TenantID           string  `json:"tenant_id"`
	ActNumber          string  `json:"act_number"`
	ActDate            string  `json:"act_date"`
	SellerCompanyID    string  `json:"seller_company_id"`
	BuyerCompanyID     string  `json:"buyer_company_id"`
	ServiceDescription *string `json:"service_description"`
}

type createVATInvoiceRequest struct {
	TenantID         string `json:"tenant_id"`
	VATInvoiceNumber string `json:"vat_invoice_number"`
	VATInvoiceDate   string `json:"vat_invoice_date"`
	SellerCompanyID  string `json:"seller_company_id"`
	BuyerCompanyID   string `json:"buyer_company_id"`
}

type createUPDRequest struct {
	TenantID        string `json:"tenant_id"`
	UPDNumber       string `json:"upd_number"`
	UPDDate         string `json:"upd_date"`
	SellerCompanyID string `json:"seller_company_id"`
	BuyerCompanyID  string `json:"buyer_company_id"`
	FunctionCode    string `json:"function_code"`
}

func (h *ClosingDocumentHandler) CreatePackage(w http.ResponseWriter, r *http.Request) {
	registerID, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req createPackageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}
	tenantID, err := domain.ParseUUID(req.TenantID, "tenant_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	pkg, err := h.closing.CreatePackage(r.Context(), registerID, domain.CreateClosingDocumentPackageInput{
		TenantID: tenantID, PackageNumber: req.PackageNumber, PackageType: req.PackageType,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toPackageResponse(pkg))
}

func (h *ClosingDocumentHandler) CreateInvoice(w http.ResponseWriter, r *http.Request) {
	registerID, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req createInvoiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}
	input, err := parseInvoiceRequest(req)
	if err != nil {
		respond.Error(w, err)
		return
	}
	inv, err := h.closing.CreateInvoice(r.Context(), registerID, input)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toInvoiceResponse(inv))
}

func (h *ClosingDocumentHandler) CreateAct(w http.ResponseWriter, r *http.Request) {
	registerID, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req createActRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}
	input, err := parseActRequest(req)
	if err != nil {
		respond.Error(w, err)
		return
	}
	act, err := h.closing.CreateAct(r.Context(), registerID, input)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toActResponse(act))
}

func (h *ClosingDocumentHandler) CreateVATInvoice(w http.ResponseWriter, r *http.Request) {
	registerID, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req createVATInvoiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}
	input, err := parseVATInvoiceRequest(req)
	if err != nil {
		respond.Error(w, err)
		return
	}
	inv, err := h.closing.CreateVATInvoice(r.Context(), registerID, input)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toVATInvoiceResponse(inv))
}

func (h *ClosingDocumentHandler) CreateUPD(w http.ResponseWriter, r *http.Request) {
	registerID, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req createUPDRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}
	input, err := parseUPDRequest(req)
	if err != nil {
		respond.Error(w, err)
		return
	}
	upd, err := h.closing.CreateUPD(r.Context(), registerID, input)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toUPDResponse(upd))
}

func parseInvoiceRequest(req createInvoiceRequest) (domain.CreateInvoiceInput, error) {
	tenantID, err := domain.ParseUUID(req.TenantID, "tenant_id")
	if err != nil {
		return domain.CreateInvoiceInput{}, err
	}
	sellerID, err := domain.ParseUUID(req.SellerCompanyID, "seller_company_id")
	if err != nil {
		return domain.CreateInvoiceInput{}, err
	}
	buyerID, err := domain.ParseUUID(req.BuyerCompanyID, "buyer_company_id")
	if err != nil {
		return domain.CreateInvoiceInput{}, err
	}
	date, err := domain.ParseDate(req.InvoiceDate, "invoice_date")
	if err != nil {
		return domain.CreateInvoiceInput{}, err
	}
	return domain.CreateInvoiceInput{
		TenantID: tenantID, InvoiceNumber: req.InvoiceNumber, InvoiceDate: date,
		SellerCompanyID: sellerID, BuyerCompanyID: buyerID,
	}, nil
}

func parseActRequest(req createActRequest) (domain.CreateActInput, error) {
	tenantID, err := domain.ParseUUID(req.TenantID, "tenant_id")
	if err != nil {
		return domain.CreateActInput{}, err
	}
	sellerID, err := domain.ParseUUID(req.SellerCompanyID, "seller_company_id")
	if err != nil {
		return domain.CreateActInput{}, err
	}
	buyerID, err := domain.ParseUUID(req.BuyerCompanyID, "buyer_company_id")
	if err != nil {
		return domain.CreateActInput{}, err
	}
	date, err := domain.ParseDate(req.ActDate, "act_date")
	if err != nil {
		return domain.CreateActInput{}, err
	}
	return domain.CreateActInput{
		TenantID: tenantID, ActNumber: req.ActNumber, ActDate: date,
		SellerCompanyID: sellerID, BuyerCompanyID: buyerID, ServiceDescription: req.ServiceDescription,
	}, nil
}

func parseVATInvoiceRequest(req createVATInvoiceRequest) (domain.CreateVATInvoiceInput, error) {
	tenantID, err := domain.ParseUUID(req.TenantID, "tenant_id")
	if err != nil {
		return domain.CreateVATInvoiceInput{}, err
	}
	sellerID, err := domain.ParseUUID(req.SellerCompanyID, "seller_company_id")
	if err != nil {
		return domain.CreateVATInvoiceInput{}, err
	}
	buyerID, err := domain.ParseUUID(req.BuyerCompanyID, "buyer_company_id")
	if err != nil {
		return domain.CreateVATInvoiceInput{}, err
	}
	date, err := domain.ParseDate(req.VATInvoiceDate, "vat_invoice_date")
	if err != nil {
		return domain.CreateVATInvoiceInput{}, err
	}
	return domain.CreateVATInvoiceInput{
		TenantID: tenantID, VATInvoiceNumber: req.VATInvoiceNumber, VATInvoiceDate: date,
		SellerCompanyID: sellerID, BuyerCompanyID: buyerID,
	}, nil
}

func parseUPDRequest(req createUPDRequest) (domain.CreateUPDInput, error) {
	tenantID, err := domain.ParseUUID(req.TenantID, "tenant_id")
	if err != nil {
		return domain.CreateUPDInput{}, err
	}
	sellerID, err := domain.ParseUUID(req.SellerCompanyID, "seller_company_id")
	if err != nil {
		return domain.CreateUPDInput{}, err
	}
	buyerID, err := domain.ParseUUID(req.BuyerCompanyID, "buyer_company_id")
	if err != nil {
		return domain.CreateUPDInput{}, err
	}
	date, err := domain.ParseDate(req.UPDDate, "upd_date")
	if err != nil {
		return domain.CreateUPDInput{}, err
	}
	return domain.CreateUPDInput{
		TenantID: tenantID, UPDNumber: req.UPDNumber, UPDDate: date, FunctionCode: req.FunctionCode,
		SellerCompanyID: sellerID, BuyerCompanyID: buyerID,
	}, nil
}

func toPackageResponse(p *domain.ClosingDocumentPackage) map[string]any {
	return map[string]any{
		"id": p.ID.String(), "tenant_id": p.TenantID.String(), "register_id": p.RegisterID.String(),
		"package_number": p.PackageNumber, "package_type": p.PackageType, "status": p.Status,
		"created_at": p.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func toInvoiceResponse(inv *domain.Invoice) map[string]any {
	return map[string]any{
		"id": inv.ID.String(), "tenant_id": inv.TenantID.String(), "register_id": inv.RegisterID.String(),
		"invoice_number": inv.InvoiceNumber, "invoice_date": domain.FormatDate(inv.InvoiceDate),
		"seller_company_id": inv.SellerCompanyID.String(), "buyer_company_id": inv.BuyerCompanyID.String(),
		"total_amount": inv.TotalAmount, "currency_code": inv.CurrencyCode, "status": inv.Status,
		"document_id": optionalUUIDString(inv.DocumentID), "created_at": inv.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func toActResponse(act *domain.Act) map[string]any {
	return map[string]any{
		"id": act.ID.String(), "tenant_id": act.TenantID.String(), "register_id": act.RegisterID.String(),
		"act_number": act.ActNumber, "act_date": domain.FormatDate(act.ActDate),
		"seller_company_id": act.SellerCompanyID.String(), "buyer_company_id": act.BuyerCompanyID.String(),
		"service_description": act.ServiceDescription, "total_amount": act.TotalAmount, "currency_code": act.CurrencyCode,
		"status": act.Status, "document_id": optionalUUIDString(act.DocumentID), "created_at": act.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func toVATInvoiceResponse(inv *domain.VATInvoice) map[string]any {
	return map[string]any{
		"id": inv.ID.String(), "tenant_id": inv.TenantID.String(), "register_id": inv.RegisterID.String(),
		"vat_invoice_number": inv.VATInvoiceNumber, "vat_invoice_date": domain.FormatDate(inv.VATInvoiceDate),
		"seller_company_id": inv.SellerCompanyID.String(), "buyer_company_id": inv.BuyerCompanyID.String(),
		"amount_without_vat": inv.AmountWithoutVAT, "vat_rate": inv.VATRate, "vat_amount": inv.VATAmount,
		"amount_with_vat": inv.AmountWithVAT, "status": inv.Status, "document_id": optionalUUIDString(inv.DocumentID),
		"created_at": inv.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func toUPDResponse(upd *domain.UPDDocument) map[string]any {
	return map[string]any{
		"id": upd.ID.String(), "tenant_id": upd.TenantID.String(), "register_id": upd.RegisterID.String(),
		"upd_number": upd.UPDNumber, "upd_date": domain.FormatDate(upd.UPDDate),
		"seller_company_id": upd.SellerCompanyID.String(), "buyer_company_id": upd.BuyerCompanyID.String(),
		"function_code": upd.FunctionCode, "amount_without_vat": upd.AmountWithoutVAT, "vat_rate": upd.VATRate,
		"vat_amount": upd.VATAmount, "amount_with_vat": upd.AmountWithVAT, "status": upd.Status,
		"document_id": optionalUUIDString(upd.DocumentID), "created_at": upd.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func mapPackages(items []domain.ClosingDocumentPackage) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for i := range items {
		result = append(result, toPackageResponse(&items[i]))
	}
	return result
}

func mapInvoices(items []domain.Invoice) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for i := range items {
		result = append(result, toInvoiceResponse(&items[i]))
	}
	return result
}

func mapActs(items []domain.Act) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for i := range items {
		result = append(result, toActResponse(&items[i]))
	}
	return result
}

func mapVATInvoices(items []domain.VATInvoice) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for i := range items {
		result = append(result, toVATInvoiceResponse(&items[i]))
	}
	return result
}

func mapUPDs(items []domain.UPDDocument) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for i := range items {
		result = append(result, toUPDResponse(&items[i]))
	}
	return result
}
