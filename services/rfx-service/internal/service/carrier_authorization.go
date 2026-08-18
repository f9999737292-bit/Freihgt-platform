package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

func (s *RfxService) requireCarrierActor(ctx context.Context, actor domain.ActorContext) ([]uuid.UUID, error) {
	kind, carrierIDs, err := s.resolveActor(ctx, actor)
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

func (s *RfxService) resolveCarrierCompanyID(ctx context.Context, actor domain.ActorContext, requested uuid.UUID) (uuid.UUID, []uuid.UUID, error) {
	carrierIDs, err := s.requireCarrierActor(ctx, actor)
	if err != nil {
		return uuid.Nil, nil, err
	}
	carrierCompanyID, err := domain.ResolveCarrierCompanyID(requested, carrierIDs)
	if err != nil {
		return uuid.Nil, nil, err
	}
	return carrierCompanyID, carrierIDs, nil
}

func (s *RfxService) requireParticipantAccess(ctx context.Context, actor domain.ActorContext, eventID, carrierCompanyID uuid.UUID) error {
	exists, err := s.repo.ParticipantExists(ctx, eventID, carrierCompanyID, actor.TenantID)
	if err != nil {
		return err
	}
	if !exists {
		return apperrors.NotFound("rfx event not found")
	}
	return nil
}

func (s *RfxService) requireCarrierEventAccess(ctx context.Context, actor domain.ActorContext, eventID uuid.UUID, requestedCarrierCompanyID uuid.UUID) (uuid.UUID, error) {
	carrierCompanyID, _, err := s.resolveCarrierCompanyID(ctx, actor, requestedCarrierCompanyID)
	if err != nil {
		return uuid.Nil, err
	}
	event, err := s.repo.GetEventByID(ctx, eventID, actor.TenantID)
	if err != nil {
		return uuid.Nil, err
	}
	kind, _, err := s.resolveActor(ctx, actor)
	if err != nil {
		return uuid.Nil, err
	}
	if kind == domain.ActorKindBuyer {
		if _, err := s.requireOwnerCompanyAccess(ctx, actor, event.OwnerCompanyID); err != nil {
			return uuid.Nil, err
		}
		return carrierCompanyID, nil
	}
	if err := s.requireParticipantAccess(ctx, actor, eventID, carrierCompanyID); err != nil {
		return uuid.Nil, err
	}
	return carrierCompanyID, nil
}

func (s *RfxService) requireCarrierLotAccess(ctx context.Context, actor domain.ActorContext, lotID uuid.UUID, requestedCarrierCompanyID uuid.UUID) (uuid.UUID, *domain.LotOwnerContext, error) {
	lotCtx, err := s.repo.GetLotOwnerContext(ctx, lotID, actor.TenantID)
	if err != nil {
		return uuid.Nil, nil, err
	}
	carrierCompanyID, err := s.requireCarrierEventAccess(ctx, actor, lotCtx.RfxEventID, requestedCarrierCompanyID)
	if err != nil {
		return uuid.Nil, nil, err
	}
	return carrierCompanyID, lotCtx, nil
}
