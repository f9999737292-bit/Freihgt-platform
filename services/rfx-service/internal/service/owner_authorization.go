package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

type CompanyMembershipResolver interface {
	ActorResolver
	ListBuyerCompanyIDs(ctx context.Context, actor domain.ActorContext) ([]uuid.UUID, error)
	ListUserRoleCodes(ctx context.Context, tenantID, userID uuid.UUID) ([]string, error)
}

func (s *RfxService) listBuyerCompanyIDs(ctx context.Context, actor domain.ActorContext) ([]uuid.UUID, error) {
	if actor.UserID == uuid.Nil {
		return nil, apperrors.Forbidden("user context is required")
	}
	if s.actors == nil {
		return nil, apperrors.Forbidden("buyer company membership is required")
	}
	resolver, ok := s.actors.(CompanyMembershipResolver)
	if !ok {
		return nil, apperrors.Forbidden("buyer company membership is required")
	}
	return resolver.ListBuyerCompanyIDs(ctx, actor)
}

func (s *RfxService) requireBuyerActor(ctx context.Context, actor domain.ActorContext) error {
	kind, _, err := s.resolveActor(ctx, actor)
	if err != nil {
		return err
	}
	if kind != domain.ActorKindBuyer {
		return apperrors.Forbidden("buyer authorization required")
	}
	if actor.UserID == uuid.Nil {
		return apperrors.Forbidden("user context is required")
	}
	resolver, ok := s.actors.(CompanyMembershipResolver)
	if ok {
		roles, err := resolver.ListUserRoleCodes(ctx, actor.TenantID, actor.UserID)
		if err != nil {
			return err
		}
		if !domain.HasBuyerRole(roles) {
			return apperrors.Forbidden("buyer role is required")
		}
	}
	return nil
}

func (s *RfxService) requireOwnerCompanyAccess(ctx context.Context, actor domain.ActorContext, ownerCompanyID uuid.UUID) (uuid.UUID, error) {
	if err := s.requireBuyerActor(ctx, actor); err != nil {
		return uuid.Nil, err
	}
	buyerCompanyIDs, err := s.listBuyerCompanyIDs(ctx, actor)
	if err != nil {
		return uuid.Nil, err
	}
	if !domain.ContainsCompanyID(buyerCompanyIDs, ownerCompanyID) {
		return uuid.Nil, apperrors.NotFound("rfx event not found")
	}
	return ownerCompanyID, nil
}

func (s *RfxService) requireOwnerCompanyAccessForEvent(ctx context.Context, actor domain.ActorContext, event *domain.RfxEvent) error {
	_, err := s.requireOwnerCompanyAccess(ctx, actor, event.OwnerCompanyID)
	return err
}

func (s *RfxService) applyBuyerListScope(ctx context.Context, actor domain.ActorContext, filter *domain.ListRfxEventsFilter) error {
	if err := s.requireBuyerActor(ctx, actor); err != nil {
		return err
	}
	buyerCompanyIDs, err := s.listBuyerCompanyIDs(ctx, actor)
	if err != nil {
		return err
	}
	if len(buyerCompanyIDs) == 0 {
		return apperrors.Forbidden("buyer company membership is required")
	}
	if filter.OwnerCompanyID != nil {
		if !domain.ContainsCompanyID(buyerCompanyIDs, *filter.OwnerCompanyID) {
			filter.OwnerCompanyIDs = []uuid.UUID{uuid.Nil}
			return nil
		}
		filter.OwnerCompanyIDs = []uuid.UUID{*filter.OwnerCompanyID}
		return nil
	}
	filter.OwnerCompanyIDs = buyerCompanyIDs
	return nil
}

func (s *RfxService) resolveCreateOwnerCompanyID(ctx context.Context, actor domain.ActorContext, requested uuid.UUID) (uuid.UUID, error) {
	if err := s.requireBuyerActor(ctx, actor); err != nil {
		return uuid.Nil, err
	}
	buyerCompanyIDs, err := s.listBuyerCompanyIDs(ctx, actor)
	if err != nil {
		return uuid.Nil, err
	}
	return domain.ResolveBuyerCompanyID(requested, buyerCompanyIDs)
}
