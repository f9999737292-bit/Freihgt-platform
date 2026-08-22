package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/billing-register-service/internal/domain"
	"github.com/freight-platform/billing-register-service/internal/platform/respond"
	"github.com/freight-platform/billing-register-service/internal/repository"
)

type BillingRegisterInternalHandler struct {
	registers *repository.BillingRegisterRepository
}

func NewBillingRegisterInternalHandler(registers *repository.BillingRegisterRepository) *BillingRegisterInternalHandler {
	return &BillingRegisterInternalHandler{registers: registers}
}

type internalPayableResponse struct {
	RegisterID   string `json:"register_id"`
	TenantID     string `json:"tenant_id"`
	Status       string `json:"status"`
	Version      int    `json:"version"`
	CurrencyCode string `json:"currency_code"`
	TotalWithVAT string `json:"total_with_vat"`
	TaxBasis     string `json:"tax_basis"`
	UpdatedAt    string `json:"updated_at"`
}

func (h *BillingRegisterInternalHandler) GetPayable(w http.ResponseWriter, r *http.Request) {
	tenantID, err := tenantIDFromInternalRequest(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	registerID, err := domain.ParseUUID(chi.URLParam(r, "registerId"), "register_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	read, err := h.registers.GetInternalPayable(r.Context(), tenantID, registerID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, internalPayableResponse{
		RegisterID:   read.RegisterID.String(),
		TenantID:     read.TenantID.String(),
		Status:       read.Status,
		Version:      read.Version,
		CurrencyCode: read.CurrencyCode,
		TotalWithVAT: read.TotalWithVAT,
		TaxBasis:     read.TaxBasis,
		UpdatedAt:    read.UpdatedAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
	})
}
