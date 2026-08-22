package handlers

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/payment-service/internal/domain"
	apperrors "github.com/freight-platform/payment-service/internal/platform/errors"
	"github.com/freight-platform/payment-service/internal/platform/respond"
	"github.com/freight-platform/payment-service/internal/repository"
)

type internalObligationByRegisterResponse struct {
	TenantID          string `json:"tenant_id"`
	ObligationID      string `json:"obligation_id"`
	BillingRegisterID string `json:"billing_register_id"`
	TransportOrderID  string `json:"transport_order_id"`
	BuyerCompanyID    string `json:"buyer_company_id"`
	CarrierCompanyID  string `json:"carrier_company_id"`
	Version           int    `json:"version"`
	OriginalAmount    string `json:"original_amount"`
	PaidAmount        string `json:"paid_amount"`
	CurrencyCode      string `json:"currency_code"`
	Status            string `json:"status"`
	TaxBasis          string `json:"tax_basis"`
	UpdatedAt         string `json:"updated_at"`
}

func (h *PaymentHandler) GetObligationByBillingRegister(w http.ResponseWriter, r *http.Request) {
	tenantID, err := tenantIDFromInternalRequest(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	billingRegisterID, err := domain.ParseUUID(chi.URLParam(r, "billingRegisterId"), "billing_register_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	read, err := h.payments.GetInternalObligationByBillingRegister(r.Context(), tenantID, billingRegisterID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toInternalObligationByRegisterResponse(read))
}

func tenantIDFromInternalRequest(r *http.Request) (uuid.UUID, error) {
	raw := strings.TrimSpace(r.Header.Get("X-Tenant-ID"))
	if raw == "" {
		return uuid.Nil, apperrors.Validation("X-Tenant-ID is required", map[string]any{"field": "tenant_id"})
	}
	return domain.ParseUUID(raw, "tenant_id")
}

func toInternalObligationByRegisterResponse(read *repository.InternalObligationRead) internalObligationByRegisterResponse {
	return internalObligationByRegisterResponse{
		TenantID:          read.TenantID.String(),
		ObligationID:      read.ObligationID.String(),
		BillingRegisterID: read.BillingRegisterID.String(),
		TransportOrderID:  read.TransportOrderID.String(),
		BuyerCompanyID:    read.BuyerCompanyID.String(),
		CarrierCompanyID:  read.CarrierCompanyID.String(),
		Version:           read.Version,
		OriginalAmount:    read.OriginalAmount,
		PaidAmount:        read.PaidAmount,
		CurrencyCode:      read.CurrencyCode,
		Status:            read.Status,
		TaxBasis:          read.TaxBasis,
		UpdatedAt:         read.UpdatedAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
	}
}
