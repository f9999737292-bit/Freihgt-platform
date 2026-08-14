package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/tracking-service/internal/config"
	"github.com/freight-platform/tracking-service/internal/domain"
	apperrors "github.com/freight-platform/tracking-service/internal/platform/errors"
	"github.com/freight-platform/tracking-service/internal/repository"
)

type TrackingQueryService struct {
	repo      *repository.TrackingRepository
	evaluator *StateEvaluator
	policy    domain.FreshnessPolicy
}

func NewTrackingQueryService(repo *repository.TrackingRepository, evaluator *StateEvaluator, cfg config.Config) *TrackingQueryService {
	return &TrackingQueryService{
		repo:      repo,
		evaluator: evaluator,
		policy: domain.FreshnessPolicy{
			FreshThreshold: cfg.FreshnessPolicy.FreshThreshold,
			StaleThreshold: cfg.FreshnessPolicy.StaleThreshold,
		},
	}
}

func (s *TrackingQueryService) GetTrackingSummary(ctx context.Context, tenantID, shipmentID uuid.UUID) (domain.TrackingSummary, error) {
	hasBinding, err := s.repo.HasActiveBinding(ctx, tenantID, shipmentID)
	if err != nil {
		return domain.TrackingSummary{}, apperrors.Internal("failed to load tracking binding", err)
	}

	state, err := s.repo.GetTrackingState(ctx, tenantID, shipmentID)
	if err != nil && !isNotFound(err) {
		return domain.TrackingSummary{}, apperrors.Internal("failed to load tracking state", err)
	}

	now := time.Now().UTC()
	if state == nil {
		status := domain.TrackingStatusNotConfigured
		if hasBinding {
			status = domain.TrackingStatusAwaitingData
		}
		return domain.TrackingSummary{
			ShipmentID:     shipmentID,
			TrackingStatus: status,
			Freshness:      domain.FreshnessSummary{Status: domain.FreshnessUnknown},
			Quality:        domain.QualitySummary{Status: domain.QualityUnknown},
		}, nil
	}

	refreshed := s.evaluator.RefreshComputedState(*state, hasBinding, now)
	if refreshed.TrackingStatus != state.TrackingStatus ||
		refreshed.FreshnessStatus != state.FreshnessStatus ||
		(refreshed.AgeSeconds != nil && state.AgeSeconds != nil && *refreshed.AgeSeconds != *state.AgeSeconds) {
		_ = s.repo.RefreshTrackingStateComputed(ctx, tenantID, shipmentID, refreshed)
		_ = s.evaluator.RecordTransitionIfNeeded(ctx, tenantID, shipmentID, refreshed.TrackingStatus)
		state = &refreshed
	}

	return mapTrackingSummary(shipmentID, *state), nil
}

func (s *TrackingQueryService) ListLocationHistory(ctx context.Context, tenantID, shipmentID uuid.UUID, from, to *time.Time, limit, offset int) ([]domain.LocationEvent, int, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > domain.MaxLocationHistoryLimit {
		limit = domain.MaxLocationHistoryLimit
	}
	if offset < 0 {
		offset = 0
	}
	return s.repo.ListLocationHistory(ctx, tenantID, shipmentID, from, to, limit, offset)
}

func (s *TrackingQueryService) LookupTrackingStates(ctx context.Context, tenantID uuid.UUID, shipmentIDs []uuid.UUID) (map[uuid.UUID]domain.TrackingSummary, error) {
	states, err := s.repo.LookupTrackingStates(ctx, tenantID, shipmentIDs)
	if err != nil {
		return nil, apperrors.Internal("batch tracking lookup failed", err)
	}
	bindings, err := s.repo.BatchActiveBindings(ctx, tenantID, shipmentIDs)
	if err != nil {
		return nil, apperrors.Internal("batch tracking binding lookup failed", err)
	}
	now := time.Now().UTC()
	out := make(map[uuid.UUID]domain.TrackingSummary, len(shipmentIDs))
	for _, shipmentID := range shipmentIDs {
		hasBinding := bindings[shipmentID]
		state, ok := states[shipmentID]
		if !ok {
			status := domain.TrackingStatusNotConfigured
			if hasBinding {
				status = domain.TrackingStatusAwaitingData
			}
			out[shipmentID] = domain.TrackingSummary{
				ShipmentID:     shipmentID,
				TrackingStatus: status,
				Freshness:      domain.FreshnessSummary{Status: domain.FreshnessUnknown},
				Quality:        domain.QualitySummary{Status: domain.QualityUnknown},
			}
			continue
		}
		refreshed := s.evaluator.RefreshComputedState(state, hasBinding, now)
		out[shipmentID] = mapTrackingSummary(shipmentID, refreshed)
	}
	return out, nil
}

func mapTrackingSummary(shipmentID uuid.UUID, state domain.ShipmentTrackingState) domain.TrackingSummary {
	summary := domain.TrackingSummary{
		ShipmentID:           shipmentID,
		TrackingStatus:       state.TrackingStatus,
		LastRecordedAt:       state.LastRecordedAt,
		LastReceivedAt:       state.LastReceivedAt,
		SpeedKph:             state.LastSpeedKph,
		HeadingDegrees:       state.LastHeadingDegrees,
		DeliveryDelaySeconds: state.DeliveryDelaySeconds,
		Freshness: domain.FreshnessSummary{
			Status:     state.FreshnessStatus,
			AgeSeconds: state.AgeSeconds,
		},
		Quality: domain.QualitySummary{Status: state.QualityStatus},
	}
	if state.ProviderCode != nil {
		summary.Provider = state.ProviderCode
	}
	if state.LastLatitude != nil && state.LastLongitude != nil && state.LastRecordedAt != nil && state.AgeSeconds != nil {
		summary.LastKnownPosition = &domain.LastKnownPosition{
			Latitude:   *state.LastLatitude,
			Longitude:  *state.LastLongitude,
			RecordedAt: *state.LastRecordedAt,
			AgeSeconds: *state.AgeSeconds,
		}
	}
	return summary
}

func isNotFound(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
