package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/tracking-service/internal/domain"
	apperrors "github.com/freight-platform/tracking-service/internal/platform/errors"
	"github.com/freight-platform/tracking-service/internal/repository"
)

type SlotMilestoneContext struct {
	ShipmentStatus   string
	ActualPickupAt   *time.Time
	ActualDeliveryAt *time.Time
	PickupETA        domain.ETASnapshot
	DeliveryETA      domain.ETASnapshot
}

type SlotQueryService struct {
	repo     *repository.SlotRepository
	policy   domain.SlotPolicy
}

func NewSlotQueryService(repo *repository.SlotRepository) *SlotQueryService {
	return &SlotQueryService{
		repo:   repo,
		policy: domain.DefaultSlotPolicy(),
	}
}

func (s *SlotQueryService) GetShipmentSlots(ctx context.Context, tenantID, shipmentID uuid.UUID, milestone SlotMilestoneContext) (domain.ShipmentSlotSummary, error) {
	summary := domain.ShipmentSlotSummary{ShipmentID: shipmentID}
	pickupCompleted := milestone.ActualPickupAt != nil || isPickupMilestoneDone(milestone.ShipmentStatus)
	deliveryCompleted := milestone.ActualDeliveryAt != nil || isDeliveredStatus(milestone.ShipmentStatus)

	pickup, err := s.getTargetSummary(ctx, tenantID, shipmentID, domain.SlotTypePickup, milestone.PickupETA, milestone.ActualPickupAt, pickupCompleted)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return summary, apperrors.Internal("failed to load pickup slot", err)
	}
	if pickup != nil {
		summary.Pickup = pickup
	}

	delivery, err := s.getTargetSummary(ctx, tenantID, shipmentID, domain.SlotTypeDelivery, milestone.DeliveryETA, milestone.ActualDeliveryAt, deliveryCompleted)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return summary, apperrors.Internal("failed to load delivery slot", err)
	}
	if delivery != nil {
		summary.Delivery = delivery
	}

	return summary, nil
}

func (s *SlotQueryService) getTargetSummary(
	ctx context.Context,
	tenantID, shipmentID uuid.UUID,
	slotType string,
	eta domain.ETASnapshot,
	actualArrival *time.Time,
	milestoneCompleted bool,
) (*domain.SlotTargetSummary, error) {
	state, err := s.repo.GetSlotState(ctx, tenantID, shipmentID, slotType)
	if errors.Is(err, pgx.ErrNoRows) {
		summary := &domain.SlotTargetSummary{
			WindowStatus:      domain.SlotWindowUnavailable,
			QualityStatus:     domain.SlotQualityUnknown,
			ArrivalProjection: domain.SlotArrivalUnknown,
		}
		return summary, nil
	}
	if err != nil {
		return nil, err
	}
	summary := mapSlotTargetSummary(*state)
	domain.ApplySlotArrivalAssessment(summary, eta, actualArrival, milestoneCompleted, s.policy)
	return summary, nil
}

func (s *SlotQueryService) ListSlotHistory(ctx context.Context, tenantID, shipmentID uuid.UUID, slotType string, from, to *time.Time, limit, offset int) ([]domain.SlotRevision, int, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > domain.MaxSlotHistoryLimit {
		limit = domain.MaxSlotHistoryLimit
	}
	if offset < 0 {
		offset = 0
	}
	if slotType == "" {
		slotType = domain.SlotTypeDelivery
	}
	return s.repo.ListSlotHistory(ctx, tenantID, shipmentID, slotType, from, to, limit, offset)
}

func (s *SlotQueryService) LookupSlotSummaries(ctx context.Context, tenantID uuid.UUID, shipmentIDs []uuid.UUID, contextByShipment map[uuid.UUID]SlotMilestoneContext) (map[uuid.UUID]domain.ShipmentSlotSummary, error) {
	pickupStates, err := s.repo.LookupSlotStates(ctx, tenantID, shipmentIDs, domain.SlotTypePickup)
	if err != nil {
		return nil, apperrors.Internal("batch pickup slot lookup failed", err)
	}
	deliveryStates, err := s.repo.LookupSlotStates(ctx, tenantID, shipmentIDs, domain.SlotTypeDelivery)
	if err != nil {
		return nil, apperrors.Internal("batch delivery slot lookup failed", err)
	}

	out := make(map[uuid.UUID]domain.ShipmentSlotSummary, len(shipmentIDs))
	for _, shipmentID := range shipmentIDs {
		ctxData := contextByShipment[shipmentID]
		summary := domain.ShipmentSlotSummary{ShipmentID: shipmentID}
		pickupCompleted := ctxData.ActualPickupAt != nil || isPickupMilestoneDone(ctxData.ShipmentStatus)
		deliveryCompleted := ctxData.ActualDeliveryAt != nil || isDeliveredStatus(ctxData.ShipmentStatus)

		if state, ok := pickupStates[shipmentID]; ok {
			item := mapSlotTargetSummary(state)
			domain.ApplySlotArrivalAssessment(item, ctxData.PickupETA, ctxData.ActualPickupAt, pickupCompleted, s.policy)
			summary.Pickup = item
		} else {
			summary.Pickup = &domain.SlotTargetSummary{
				WindowStatus:      domain.SlotWindowUnavailable,
				QualityStatus:     domain.SlotQualityUnknown,
				ArrivalProjection: domain.SlotArrivalUnknown,
			}
		}

		if state, ok := deliveryStates[shipmentID]; ok {
			item := mapSlotTargetSummary(state)
			domain.ApplySlotArrivalAssessment(item, ctxData.DeliveryETA, ctxData.ActualDeliveryAt, deliveryCompleted, s.policy)
			summary.Delivery = item
		} else {
			summary.Delivery = &domain.SlotTargetSummary{
				WindowStatus:      domain.SlotWindowUnavailable,
				QualityStatus:     domain.SlotQualityUnknown,
				ArrivalProjection: domain.SlotArrivalUnknown,
			}
		}
		out[shipmentID] = summary
	}
	return out, nil
}

func mapSlotTargetSummary(state domain.ShipmentSlotState) *domain.SlotTargetSummary {
	summary := &domain.SlotTargetSummary{
		WindowStatus:      state.WindowStatus,
		SlotStatus:        state.SlotStatus,
		WindowStart:       state.WindowStart,
		WindowEnd:         state.WindowEnd,
		Timezone:          state.Timezone,
		SourceObservedAt:  state.SourceObservedAt,
		QualityStatus:     state.QualityStatus,
		BookedAt:          state.BookedAt,
		ConfirmedAt:       state.ConfirmedAt,
		ArrivalProjection: domain.SlotArrivalUnknown,
		ETARelation:       "unknown",
	}
	if state.SourceType != nil {
		summary.SourceType = state.SourceType
	}
	if state.ProviderCode != nil {
		summary.Provider = state.ProviderCode
	}
	if state.ProviderSlotID != nil {
		summary.ProviderSlotID = state.ProviderSlotID
	}
	return summary
}

func isPickupMilestoneDone(status string) bool {
	switch status {
	case "LOADED", "IN_TRANSIT", "ARRIVED_AT_CONSIGNEE", "UNLOADING", "DELIVERED",
		"DELIVERY_CONFIRMED", "DOCUMENTS_COMPLETED", "READY_FOR_BILLING",
		"INCLUDED_IN_BILLING_REGISTER", "FINANCIALLY_CLOSED":
		return true
	default:
		return false
	}
}
