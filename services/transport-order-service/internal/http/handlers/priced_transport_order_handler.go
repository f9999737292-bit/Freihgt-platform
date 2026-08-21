package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/freight-platform/transport-order-service/internal/domain"
	apperrors "github.com/freight-platform/transport-order-service/internal/platform/errors"
	"github.com/freight-platform/transport-order-service/internal/platform/respond"
	"github.com/freight-platform/transport-order-service/internal/service"
)

type PricedTransportOrderHandler struct {
	priced *service.PricedTransportOrderService
}

func NewPricedTransportOrderHandler(priced *service.PricedTransportOrderService) *PricedTransportOrderHandler {
	return &PricedTransportOrderHandler{priced: priced}
}

type pricingContextRequest struct {
	CarrierCompanyID  *string `json:"carrier_company_id"`
	AwardLinkID       *string `json:"award_link_id"`
	AwardScopeEventID *string `json:"award_scope_event_id"`
	AwardScopeLotID   *string `json:"award_scope_lot_id"`
	BidID             *string `json:"bid_id"`
	ManualSpotAmount  *string `json:"manual_spot_amount"`
	ManualSpotCurrency *string `json:"manual_spot_currency"`
	PricingSource     *string `json:"pricing_source"`
}

type createPricedTransportOrderRequest struct {
	createTransportOrderRequest
	PricingContext pricingContextRequest `json:"pricing_context"`
}

type createFromAwardScopeRequest struct {
	RfxEventID            string  `json:"rfx_event_id"`
	RfxLotID              *string `json:"rfx_lot_id"`
	OrderNumber           string  `json:"order_number"`
	ShipperCompanyID      string  `json:"shipper_company_id"`
	ConsigneeCompanyID    string  `json:"consignee_company_id"`
	OriginLocationID      string  `json:"origin_location_id"`
	DestinationLocationID string  `json:"destination_location_id"`
	CargoID               string  `json:"cargo_id"`
	TransportMode         string  `json:"transport_mode"`
	EquipmentType         string  `json:"equipment_type"`
	CarrierCompanyID      string  `json:"carrier_company_id"`
	SourceSystem          *string `json:"source_system"`
	ExternalReference     *string `json:"external_reference"`
	ActorUserID           string  `json:"actor_user_id"`
	ActorCompanyID        string  `json:"actor_company_id"`
}

func (h *PricedTransportOrderHandler) CreatePricedTransportOrder(w http.ResponseWriter, r *http.Request) {
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if err := domain.ValidateIdempotencyKey(idempotencyKey); err != nil {
		respond.Error(w, err)
		return
	}
	var req createPricedTransportOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}
	base, err := parseCreateTransportOrderRequest(req.createTransportOrderRequest)
	if err != nil {
		respond.Error(w, err)
		return
	}
	actor, err := parseActorFromRequest(r, base.TenantID)
	if err != nil {
		respond.Error(w, err)
		return
	}
	pricing, err := parsePricingContext(req.PricingContext)
	if err != nil {
		respond.Error(w, err)
		return
	}
	result, err := h.priced.CreatePricedTransportOrder(r.Context(), domain.CreatePricedTransportOrderInput{
		CreateTransportOrderInput: base,
		Actor:                     actor,
		PricingContext:            pricing,
		IdempotencyKey:            idempotencyKey,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toPricedTransportOrderResponse(result))
}

func (h *PricedTransportOrderHandler) CreateFromAwardScope(w http.ResponseWriter, r *http.Request) {
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idempotencyKey == "" {
		respond.Error(w, apperrors.Validation("idempotency key is required", map[string]any{"field": "idempotency_key"}))
		return
	}
	tenantID, err := parseTrustedTenant(r)
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req createFromAwardScopeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}
	input, err := parseCreateFromAwardScopeRequest(tenantID, idempotencyKey, req)
	if err != nil {
		respond.Error(w, err)
		return
	}
	result, err := h.priced.CreateFromAwardScope(r.Context(), input)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toPricedTransportOrderResponse(result))
}

func parsePricingContext(req pricingContextRequest) (domain.PricingContext, error) {
	out := domain.PricingContext{
		ManualSpotAmount:   trimOptional(req.ManualSpotAmount),
		ManualSpotCurrency: trimOptional(req.ManualSpotCurrency),
		PricingSource:      trimOptional(req.PricingSource),
	}
	if req.CarrierCompanyID != nil && strings.TrimSpace(*req.CarrierCompanyID) != "" {
		id, err := domain.ParseUUID(*req.CarrierCompanyID, "pricing_context.carrier_company_id")
		if err != nil {
			return domain.PricingContext{}, err
		}
		out.CarrierCompanyID = id
	}
	if req.AwardLinkID != nil && strings.TrimSpace(*req.AwardLinkID) != "" {
		id, err := domain.ParseUUID(*req.AwardLinkID, "pricing_context.award_link_id")
		if err != nil {
			return domain.PricingContext{}, err
		}
		out.AwardLinkID = &id
	}
	if req.AwardScopeEventID != nil && strings.TrimSpace(*req.AwardScopeEventID) != "" {
		id, err := domain.ParseUUID(*req.AwardScopeEventID, "pricing_context.award_scope_event_id")
		if err != nil {
			return domain.PricingContext{}, err
		}
		out.AwardScopeEventID = &id
	}
	if req.AwardScopeLotID != nil && strings.TrimSpace(*req.AwardScopeLotID) != "" {
		id, err := domain.ParseUUID(*req.AwardScopeLotID, "pricing_context.award_scope_lot_id")
		if err != nil {
			return domain.PricingContext{}, err
		}
		out.AwardScopeLotID = &id
	}
	if req.BidID != nil && strings.TrimSpace(*req.BidID) != "" {
		id, err := domain.ParseUUID(*req.BidID, "pricing_context.bid_id")
		if err != nil {
			return domain.PricingContext{}, err
		}
		out.BidID = &id
	}
	return out, nil
}

func parseCreateFromAwardScopeRequest(tenantID uuid.UUID, idempotencyKey string, req createFromAwardScopeRequest) (domain.CreateFromAwardScopeInput, error) {
	eventID, err := domain.ParseUUID(req.RfxEventID, "rfx_event_id")
	if err != nil {
		return domain.CreateFromAwardScopeInput{}, err
	}
	shipperID, err := domain.ParseUUID(req.ShipperCompanyID, "shipper_company_id")
	if err != nil {
		return domain.CreateFromAwardScopeInput{}, err
	}
	consigneeID, err := domain.ParseUUID(req.ConsigneeCompanyID, "consignee_company_id")
	if err != nil {
		return domain.CreateFromAwardScopeInput{}, err
	}
	originID, err := domain.ParseUUID(req.OriginLocationID, "origin_location_id")
	if err != nil {
		return domain.CreateFromAwardScopeInput{}, err
	}
	destID, err := domain.ParseUUID(req.DestinationLocationID, "destination_location_id")
	if err != nil {
		return domain.CreateFromAwardScopeInput{}, err
	}
	cargoID, err := domain.ParseUUID(req.CargoID, "cargo_id")
	if err != nil {
		return domain.CreateFromAwardScopeInput{}, err
	}
	carrierID, err := domain.ParseUUID(req.CarrierCompanyID, "carrier_company_id")
	if err != nil {
		return domain.CreateFromAwardScopeInput{}, err
	}
	actorUserID, err := domain.ParseUUID(req.ActorUserID, "actor_user_id")
	if err != nil {
		return domain.CreateFromAwardScopeInput{}, err
	}
	actorCompanyID, err := domain.ParseUUID(req.ActorCompanyID, "actor_company_id")
	if err != nil {
		return domain.CreateFromAwardScopeInput{}, err
	}
	var lotID *uuid.UUID
	if req.RfxLotID != nil && strings.TrimSpace(*req.RfxLotID) != "" {
		id, err := domain.ParseUUID(*req.RfxLotID, "rfx_lot_id")
		if err != nil {
			return domain.CreateFromAwardScopeInput{}, err
		}
		lotID = &id
	}
	createdBy := actorUserID
	return domain.CreateFromAwardScopeInput{
		TenantID:              tenantID,
		ActorUserID:           actorUserID,
		ActorCompanyID:        actorCompanyID,
		IdempotencyKey:        idempotencyKey,
		RfxEventID:            eventID,
		RfxLotID:              lotID,
		OrderNumber:           req.OrderNumber,
		ShipperCompanyID:      shipperID,
		ConsigneeCompanyID:    consigneeID,
		OriginLocationID:      originID,
		DestinationLocationID: destID,
		CargoID:               cargoID,
		TransportMode:         req.TransportMode,
		EquipmentType:         req.EquipmentType,
		CarrierCompanyID:      carrierID,
		SourceSystem:          req.SourceSystem,
		ExternalReference:     req.ExternalReference,
		CreatedBy:             &createdBy,
	}, nil
}

func parseActorFromRequest(r *http.Request, tenantID uuid.UUID) (domain.InternalActor, error) {
	userRaw := strings.TrimSpace(r.Header.Get("X-User-ID"))
	companyRaw := strings.TrimSpace(r.Header.Get("X-Company-ID"))
	kind := strings.TrimSpace(r.Header.Get("X-Actor-Kind"))
	if userRaw == "" || companyRaw == "" || kind == "" {
		return domain.InternalActor{}, apperrors.Validation("actor context is required", map[string]any{"field": "actor"})
	}
	userID, err := uuid.Parse(userRaw)
	if err != nil {
		return domain.InternalActor{}, apperrors.Validation("invalid user id", map[string]any{"field": "user_id"})
	}
	companyID, err := uuid.Parse(companyRaw)
	if err != nil {
		return domain.InternalActor{}, apperrors.Validation("invalid company id", map[string]any{"field": "company_id"})
	}
	return domain.InternalActor{
		TenantID:  tenantID,
		UserID:    userID,
		CompanyID: companyID,
		ActorKind: kind,
	}, nil
}

func parseTrustedTenant(r *http.Request) (uuid.UUID, error) {
	raw := strings.TrimSpace(r.Header.Get("X-Tenant-ID"))
	if raw == "" {
		return uuid.Nil, apperrors.Validation("tenant context is required", map[string]any{"field": "tenant_id"})
	}
	tenantID, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, apperrors.Validation("invalid tenant id", map[string]any{"field": "tenant_id"})
	}
	return tenantID, nil
}

func trimOptional(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func toPricedTransportOrderResponse(result *domain.PricedTransportOrderResult) map[string]any {
	resp := toTransportOrderResponse(result.Order)
	resp["pricing_model_version"] = result.Order.PricingModelVersion
	resp["rate_snapshot_id"] = result.RateSnapshotID.String()
	if result.RateSnapshot != nil {
		resp["rate_snapshot"] = map[string]any{
			"id":                         result.RateSnapshot.ID.String(),
			"pricing_source":             result.RateSnapshot.PricingSource,
			"currency_code":              result.RateSnapshot.CurrencyCode,
			"total_amount":               result.RateSnapshot.TotalAmount.StringFixed(domain.MoneyScale),
			"component_breakdown_status": result.RateSnapshot.ComponentBreakdownStatus,
			"resolved_at":                result.RateSnapshot.ResolvedAt.UTC().Format("2006-01-02T15:04:05Z"),
		}
	}
	return resp
}
