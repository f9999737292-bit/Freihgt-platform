package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/shipment-service/internal/domain"
	apperrors "github.com/freight-platform/shipment-service/internal/platform/errors"
	"github.com/freight-platform/shipment-service/internal/repository"
)

type OrderExecutionStore interface {
	GetAwardLinkByTransportOrderID(ctx context.Context, tenantID, transportOrderID uuid.UUID) (*domain.AwardTransportOrderLink, error)
	GetShipmentByTransportOrderID(ctx context.Context, tenantID, transportOrderID uuid.UUID) (*domain.Shipment, error)
	GetTransportOrderMeta(ctx context.Context, tenantID, orderID uuid.UUID) (orderNumber, status string, err error)
	ExecuteAwardOrder(ctx context.Context, params repository.ExecuteAwardOrderParams) (*repository.ExecuteAwardOrderResult, error)
	ListCarrierTransportOrders(ctx context.Context, filter domain.ListCarrierTransportOrdersFilter) ([]domain.CarrierTransportOrderListItem, int, error)
	ListBuyerTransportOrders(ctx context.Context, filter domain.ListBuyerTransportOrdersFilter) ([]domain.BuyerTransportOrderListItem, int, error)
	ListShipmentMilestones(ctx context.Context, tenantID, shipmentID uuid.UUID, limit int) ([]domain.ShipmentStatusHistory, error)
	ListShipmentPODDocuments(ctx context.Context, tenantID, shipmentID uuid.UUID) ([]domain.PODDocumentSummary, error)
}

type OrderExecutionService struct {
	execution OrderExecutionStore
	shipments *ShipmentService
}

func NewOrderExecutionService(execution OrderExecutionStore, shipments *ShipmentService) *OrderExecutionService {
	return &OrderExecutionService{execution: execution, shipments: shipments}
}

func (s *OrderExecutionService) ExecuteAwardOrder(
	ctx context.Context,
	tenantID, transportOrderID, carrierCompanyID uuid.UUID,
	in domain.ExecuteTransportOrderInput,
	transition domain.StatusTransitionContext,
) (*repository.ExecuteAwardOrderResult, error) {
	if err := domain.ValidateVerifiedTenant(tenantID); err != nil {
		return nil, err
	}
	if transportOrderID == uuid.Nil {
		return nil, apperrors.Validation("transport_order_id is required", map[string]any{"field": "transport_order_id"})
	}
	if carrierCompanyID == uuid.Nil {
		return nil, apperrors.Validation("carrier_company_id is required", map[string]any{"field": "carrier_company_id"})
	}
	if err := domain.ValidateExecuteTransportOrderInput(in); err != nil {
		return nil, err
	}
	if err := domain.ValidateStatusTransitionContext(transition); err != nil {
		return nil, err
	}

	return s.execution.ExecuteAwardOrder(ctx, repository.ExecuteAwardOrderParams{
		TenantID:              tenantID,
		TransportOrderID:      transportOrderID,
		CarrierCompanyID:      carrierCompanyID,
		ShipmentNumber:        in.ShipmentNumber,
		PlannedPickupAt:       in.PlannedPickupAt,
		PlannedDeliveryAt:     in.PlannedDeliveryAt,
		Transition:            transition,
		ImplicitCarrierAccept: true,
	})
}

func (s *OrderExecutionService) GetExecution(
	ctx context.Context,
	tenantID, transportOrderID uuid.UUID,
	actorCompanyID uuid.UUID,
	actorKind string,
) (*domain.OrderExecutionView, error) {
	if err := domain.ValidateVerifiedTenant(tenantID); err != nil {
		return nil, err
	}
	if transportOrderID == uuid.Nil {
		return nil, apperrors.Validation("transport_order_id is required", map[string]any{"field": "transport_order_id"})
	}
	if actorCompanyID == uuid.Nil {
		return nil, apperrors.Validation("company_id is required", map[string]any{"field": "company_id"})
	}

	link, err := s.execution.GetAwardLinkByTransportOrderID(ctx, tenantID, transportOrderID)
	if err != nil {
		return nil, err
	}
	if err := validateExecutionAccess(link, actorCompanyID, actorKind); err != nil {
		return nil, err
	}

	orderNumber, orderStatus, err := s.execution.GetTransportOrderMeta(ctx, tenantID, transportOrderID)
	if err != nil {
		return nil, err
	}

	shipment, err := s.execution.GetShipmentByTransportOrderID(ctx, tenantID, transportOrderID)
	if err != nil {
		var appErr *apperrors.AppError
		if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeNotFound {
			return nil, err
		}
		shipment = nil
	}

	view := &domain.OrderExecutionView{
		Link:        *link,
		OrderNumber: orderNumber,
		OrderStatus: orderStatus,
		Shipment:    shipment,
		Readiness:   domain.BuildExecutionReadiness(shipment),
		Provenance: domain.ExecutionProvenance{
			RfxEventID:    link.RfxEventID,
			RfxAwardID:    link.RfxAwardID,
			RfxResponseID: link.RfxResponseID,
			RfxLotID:      link.RfxLotID,
			Amount:        link.Amount,
			CurrencyCode:  link.CurrencyCode,
		},
	}
	if shipment != nil {
		milestones, mErr := s.execution.ListShipmentMilestones(ctx, tenantID, shipment.ID, 100)
		if mErr != nil {
			return nil, mErr
		}
		view.Milestones = milestones
		view.SLASignals = domain.BuildExecutionSLASignals(shipment, time.Now().UTC())
		pods, pErr := s.execution.ListShipmentPODDocuments(ctx, tenantID, shipment.ID)
		if pErr != nil {
			return nil, pErr
		}
		view.PODDocuments = pods
		if actorKind == domain.ExecutionActorCarrier {
			view.AllowedActions = domain.AllowedDriverMilestoneActions(shipment.Status)
		}
	}
	return view, nil
}

func (s *OrderExecutionService) ListBuyerTransportOrders(ctx context.Context, filter domain.ListBuyerTransportOrdersFilter) ([]domain.BuyerTransportOrderListItem, int, error) {
	if filter.Limit == 0 {
		filter.Limit = 20
	}
	if err := domain.ValidateListBuyerTransportOrdersFilter(filter); err != nil {
		return nil, 0, err
	}
	return s.execution.ListBuyerTransportOrders(ctx, filter)
}

func (s *OrderExecutionService) ListCarrierTransportOrders(ctx context.Context, filter domain.ListCarrierTransportOrdersFilter) ([]domain.CarrierTransportOrderListItem, int, error) {
	if filter.Limit == 0 {
		filter.Limit = 20
	}
	if err := domain.ValidateListCarrierTransportOrdersFilter(filter); err != nil {
		return nil, 0, err
	}
	return s.execution.ListCarrierTransportOrders(ctx, filter)
}

func (s *OrderExecutionService) StartExecution(
	ctx context.Context,
	tenantID, transportOrderID, carrierCompanyID uuid.UUID,
	transition domain.StatusTransitionContext,
) (*domain.Shipment, error) {
	if err := domain.ValidateVerifiedTenant(tenantID); err != nil {
		return nil, err
	}
	if err := domain.ValidateStatusTransitionContext(transition); err != nil {
		return nil, err
	}
	if carrierCompanyID == uuid.Nil {
		return nil, apperrors.Validation("carrier_company_id is required", map[string]any{"field": "carrier_company_id"})
	}

	link, err := s.execution.GetAwardLinkByTransportOrderID(ctx, tenantID, transportOrderID)
	if err != nil {
		return nil, err
	}
	if link.CarrierCompanyID != carrierCompanyID {
		return nil, apperrors.Forbidden("carrier company is not assigned to this transport order")
	}

	shipment, err := s.execution.GetShipmentByTransportOrderID(ctx, tenantID, transportOrderID)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidateStartExecution(shipment, carrierCompanyID); err != nil {
		return nil, err
	}

	current := shipment
	if current.Status == domain.ShipmentStatusDriverAssigned {
		updated, err := s.shipments.UpdateStatus(ctx, tenantID, current.ID, domain.UpdateShipmentStatusInput{
			Status: domain.ShipmentStatusPickupSlotBooked,
		}, transition)
		if err != nil {
			return nil, err
		}
		current = updated
	}
	if current.Status == domain.ShipmentStatusPickupSlotBooked {
		return s.shipments.UpdateStatus(ctx, tenantID, current.ID, domain.UpdateShipmentStatusInput{
			Status: domain.ShipmentStatusInPickup,
		}, transition)
	}
	return current, nil
}

func validateExecutionAccess(link *domain.AwardTransportOrderLink, actorCompanyID uuid.UUID, actorKind string) error {
	switch actorKind {
	case domain.ExecutionActorCarrier:
		if link.CarrierCompanyID != actorCompanyID {
			return apperrors.Forbidden("carrier cannot access another carrier's transport order")
		}
	case domain.ExecutionActorBuyer:
		if link.BuyerCompanyID != actorCompanyID {
			return apperrors.Forbidden("buyer cannot access another buyer's transport order")
		}
	default:
		return apperrors.Validation("actor kind is invalid", map[string]any{"field": "actor_kind"})
	}
	return nil
}
