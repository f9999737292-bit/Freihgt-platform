package handlers

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/freight-platform/transport-order-service/internal/domain"
	apperrors "github.com/freight-platform/transport-order-service/internal/platform/errors"
	"github.com/freight-platform/transport-order-service/internal/platform/respond"
)

type rateSnapshotGetter interface {
	GetRateSnapshotByTransportOrder(ctx context.Context, tenantID, transportOrderID uuid.UUID) (*domain.RateSnapshot, error)
}

type RateSnapshotInternalHandler struct {
	read rateSnapshotGetter
}

func NewRateSnapshotInternalHandler(read rateSnapshotGetter) *RateSnapshotInternalHandler {
	return &RateSnapshotInternalHandler{read: read}
}

type rateSnapshotInternalResponse struct {
	TransportOrderID    string `json:"transport_order_id"`
	TenantID            string `json:"tenant_id"`
	BuyerCompanyID      string `json:"buyer_company_id"`
	CarrierCompanyID    string `json:"carrier_company_id"`
	SnapshotID          string `json:"snapshot_id"`
	CurrencyCode        string `json:"currency_code"`
	TotalAmount         string `json:"total_amount"`
	PricingSource       string `json:"pricing_source"`
	PricingModelVersion string `json:"pricing_model_version"`
	ResolvedAt          string `json:"resolved_at"`
}

func (h *RateSnapshotInternalHandler) GetRateSnapshot(w http.ResponseWriter, r *http.Request) {
	tenantID, err := resolveVerifiedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	transportOrderID, err := uuid.Parse(chi.URLParam(r, "transportOrderId"))
	if err != nil {
		respond.Error(w, apperrors.Validation("invalid transport order id", map[string]any{"field": "transport_order_id"}))
		return
	}
	snapshot, err := h.read.GetRateSnapshotByTransportOrder(r.Context(), tenantID, transportOrderID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toRateSnapshotInternalResponse(snapshot))
}

func toRateSnapshotInternalResponse(snapshot *domain.RateSnapshot) rateSnapshotInternalResponse {
	return rateSnapshotInternalResponse{
		TransportOrderID:    snapshot.TransportOrderID.String(),
		TenantID:            snapshot.TenantID.String(),
		BuyerCompanyID:      snapshot.BuyerCompanyID.String(),
		CarrierCompanyID:    snapshot.CarrierCompanyID.String(),
		SnapshotID:          snapshot.ID.String(),
		CurrencyCode:        snapshot.CurrencyCode,
		TotalAmount:         snapshot.TotalAmount.StringFixed(domain.MoneyScale),
		PricingSource:       snapshot.PricingSource,
		PricingModelVersion: domain.PricingModelVersionSnapshotV1,
		ResolvedAt:          snapshot.ResolvedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}
