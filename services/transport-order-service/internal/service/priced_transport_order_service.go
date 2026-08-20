package service

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/transport-order-service/internal/client/contractrate"
	"github.com/freight-platform/transport-order-service/internal/domain"
	apperrors "github.com/freight-platform/transport-order-service/internal/platform/errors"
)

type RateResolver interface {
	Resolve(ctx context.Context, in domain.CreatePricedTransportOrderInput, pricingDate time.Time) (domain.ResolveRateResult, error)
}

type PricedOrderStore interface {
	FindCreateIdempotency(ctx context.Context, tenantID, actorCompanyID uuid.UUID, idempotencyKey string) (*domain.CreateIdempotencyRecord, error)
	GetPricedResult(ctx context.Context, tenantID, orderID, snapshotID uuid.UUID) (*domain.PricedTransportOrderResult, error)
	CreatePricedOrder(ctx context.Context, in domain.CreatePricedTransportOrderInput, snapshot domain.RateSnapshot, requestHash string) (*domain.PricedTransportOrderResult, error)
}

type PricedTransportOrderService struct {
	orders         TransportOrderStore
	cargoes        CargoStore
	locationLookup LocationReferenceStore
	pricedOrders   PricedOrderStore
	rates          RateResolver
}

func NewPricedTransportOrderService(
	orders TransportOrderStore,
	cargoes CargoStore,
	locationLookup LocationReferenceStore,
	pricedOrders PricedOrderStore,
	rates RateResolver,
) *PricedTransportOrderService {
	return &PricedTransportOrderService{
		orders:         orders,
		cargoes:        cargoes,
		locationLookup: locationLookup,
		pricedOrders:   pricedOrders,
		rates:          rates,
	}
}

func (s *PricedTransportOrderService) CreatePricedTransportOrder(ctx context.Context, in domain.CreatePricedTransportOrderInput) (*domain.PricedTransportOrderResult, error) {
	in.TransportMode = domain.NormalizeTransportMode(in.TransportMode)
	if in.EquipmentType != nil {
		trimmed := strings.ToUpper(strings.TrimSpace(*in.EquipmentType))
		in.EquipmentType = &trimmed
	}
	if err := domain.ValidateCreatePricedTransportOrderInput(in); err != nil {
		return nil, err
	}
	if err := s.validateOrderReferences(ctx, in.CreateTransportOrderInput); err != nil {
		return nil, err
	}

	requestHash, err := domain.ComputeCreateRequestHash(in)
	if err != nil {
		return nil, err
	}
	if existing, err := s.pricedOrders.FindCreateIdempotency(ctx, in.TenantID, in.Actor.CompanyID, in.IdempotencyKey); err != nil {
		return nil, err
	} else if existing != nil {
		if existing.RequestHash != requestHash {
			return nil, apperrors.Conflict("idempotency key reused with different request payload", map[string]any{"field": "idempotency_key"})
		}
		return s.pricedOrders.GetPricedResult(ctx, in.TenantID, existing.TransportOrderID, existing.RateSnapshotID)
	}

	pricingDate := domain.PricingDateForOrder(in.CreateTransportOrderInput)
	resolutionHash, err := domain.ComputeResolutionRequestHash(in, pricingDate)
	if err != nil {
		return nil, err
	}
	resolved, err := s.rates.Resolve(ctx, in, pricingDate)
	if err != nil {
		return nil, err
	}
	carrierID, err := contractrate.ResolveCarrierID(in, resolved)
	if err != nil {
		return nil, err
	}
	in.PricingContext.CarrierCompanyID = carrierID
	snapshot, err := contractrate.BuildSnapshotFromResolve(in, resolved, resolutionHash, carrierID)
	if err != nil {
		return nil, err
	}
	return s.pricedOrders.CreatePricedOrder(ctx, in, snapshot, requestHash)
}

func (s *PricedTransportOrderService) CreateFromAwardScope(ctx context.Context, in domain.CreateFromAwardScopeInput) (*domain.PricedTransportOrderResult, error) {
	if err := domain.ValidateCreateFromAwardScopeInput(in); err != nil {
		return nil, err
	}
	equipment := strings.ToUpper(strings.TrimSpace(in.EquipmentType))
	pricingSource := domain.NormalizePricingSource("RFQ_AWARD")
	var lotID *uuid.UUID
	if in.RfxLotID != nil && *in.RfxLotID != uuid.Nil {
		lotID = in.RfxLotID
	}
	actorKind := "INTERNAL_SERVICE"
	if in.CreatedBy != nil {
		actorKind = "USER"
	}
	actorUser := in.ActorUserID
	if in.CreatedBy != nil {
		actorUser = *in.CreatedBy
	}
	pricedInput := domain.CreatePricedTransportOrderInput{
		CreateTransportOrderInput: domain.CreateTransportOrderInput{
			TenantID:              in.TenantID,
			OrderNumber:           in.OrderNumber,
			ShipperCompanyID:      in.ShipperCompanyID,
			ConsigneeCompanyID:    in.ConsigneeCompanyID,
			OriginLocationID:      in.OriginLocationID,
			DestinationLocationID: in.DestinationLocationID,
			CargoID:               in.CargoID,
			TransportMode:         in.TransportMode,
			EquipmentType:         &equipment,
			SourceSystem:          in.SourceSystem,
			ExternalReference:     in.ExternalReference,
		},
		Actor: domain.InternalActor{
			TenantID:  in.TenantID,
			UserID:    actorUser,
			CompanyID: in.ActorCompanyID,
			ActorKind: actorKind,
		},
		PricingContext: domain.PricingContext{
			CarrierCompanyID:  in.CarrierCompanyID,
			AwardScopeEventID: &in.RfxEventID,
			AwardScopeLotID:   lotID,
			PricingSource:     &pricingSource,
		},
		IdempotencyKey: in.IdempotencyKey,
	}
	return s.CreatePricedTransportOrder(ctx, pricedInput)
}

func (s *PricedTransportOrderService) validateOrderReferences(ctx context.Context, in domain.CreateTransportOrderInput) error {
	for _, ref := range []struct {
		id    uuid.UUID
		field string
	}{
		{in.ShipperCompanyID, "shipper_company_id"},
		{in.ConsigneeCompanyID, "consignee_company_id"},
	} {
		exists, err := s.orders.CompanyExists(ctx, ref.id, in.TenantID)
		if err != nil {
			return err
		}
		if !exists {
			return apperrors.NotFound(ref.field + " not found")
		}
	}
	for _, ref := range []struct {
		id    uuid.UUID
		field string
	}{
		{in.OriginLocationID, "origin_location_id"},
		{in.DestinationLocationID, "destination_location_id"},
	} {
		exists, err := s.locationLookup.ExistsInTenant(ctx, ref.id, in.TenantID)
		if err != nil {
			return err
		}
		if !exists {
			return apperrors.NotFound(ref.field + " not found")
		}
	}
	cargoExists, err := s.cargoes.ExistsInTenant(ctx, in.CargoID, in.TenantID)
	if err != nil {
		return err
	}
	if !cargoExists {
		return apperrors.NotFound("cargo_id not found")
	}
	return nil
}
