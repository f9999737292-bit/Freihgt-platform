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

type PlannedTimes struct {
	PlannedPickupAt   *time.Time
	PlannedDeliveryAt *time.Time
	ActualDeliveryAt  *time.Time
	ActualPickupAt    *time.Time
	ShipmentStatus    string
}

type ETAQueryService struct {
	repo      *repository.ETARepository
	evaluator *ETAStateEvaluator
}

func NewETAQueryService(repo *repository.ETARepository, evaluator *ETAStateEvaluator) *ETAQueryService {
	return &ETAQueryService{repo: repo, evaluator: evaluator}
}

func (s *ETAQueryService) GetShipmentETA(ctx context.Context, tenantID, shipmentID uuid.UUID, planned PlannedTimes) (domain.ShipmentETASummary, error) {
	summary := domain.ShipmentETASummary{ShipmentID: shipmentID}
	completedDelivery := isDeliveredStatus(planned.ShipmentStatus) || planned.ActualDeliveryAt != nil
	completedPickup := planned.ActualPickupAt != nil

	delivery, err := s.getTargetSummary(ctx, tenantID, shipmentID, domain.TargetDelivery, planned.PlannedDeliveryAt, completedDelivery)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return summary, apperrors.Internal("failed to load delivery ETA", err)
	}
	if delivery != nil {
		summary.Delivery = delivery
	}

	pickup, err := s.getTargetSummary(ctx, tenantID, shipmentID, domain.TargetPickup, planned.PlannedPickupAt, completedPickup)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return summary, apperrors.Internal("failed to load pickup ETA", err)
	}
	if pickup != nil {
		summary.Pickup = pickup
	}

	return summary, nil
}

func (s *ETAQueryService) getTargetSummary(ctx context.Context, tenantID, shipmentID uuid.UUID, targetType string, plannedAt *time.Time, completed bool) (*domain.ETATargetSummary, error) {
	state, err := s.repo.GetETAState(ctx, tenantID, shipmentID, targetType)
	if errors.Is(err, pgx.ErrNoRows) {
		summary := &domain.ETATargetSummary{
			Status:          domain.ETAStatusUnavailable,
			FreshnessStatus: domain.ETAFreshnessUnknown,
			QualityStatus:   domain.ETAQualityUnknown,
			ArrivalProjection: domain.ArrivalUnknown,
		}
		if completed {
			summary.Status = domain.ETAStatusCompleted
		}
		domain.ApplyDeviation(summary, plannedAt, false)
		return summary, nil
	}
	if err != nil {
		return nil, err
	}
	refreshed := s.evaluator.RefreshComputedState(*state, completed, time.Now().UTC())
	if refreshed.Status != state.Status || refreshed.FreshnessStatus != state.FreshnessStatus {
		_ = s.repo.RefreshETAStateComputed(ctx, refreshed)
		s.evaluator.RecordTransitionIfNeeded(ctx, tenantID, shipmentID, targetType, refreshed.Status)
	}
	return mapETATargetSummary(refreshed, plannedAt), nil
}

func (s *ETAQueryService) ListETAHistory(ctx context.Context, tenantID, shipmentID uuid.UUID, targetType string, from, to *time.Time, limit, offset int) ([]domain.ETAObservation, int, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > domain.MaxETAHistoryLimit {
		limit = domain.MaxETAHistoryLimit
	}
	if offset < 0 {
		offset = 0
	}
	if targetType == "" {
		targetType = domain.TargetDelivery
	}
	return s.repo.ListETAHistory(ctx, tenantID, shipmentID, targetType, from, to, limit, offset)
}

func (s *ETAQueryService) LookupDeliveryETA(ctx context.Context, tenantID uuid.UUID, shipmentIDs []uuid.UUID, planned map[uuid.UUID]PlannedTimes) (map[uuid.UUID]domain.ETATargetSummary, error) {
	states, err := s.repo.LookupETAStates(ctx, tenantID, shipmentIDs, domain.TargetDelivery)
	if err != nil {
		return nil, apperrors.Internal("batch ETA lookup failed", err)
	}
	now := time.Now().UTC()
	out := make(map[uuid.UUID]domain.ETATargetSummary, len(shipmentIDs))
	for _, shipmentID := range shipmentIDs {
		pt := planned[shipmentID]
		completed := isDeliveredStatus(pt.ShipmentStatus) || pt.ActualDeliveryAt != nil
		state, ok := states[shipmentID]
		if !ok {
			summary := domain.ETATargetSummary{
				Status:            domain.ETAStatusUnavailable,
				FreshnessStatus:   domain.ETAFreshnessUnknown,
				QualityStatus:     domain.ETAQualityUnknown,
				ArrivalProjection: domain.ArrivalUnknown,
			}
			if completed {
				summary.Status = domain.ETAStatusCompleted
			}
			domain.ApplyDeviation(&summary, pt.PlannedDeliveryAt, false)
			out[shipmentID] = summary
			continue
		}
		refreshed := s.evaluator.RefreshComputedState(state, completed, now)
		summary := mapETATargetSummary(refreshed, pt.PlannedDeliveryAt)
		out[shipmentID] = *summary
	}
	return out, nil
}

func mapETATargetSummary(state domain.ShipmentETAState, plannedAt *time.Time) *domain.ETATargetSummary {
	summary := &domain.ETATargetSummary{
		Status:          state.Status,
		EstimatedArrivalAt: state.EstimatedArrivalAt,
		FreshnessStatus: state.FreshnessStatus,
		QualityStatus:   state.QualityStatus,
		AgeSeconds:      state.AgeSeconds,
		DeliveryLagSeconds: state.DeliveryLagSeconds,
	}
	if state.SourceType != nil {
		summary.SourceType = state.SourceType
	}
	if state.ProviderCode != nil {
		summary.Provider = state.ProviderCode
	}
	if state.SourceObservedAt != nil {
		summary.SourceObservedAt = state.SourceObservedAt
	}
	if state.ReceivedAt != nil {
		summary.ReceivedAt = state.ReceivedAt
	}
	usable := domain.ETAUsableForRisk(state.Status)
	domain.ApplyDeviation(summary, plannedAt, usable)
	return summary
}

func isDeliveredStatus(status string) bool {
	switch status {
	case "DELIVERED", "DELIVERY_CONFIRMED", "DOCUMENTS_COMPLETED", "READY_FOR_BILLING", "COMPLETED", "CLOSED":
		return true
	default:
		return false
	}
}
