package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

func (s *RfxService) ListCarrierInvitedEvents(ctx context.Context, actor domain.ActorContext, filter domain.ListCarrierInvitedEventsFilter) ([]domain.CarrierInvitedRfxEvent, int, error) {
	if err := actor.Validate(); err != nil {
		return nil, 0, err
	}
	carrierCompanyID, err := s.resolveCarrierCompanyIDOnly(ctx, actor, filter.CarrierCompanyID)
	if err != nil {
		return nil, 0, err
	}
	filter.TenantID = actor.TenantID
	filter.CarrierCompanyID = carrierCompanyID
	if filter.Limit == 0 {
		filter.Limit = 20
	}
	if err := domain.ValidateListCarrierInvitedEventsFilter(filter); err != nil {
		return nil, 0, err
	}
	return s.repo.ListCarrierInvitedEvents(ctx, filter, nowUTC())
}

func (s *RfxService) GetOwnResponse(ctx context.Context, actor domain.ActorContext, eventID uuid.UUID, requestedCarrierCompanyID uuid.UUID) (*domain.RfxResponse, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	carrierCompanyID, err := s.requireCarrierEventAccess(ctx, actor, eventID, requestedCarrierCompanyID)
	if err != nil {
		return nil, err
	}
	response, err := s.repo.GetResponseByEventAndCompany(ctx, eventID, carrierCompanyID, actor.TenantID)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (s *RfxService) GetResponse(ctx context.Context, actor domain.ActorContext, id uuid.UUID) (*domain.RfxResponse, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	response, err := s.repo.GetResponseByID(ctx, id, actor.TenantID)
	if err != nil {
		return nil, err
	}
	kind, carrierIDs, err := s.resolveActor(ctx, actor)
	if err != nil {
		return nil, err
	}
	if kind == domain.ActorKindBuyer {
		event, eventErr := s.repo.GetEventByID(ctx, response.RfxEventID, actor.TenantID)
		if eventErr != nil {
			return nil, eventErr
		}
		if _, err := s.requireOwnerCompanyAccess(ctx, actor, event.OwnerCompanyID); err != nil {
			return nil, err
		}
		return response, nil
	}
	if kind == domain.ActorKindCarrier {
		if !carrierCanViewResponse(carrierIDs, response.ParticipantCompanyID) {
			return nil, apperrors.NotFound("rfx response not found")
		}
		return response, nil
	}
	return nil, apperrors.Forbidden("authorization required")
}

func (s *RfxService) ListLanes(ctx context.Context, actor domain.ActorContext, lotID uuid.UUID, requestedCarrierCompanyID uuid.UUID) ([]domain.RfxLane, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	kind, _, err := s.resolveActor(ctx, actor)
	if err != nil {
		return nil, err
	}
	if kind == domain.ActorKindBuyer {
		if _, err := s.requireLotOwnerCompanyAccess(ctx, actor, lotID); err != nil {
			return nil, err
		}
		return s.repo.ListLanesByLot(ctx, lotID, actor.TenantID)
	}
	if _, _, err := s.requireCarrierLotAccess(ctx, actor, lotID, requestedCarrierCompanyID); err != nil {
		return nil, err
	}
	return s.repo.ListLanesByLot(ctx, lotID, actor.TenantID)
}

func (s *RfxService) GetCarrierParticipant(ctx context.Context, actor domain.ActorContext, eventID uuid.UUID, requestedCarrierCompanyID uuid.UUID) (*domain.RfxParticipant, error) {
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	carrierCompanyID, err := s.requireCarrierEventAccess(ctx, actor, eventID, requestedCarrierCompanyID)
	if err != nil {
		return nil, err
	}
	return s.repo.GetParticipantByEventAndCompany(ctx, eventID, carrierCompanyID, actor.TenantID)
}

func (s *RfxService) resolveCarrierCompanyIDOnly(ctx context.Context, actor domain.ActorContext, requested uuid.UUID) (uuid.UUID, error) {
	carrierCompanyID, _, err := s.resolveCarrierCompanyID(ctx, actor, requested)
	return carrierCompanyID, err
}

func carrierCanViewResponse(carrierIDs []uuid.UUID, responseCarrierCompanyID uuid.UUID) bool {
	for _, id := range carrierIDs {
		if id == responseCarrierCompanyID {
			return true
		}
	}
	return false
}
