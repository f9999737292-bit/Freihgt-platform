package handlers

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/billing-register-service/internal/domain"
	apperrors "github.com/freight-platform/billing-register-service/internal/platform/errors"
	"github.com/freight-platform/billing-register-service/internal/platform/respond"
	"github.com/freight-platform/billing-register-service/internal/repository"
)

type FreightSettlementInternalHandler struct {
	settlements *repository.FreightSettlementRepository
}

func NewFreightSettlementInternalHandler(settlements *repository.FreightSettlementRepository) *FreightSettlementInternalHandler {
	return &FreightSettlementInternalHandler{settlements: settlements}
}

type internalSettlementResponse struct {
	SettlementID        string  `json:"settlement_id"`
	TenantID            string  `json:"tenant_id"`
	TransportOrderID    string  `json:"transport_order_id"`
	ShipmentID          string  `json:"shipment_id"`
	BuyerCompanyID      string  `json:"buyer_company_id"`
	CarrierCompanyID    string  `json:"carrier_company_id"`
	Status              string  `json:"status"`
	OpenDisputeCount    int     `json:"open_dispute_count"`
	Version             int     `json:"version"`
	BillingLinkRevision int64   `json:"billing_link_revision"`
	BillingLinkState    string  `json:"billing_link_state"`
	CurrencyCode        string  `json:"currency_code"`
	BaseFreightAmount   string  `json:"base_freight_amount"`
	AccrualAmountExVAT  string  `json:"accrual_amount_ex_vat"`
	TotalWithoutVAT                 string  `json:"total_without_vat"`
	ProposedAccessorialTotalExVAT   string  `json:"proposed_accessorial_total_ex_vat"`
	ProposedAccessorialSourceStatus string  `json:"proposed_accessorial_source_status"`
	RateSnapshotID      *string `json:"rate_snapshot_id,omitempty"`
	UpdatedAt           string  `json:"updated_at"`
}

type internalBillingLinkResponse struct {
	SettlementID        string  `json:"settlement_id"`
	TenantID            string  `json:"tenant_id"`
	TransportOrderID    string  `json:"transport_order_id"`
	BillingLinkRevision int64   `json:"billing_link_revision"`
	BillingLinkState    string  `json:"billing_link_state"`
	AmountExVAT         *string `json:"amount_ex_vat"`
	BillingRegisterID   *string `json:"billing_register_id,omitempty"`
	CurrencyCode        string  `json:"currency_code"`
	TaxBasis            string  `json:"tax_basis"`
}

func (h *FreightSettlementInternalHandler) GetByTransportOrder(w http.ResponseWriter, r *http.Request) {
	tenantID, err := tenantIDFromInternalRequest(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	transportOrderID, err := domain.ParseUUID(chi.URLParam(r, "transportOrderId"), "transport_order_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	read, err := h.settlements.GetInternalByTransportOrder(r.Context(), tenantID, transportOrderID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toInternalSettlementResponse(read))
}

func (h *FreightSettlementInternalHandler) GetBillingLink(w http.ResponseWriter, r *http.Request) {
	tenantID, err := tenantIDFromInternalRequest(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	settlementID, err := domain.ParseUUID(chi.URLParam(r, "settlementId"), "settlement_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	read, err := h.settlements.GetInternalBillingLink(r.Context(), tenantID, settlementID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toInternalBillingLinkResponse(read))
}

func tenantIDFromInternalRequest(r *http.Request) (uuid.UUID, error) {
	raw := strings.TrimSpace(r.Header.Get("X-Tenant-ID"))
	if raw == "" {
		return uuid.Nil, apperrors.Validation("X-Tenant-ID is required", map[string]any{"field": "tenant_id"})
	}
	return domain.ParseUUID(raw, "tenant_id")
}

func toInternalSettlementResponse(read *repository.InternalSettlementRead) internalSettlementResponse {
	var rateSnapshotID *string
	if read.RateSnapshotID != nil {
		s := read.RateSnapshotID.String()
		rateSnapshotID = &s
	}
	return internalSettlementResponse{
		SettlementID:        read.SettlementID.String(),
		TenantID:            read.TenantID.String(),
		TransportOrderID:    read.TransportOrderID.String(),
		ShipmentID:          read.ShipmentID.String(),
		BuyerCompanyID:      read.BuyerCompanyID.String(),
		CarrierCompanyID:    read.CarrierCompanyID.String(),
		Status:              read.Status,
		OpenDisputeCount:    read.OpenDisputeCount,
		Version:             read.Version,
		BillingLinkRevision: read.BillingLinkRevision,
		BillingLinkState:    read.BillingLinkState,
		CurrencyCode:        read.CurrencyCode,
		BaseFreightAmount:   read.BaseFreightAmount,
		AccrualAmountExVAT:              read.AccrualAmountExVAT,
		TotalWithoutVAT:                 read.TotalWithoutVAT,
		ProposedAccessorialTotalExVAT:   read.ProposedAccessorialTotalExVAT,
		ProposedAccessorialSourceStatus: read.ProposedAccessorialSourceStatus,
		RateSnapshotID:                  rateSnapshotID,
		UpdatedAt:           read.UpdatedAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
	}
}

func toInternalBillingLinkResponse(read *repository.InternalBillingLinkRead) internalBillingLinkResponse {
	var registerID *string
	if read.BillingRegisterID != nil {
		s := read.BillingRegisterID.String()
		registerID = &s
	}
	return internalBillingLinkResponse{
		SettlementID:        read.SettlementID.String(),
		TenantID:            read.TenantID.String(),
		TransportOrderID:    read.TransportOrderID.String(),
		BillingLinkRevision: read.BillingLinkRevision,
		BillingLinkState:    read.BillingLinkState,
		AmountExVAT:         read.AmountExVAT,
		BillingRegisterID:   registerID,
		CurrencyCode:        read.CurrencyCode,
		TaxBasis:            read.TaxBasis,
	}
}
