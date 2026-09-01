package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

func (s *FreightRequestService) requireCarrierActor(ctx context.Context, actor domain.ActorContext) ([]uuid.UUID, error) {
	if s.actors == nil {
		return nil, apperrors.Forbidden("carrier company membership is required")
	}
	kind, carrierIDs, err := s.actors.ResolveActorKind(ctx, actor)
	if err != nil {
		return nil, err
	}
	if kind != domain.ActorKindCarrier {
		return nil, apperrors.Forbidden("carrier authorization required")
	}
	if len(carrierIDs) == 0 {
		return nil, apperrors.Forbidden("carrier company membership is required")
	}
	if actor.UserID == uuid.Nil {
		return nil, apperrors.Forbidden("user context is required")
	}
	return carrierIDs, nil
}

func (s *FreightRequestService) applyCarrierListScope(ctx context.Context, actor domain.ActorContext, filter *domain.ListFreightRequestsFilter) (bool, error) {
	if _, err := s.requireCarrierActor(ctx, actor); err != nil {
		return false, err
	}
	filter.ShipperCompanyID = nil
	filter.ShipperCompanyIDs = nil
	if filter.Status != nil {
		if !domain.IsCarrierVisibleFreightRequestStatus(*filter.Status) {
			return true, nil
		}
		filter.Statuses = nil
		return false, nil
	}
	filter.Statuses = domain.CarrierVisibleFreightRequestStatuses()
	return false, nil
}

func (s *FreightRequestService) applyActorListScope(ctx context.Context, actor domain.ActorContext, filter *domain.ListFreightRequestsFilter) (bool, error) {
	if s.actors == nil {
		return false, apperrors.Forbidden("buyer company membership is required")
	}
	kind, _, err := s.actors.ResolveActorKind(ctx, actor)
	if err != nil {
		return false, err
	}
	switch kind {
	case domain.ActorKindBuyer:
		return false, s.applyBuyerListScope(ctx, actor, filter)
	case domain.ActorKindCarrier:
		return s.applyCarrierListScope(ctx, actor, filter)
	default:
		return false, apperrors.Forbidden("buyer authorization required")
	}
}

func (s *FreightRequestService) authorizeFreightRequestRead(ctx context.Context, actor domain.ActorContext, fr *domain.FreightRequest) error {
	if s.actors == nil {
		return nil
	}
	kind, _, err := s.actors.ResolveActorKind(ctx, actor)
	if err != nil {
		return err
	}
	switch kind {
	case domain.ActorKindBuyer:
		_, err := requireBuyerCompanyAccess(ctx, s.actors, actor, fr.ShipperCompanyID)
		return err
	case domain.ActorKindCarrier:
		if _, err := s.requireCarrierActor(ctx, actor); err != nil {
			return err
		}
		if !domain.IsCarrierVisibleFreightRequestStatus(fr.Status) {
			return apperrors.NotFound("freight request not found")
		}
		return nil
	default:
		return apperrors.Forbidden("buyer authorization required")
	}
}
