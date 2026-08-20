package service

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/repository"
	"github.com/freight-platform/rfx-service/internal/transportorderclient"
)

type awardConversionStore interface {
	ListAwardTransportOrdersByEvent(ctx context.Context, eventID, tenantID uuid.UUID) ([]domain.RfxAwardTransportOrder, error)
	PrepareAwardConversion(ctx context.Context, event *domain.RfxEvent, award *domain.RfxAward, scope domain.AwardConversionScope, convertedBy uuid.UUID) (*repository.AwardConversionPrepared, error)
	LinkAwardTransportOrdersTransactional(
		ctx context.Context,
		event *domain.RfxEvent,
		award *domain.RfxAward,
		response *domain.RfxResponse,
		links []repository.AwardConversionLinkInput,
		convertedBy uuid.UUID,
		preCommit func(context.Context, pgx.Tx) error,
	) (*domain.ConvertAwardTransportOrdersResult, error)
}

func (s *RfxService) awardConversionStore() awardConversionStore {
	if store, ok := s.repo.(awardConversionStore); ok {
		return store
	}
	return nil
}

func (s *RfxService) ListAwardTransportOrders(ctx context.Context, actor domain.ActorContext, eventID uuid.UUID) ([]domain.RfxAwardTransportOrder, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	if _, _, err := s.requireBuyerEventAccess(ctx, actor, eventID); err != nil {
		return nil, err
	}
	store := s.awardConversionStore()
	if store == nil {
		return nil, apperrors.Internal("award conversion store unavailable", nil)
	}
	return store.ListAwardTransportOrdersByEvent(ctx, eventID, actor.TenantID)
}

func (s *RfxService) ConvertAwardToTransportOrders(ctx context.Context, actor domain.ActorContext, eventID uuid.UUID) (*domain.ConvertAwardTransportOrdersResult, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	event, ownerCompanyID, err := s.requireBuyerEventAccess(ctx, actor, eventID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateAwardConversionEventStatus(event.Status); err != nil {
		return nil, err
	}

	store := s.awardConversionStore()
	evalStore := s.evalStore()
	if store == nil || evalStore == nil {
		return nil, apperrors.Internal("award conversion store unavailable", nil)
	}
	if s.transportOrders == nil {
		return nil, apperrors.Internal("transport order client unavailable", nil)
	}

	award, err := evalStore.GetAwardByEvent(ctx, eventID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	response, err := s.repo.GetResponseByID(ctx, award.RfxResponseID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateAwardConversionResponse(award, response); err != nil {
		return nil, err
	}

	lines, err := evalStore.ListOfferLinesByResponse(ctx, response.ID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	lotCount, err := s.repo.CountLotsByEvent(ctx, eventID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	lots, err := s.repo.ListLotsByEvent(ctx, eventID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	eventCurrency := ""
	if event.CurrencyCode != nil {
		eventCurrency = *event.CurrencyCode
	}
	scopes, err := domain.BuildAwardConversionScopes(lotCount, lots, lines, eventCurrency)
	if err != nil {
		return nil, err
	}

	existing, err := store.ListAwardTransportOrdersByEvent(ctx, event.ID, event.TenantID)
	if err != nil {
		return nil, err
	}
	existingByLot := map[uuid.UUID]domain.RfxAwardTransportOrder{}
	var existingEventLevel *domain.RfxAwardTransportOrder
	for i := range existing {
		item := existing[i]
		if item.RfxLotID == uuid.Nil {
			copyItem := item
			existingEventLevel = &copyItem
			continue
		}
		existingByLot[item.RfxLotID] = item
	}

	sourceSystem := domain.AwardTransportOrderSourceSystem
	links := make([]repository.AwardConversionLinkInput, 0, len(scopes))
	for _, scope := range scopes {
		if scope.RfxLotID != uuid.Nil {
			if _, ok := existingByLot[scope.RfxLotID]; ok {
				continue
			}
		} else if existingEventLevel != nil {
			continue
		}

		prepared, err := store.PrepareAwardConversion(ctx, event, award, scope, actor.UserID)
		if err != nil {
			return nil, err
		}
		equipmentType := ""
		if prepared.EquipmentType != nil {
			equipmentType = strings.TrimSpace(*prepared.EquipmentType)
		}
		var lotID *uuid.UUID
		if scope.RfxLotID != uuid.Nil {
			id := scope.RfxLotID
			lotID = &id
		}
		created, err := s.transportOrders.CreateFromAwardScope(ctx, transportorderclient.CreateFromAwardScopeRequest{
			TenantID:              event.TenantID,
			RfxEventID:            event.ID,
			RfxLotID:              lotID,
			OrderNumber:           prepared.OrderNumber,
			ShipperCompanyID:      event.OwnerCompanyID,
			ConsigneeCompanyID:    event.OwnerCompanyID,
			OriginLocationID:      prepared.OriginLocationID,
			DestinationLocationID: prepared.DestinationLocationID,
			CargoID:               prepared.CargoID,
			TransportMode:         prepared.TransportMode,
			EquipmentType:         equipmentType,
			CarrierCompanyID:      award.CarrierCompanyID,
			SourceSystem:          &sourceSystem,
			ExternalReference:     &prepared.ExternalReference,
			ActorUserID:           actor.UserID,
			ActorCompanyID:        ownerCompanyID,
			IdempotencyKey:        transportorderclient.AwardConversionIdempotencyKey(event.TenantID, event.ID, scope.RfxLotID),
		})
		if err != nil {
			return nil, err
		}
		links = append(links, repository.AwardConversionLinkInput{
			Scope:            scope,
			LaneID:           prepared.LaneID,
			TransportOrderID: created.TransportOrderID,
			OrderNumber:      created.OrderNumber,
			OrderStatus:      created.OrderStatus,
		})
	}

	preCommit := func(ctx context.Context, tx pgx.Tx) error {
		recorder := s.audit
		if auditRepo, ok := s.audit.(*repository.AuditRepository); ok {
			recorder = auditRepo.WithTx(tx)
		}
		return recordAudit(ctx, recorder, actor, ownerCompanyID, "rfx_event", eventID, "convert_award_transport_orders", map[string]any{
			"award_id":           award.ID.String(),
			"response_id":        response.ID.String(),
			"carrier_company_id": award.CarrierCompanyID.String(),
			"scope_count":        len(scopes),
			"created_count":      len(links),
		})
	}

	return store.LinkAwardTransportOrdersTransactional(ctx, event, award, response, links, actor.UserID, preCommit)
}
