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

type BidHandler struct {
	service *service.BidService
}

func NewBidHandler(svc *service.BidService) *BidHandler {
	return &BidHandler{service: svc}
}

type createBidItemRequest struct {
	Description   *string  `json:"description"`
	BaseAmount    float64  `json:"base_amount"`
	FuelSurcharge float64  `json:"fuel_surcharge"`
	TollAmount    float64  `json:"toll_amount"`
	ExtraCharges  float64  `json:"extra_charges"`
	VATRate       *float64 `json:"vat_rate"`
	Comment       *string  `json:"comment"`
}

type createBidRequest struct {
	TenantID         string                 `json:"tenant_id"`
	CarrierCompanyID string                 `json:"carrier_company_id"`
	BidNumber        string                 `json:"bid_number"`
	CurrencyCode     *string                `json:"currency_code"`
	VATRate          *float64               `json:"vat_rate"`
	ValidUntil       *string                `json:"valid_until"`
	Items            []createBidItemRequest `json:"items"`
}

func (h *BidHandler) CreateBid(w http.ResponseWriter, r *http.Request) {
	freightRequestID, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	var req createBidRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}

	input, err := parseCreateBidRequest(req)
	if err != nil {
		respond.Error(w, err)
		return
	}

	bid, err := h.service.CreateBid(r.Context(), freightRequestID, input)
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusCreated, toBidResponse(bid))
}

func (h *BidHandler) ListBids(w http.ResponseWriter, r *http.Request) {
	freightRequestID, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	tenantID, err := domain.ParseUUID(r.URL.Query().Get("tenant_id"), "tenant_id")
	if err != nil {
		respond.Error(w, err)
		return
	}

	var status *string
	if raw := strings.TrimSpace(r.URL.Query().Get("status")); raw != "" {
		status = &raw
	}

	bids, err := h.service.ListBids(r.Context(), freightRequestID, tenantID, status)
	if err != nil {
		respond.Error(w, err)
		return
	}

	items := make([]map[string]any, 0, len(bids))
	for i := range bids {
		items = append(items, toBidResponse(&bids[i]))
	}

	respond.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *BidHandler) SubmitBid(w http.ResponseWriter, r *http.Request) {
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

	bid, err := h.service.SubmitBid(r.Context(), id, tenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, map[string]any{
		"id":     bid.ID.String(),
		"status": bid.Status,
	})
}

func (h *BidHandler) AcceptBid(w http.ResponseWriter, r *http.Request) {
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

	bid, err := h.service.AcceptBid(r.Context(), id, tenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}

	respond.JSON(w, http.StatusOK, map[string]any{
		"id":     bid.ID.String(),
		"status": bid.Status,
	})
}

func parseCreateBidRequest(req createBidRequest) (domain.CreateBidInput, error) {
	tenantID, err := domain.ParseUUID(req.TenantID, "tenant_id")
	if err != nil {
		return domain.CreateBidInput{}, err
	}
	carrierCompanyID, err := domain.ParseUUID(req.CarrierCompanyID, "carrier_company_id")
	if err != nil {
		return domain.CreateBidInput{}, err
	}

	var validUntil *time.Time
	if req.ValidUntil != nil {
		validUntil, err = domain.ParseDateTime(*req.ValidUntil, "valid_until")
		if err != nil {
			return domain.CreateBidInput{}, err
		}
	}

	items := make([]domain.CreateBidItemInput, 0, len(req.Items))
	for _, item := range req.Items {
		items = append(items, domain.CreateBidItemInput{
			Description:   item.Description,
			BaseAmount:    item.BaseAmount,
			FuelSurcharge: item.FuelSurcharge,
			TollAmount:    item.TollAmount,
			ExtraCharges:  item.ExtraCharges,
			VATRate:       item.VATRate,
			Comment:       item.Comment,
		})
	}

	return domain.CreateBidInput{
		TenantID:         tenantID,
		CarrierCompanyID: carrierCompanyID,
		BidNumber:        req.BidNumber,
		CurrencyCode:     req.CurrencyCode,
		VATRate:          req.VATRate,
		ValidUntil:       validUntil,
		Items:            items,
	}, nil
}

func toBidResponse(bid *domain.Bid) map[string]any {
	items := make([]map[string]any, 0, len(bid.Items))
	for _, item := range bid.Items {
		items = append(items, map[string]any{
			"id":                 item.ID.String(),
			"description":        item.Description,
			"base_amount":        item.BaseAmount,
			"fuel_surcharge":     item.FuelSurcharge,
			"toll_amount":        item.TollAmount,
			"extra_charges":      item.ExtraCharges,
			"amount_without_vat": item.AmountWithoutVAT,
			"vat_rate":           item.VATRate,
			"vat_amount":         item.VATAmount,
			"amount_with_vat":    item.AmountWithVAT,
			"comment":            item.Comment,
		})
	}

	return map[string]any{
		"id":                    bid.ID.String(),
		"tenant_id":             bid.TenantID.String(),
		"freight_request_id":    bid.FreightRequestID.String(),
		"carrier_company_id":    bid.CarrierCompanyID.String(),
		"bid_number":            bid.BidNumber,
		"status":                bid.Status,
		"total_amount":          bid.TotalAmount,
		"currency_code":         bid.CurrencyCode,
		"vat_rate":              bid.VATRate,
		"vat_amount":            bid.VATAmount,
		"total_amount_with_vat": bid.TotalAmountWithVAT,
		"valid_until":           formatDateTime(bid.ValidUntil),
		"submitted_at":          formatDateTime(bid.SubmittedAt),
		"items":                 items,
		"created_at":            bid.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		"updated_at":            bid.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		"version":               bid.Version,
	}
}
