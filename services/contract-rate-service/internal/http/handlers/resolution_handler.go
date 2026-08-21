package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/contract-rate-service/internal/domain"
	"github.com/freight-platform/contract-rate-service/internal/platform/respond"
	"github.com/freight-platform/contract-rate-service/internal/service"
)

type ResolutionHandler struct {
	svc    *service.ResolutionService
	actors *ActorResolver
}

func NewResolutionHandler(svc *service.ResolutionService, actors *ActorResolver) *ResolutionHandler {
	return &ResolutionHandler{svc: svc, actors: actors}
}

func (h *ResolutionHandler) Resolve(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actors.FromRequest(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req struct {
		BuyerCompanyID        uuid.UUID `json:"buyer_company_id"`
		CarrierCompanyID      uuid.UUID `json:"carrier_company_id"`
		OriginLocationID      uuid.UUID `json:"origin_location_id"`
		DestinationLocationID uuid.UUID `json:"destination_location_id"`
		EquipmentType         string    `json:"equipment_type"`
		TransportMode         string    `json:"transport_mode"`
		PricingDate           string    `json:"pricing_date"`
		CurrencyCode          *string   `json:"currency_code"`
		ManualSpotAmount      *string   `json:"manual_spot_amount"`
		ManualSpotCurrency    *string   `json:"manual_spot_currency"`
		PricingSource         *string   `json:"pricing_source"`
		AwardLinkID           *uuid.UUID `json:"award_link_id"`
		AwardScopeEventID     *uuid.UUID `json:"award_scope_event_id"`
		AwardScopeLotID       *uuid.UUID `json:"award_scope_lot_id"`
		BidID                 *uuid.UUID `json:"bid_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, domainValidation("invalid request body"))
		return
	}
	var pricingDate time.Time
	if req.PricingDate != "" {
		pricingDate, err = parseDate(req.PricingDate, "pricing_date")
		if err != nil {
			respond.Error(w, err)
			return
		}
	}
	manualSpot, err := parseOptionalDecimal(req.ManualSpotAmount, "manual_spot_amount")
	if err != nil {
		respond.Error(w, err)
		return
	}
	result, err := h.svc.Resolve(r.Context(), domain.ResolveRateRequest{
		TenantID: actor.TenantID, BuyerCompanyID: req.BuyerCompanyID, CarrierCompanyID: req.CarrierCompanyID,
		OriginLocationID: req.OriginLocationID, DestinationLocationID: req.DestinationLocationID,
		EquipmentType: req.EquipmentType, TransportMode: req.TransportMode, PricingDate: pricingDate,
		CurrencyCode: req.CurrencyCode, ManualSpotAmount: manualSpot, ManualSpotCurrency: req.ManualSpotCurrency,
		PricingSource: req.PricingSource, AwardLinkID: req.AwardLinkID,
		AwardScopeEventID: req.AwardScopeEventID, AwardScopeLotID: req.AwardScopeLotID,
		BidID: req.BidID, Actor: actor,
	}, CorrelationID(r))
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, result)
}
