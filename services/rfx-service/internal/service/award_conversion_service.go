package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
	"github.com/freight-platform/rfx-service/internal/repository"
)

type awardConversionStore interface {
	ListAwardTransportOrdersByEvent(ctx context.Context, eventID, tenantID uuid.UUID) ([]domain.RfxAwardTransportOrder, error)
	ConvertAwardToTransportOrdersTransactional(
		ctx context.Context,
		event *domain.RfxEvent,
		award *domain.RfxAward,
		response *domain.RfxResponse,
		scopes []domain.AwardConversionScope,
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
		})
	}

	return store.ConvertAwardToTransportOrdersTransactional(ctx, event, award, response, scopes, actor.UserID, preCommit)
}
