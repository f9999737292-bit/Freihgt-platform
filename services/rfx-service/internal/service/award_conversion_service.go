package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/freight-platform/rfx-service/internal/client"
	"github.com/freight-platform/rfx-service/internal/domain"
	apperrors "github.com/freight-platform/rfx-service/internal/platform/errors"
)

type AwardConversionStore interface {
	LoadAwardConversionContext(ctx context.Context, proposalID, tenantID uuid.UUID) (domain.AwardConversionContext, error)
	GetAwardConversionByKey(ctx context.Context, tenantID uuid.UUID, idempotencyKey string) (*uuid.UUID, *uuid.UUID, error)
	SaveAwardTransportOrder(ctx context.Context, tenantID, awardID, transportOrderID, carrierID uuid.UUID, idempotencyKey string) error
	GetAwardByProposal(ctx context.Context, proposalID, tenantID uuid.UUID) (uuid.UUID, error)
}

type AwardConversionBidStore interface {
	CreateBid(ctx context.Context, in domain.CreateBidInput) (*domain.Bid, error)
	SubmitBid(ctx context.Context, id, tenantID uuid.UUID, submittedBy *uuid.UUID) (*domain.Bid, error)
	AcceptBid(ctx context.Context, id, tenantID uuid.UUID) (*domain.Bid, error)
}

type AwardConversionResult struct {
	Policy           string  `json:"policy"`
	Status           string  `json:"status"`
	ShipmentID       *string `json:"shipment_id,omitempty"`
	TransportOrderID *string `json:"transport_order_id,omitempty"`
	IdempotencyKey   string  `json:"idempotency_key"`
	Message          string  `json:"message,omitempty"`
}

type AwardConversionService struct {
	store    AwardConversionStore
	bids     AwardConversionBidStore
	shipment *client.ShipmentClient
}

func NewAwardConversionService(store AwardConversionStore, bids AwardConversionBidStore, shipment *client.ShipmentClient) *AwardConversionService {
	return &AwardConversionService{store: store, bids: bids, shipment: shipment}
}

func (s *AwardConversionService) ConvertApprovedAward(ctx context.Context, proposalID, awardID, tenantID, userID uuid.UUID) (*AwardConversionResult, error) {
	ctxData, err := s.store.LoadAwardConversionContext(ctx, proposalID, tenantID)
	if err != nil {
		return nil, err
	}
	policy := domain.TenderConversionPolicy(ctxData.RfxType)
	result := &AwardConversionResult{
		Policy: policy,
		Status: "SKIPPED",
	}

	if policy != domain.ConversionImmediateOrder {
		result.Message = "no automatic conversion for tender type"
		return result, nil
	}
	if ctxData.FreightReqID == nil || ctxData.TransportOrderID == nil {
		result.Message = "linked freight request with transport order required"
		return result, nil
	}

	idempotencyKey := conversionIdempotencyKey(awardID, ctxData.PrimaryCarrierID)
	result.IdempotencyKey = idempotencyKey

	if existingAward, existingTO, err := s.store.GetAwardConversionByKey(ctx, tenantID, idempotencyKey); err != nil {
		return nil, err
	} else if existingAward != nil && existingTO != nil {
		toStr := existingTO.String()
		result.Status = "COMPLETED"
		result.TransportOrderID = &toStr
		return result, nil
	}

	if s.shipment == nil {
		result.Status = "PENDING"
		result.Message = "shipment service not configured"
		return result, apperrors.Internal("shipment service unavailable", fmt.Errorf("missing shipment client"))
	}

	bidNumber := fmt.Sprintf("AWARD-%s", awardID.String()[:8])
	currency := ctxData.CurrencyCode
	bid, err := s.bids.CreateBid(ctx, domain.CreateBidInput{
		TenantID:         tenantID,
		FreightRequestID: *ctxData.FreightReqID,
		CarrierCompanyID: ctxData.PrimaryCarrierID,
		BidNumber:        bidNumber,
		CurrencyCode:     &currency,
		Items: []domain.CreateBidItemInput{{
			BaseAmount: ctxData.ExpectedCost,
		}},
	})
	if err != nil {
		return nil, err
	}
	bid, err = s.bids.SubmitBid(ctx, bid.ID, tenantID, &userID)
	if err != nil {
		return nil, err
	}
	bid, err = s.bids.AcceptBid(ctx, bid.ID, tenantID)
	if err != nil {
		return nil, err
	}

	shipmentNumber := fmt.Sprintf("SHP-AWARD-%s", awardID.String()[:8])
	shipment, err := s.shipment.CreateFromBid(ctx, tenantID, userID, client.CreateShipmentFromBidRequest{
		ShipmentNumber:   shipmentNumber,
		BidID:            bid.ID.String(),
		TransportOrderID: ctxData.TransportOrderID.String(),
	})
	if err != nil {
		result.Status = "FAILED"
		result.Message = err.Error()
		return result, apperrors.Internal("award conversion failed", err)
	}

	if err := s.store.SaveAwardTransportOrder(ctx, tenantID, awardID, *ctxData.TransportOrderID, ctxData.PrimaryCarrierID, idempotencyKey); err != nil {
		return nil, err
	}

	toStr := ctxData.TransportOrderID.String()
	shipmentID := shipment.ID
	result.Status = "COMPLETED"
	result.TransportOrderID = &toStr
	result.ShipmentID = &shipmentID
	return result, nil
}

func conversionIdempotencyKey(awardID, carrierID uuid.UUID) string {
	return fmt.Sprintf("award-%s-carrier-%s", awardID.String(), carrierID.String())
}
