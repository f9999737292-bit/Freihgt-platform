package service

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/tracking-service/internal/config"
	"github.com/freight-platform/tracking-service/internal/domain"
	"github.com/freight-platform/tracking-service/internal/repository"
)

type StateEvaluator struct {
	repo   *repository.TrackingRepository
	policy domain.FreshnessPolicy
	mu     sync.Mutex
	last   map[string]string
}

func NewStateEvaluator(repo *repository.TrackingRepository, cfg config.Config) *StateEvaluator {
	return &StateEvaluator{
		repo: repo,
		policy: domain.FreshnessPolicy{
			FreshThreshold: cfg.FreshnessPolicy.FreshThreshold,
			StaleThreshold: cfg.FreshnessPolicy.StaleThreshold,
		},
		last: map[string]string{},
	}
}

func (e *StateEvaluator) BuildStateFromEvent(binding *domain.ShipmentTrackingBinding, event domain.LocationEvent, quality string, now time.Time) domain.ShipmentTrackingState {
	freshness, ageSeconds := domain.EvaluateFreshness(&event.RecordedAt, now, e.policy)
	delay := event.ReceivedAt.Sub(event.RecordedAt)
	delaySeconds := int64(delay.Seconds())
	if delaySeconds < 0 {
		delaySeconds = 0
	}
	trackingStatus := domain.DeriveTrackingStatus(true, &event.RecordedAt, false, now, e.policy)
	provider := event.ProviderCode
	lat := event.Latitude
	lon := event.Longitude
	return domain.ShipmentTrackingState{
		TenantID:             binding.TenantID,
		ShipmentID:           binding.ShipmentID,
		TrackingStatus:       trackingStatus,
		ProviderCode:         &provider,
		LastLatitude:         &lat,
		LastLongitude:        &lon,
		LastRecordedAt:       &event.RecordedAt,
		LastReceivedAt:       &event.ReceivedAt,
		LastSpeedKph:         event.SpeedKph,
		LastHeadingDegrees:   event.HeadingDegrees,
		FreshnessStatus:      freshness,
		QualityStatus:        quality,
		AgeSeconds:           &ageSeconds,
		DeliveryDelaySeconds: &delaySeconds,
		UpdatedAt:            now,
	}
}

func (e *StateEvaluator) RefreshComputedState(state domain.ShipmentTrackingState, hasActiveBinding bool, now time.Time) domain.ShipmentTrackingState {
	freshness, ageSeconds := domain.EvaluateFreshness(state.LastRecordedAt, now, e.policy)
	status := domain.DeriveTrackingStatus(hasActiveBinding, state.LastRecordedAt, state.TrackingStatus == domain.TrackingStatusEnded, now, e.policy)
	state.TrackingStatus = status
	state.FreshnessStatus = freshness
	state.AgeSeconds = &ageSeconds
	if state.LastRecordedAt != nil && state.LastReceivedAt != nil {
		delay := state.LastReceivedAt.Sub(*state.LastRecordedAt)
		delaySeconds := int64(delay.Seconds())
		if delaySeconds < 0 {
			delaySeconds = 0
		}
		state.DeliveryDelaySeconds = &delaySeconds
	}
	state.UpdatedAt = now
	return state
}

func (e *StateEvaluator) RecordTransitionIfNeeded(ctx context.Context, tenantID, shipmentID uuid.UUID, newStatus string) error {
	key := tenantID.String() + ":" + shipmentID.String()
	e.mu.Lock()
	prev := e.last[key]
	if prev == newStatus {
		e.mu.Unlock()
		return nil
	}
	e.last[key] = newStatus
	e.mu.Unlock()

	transitionType := transitionFor(prev, newStatus)
	if transitionType == "" {
		return nil
	}
	var from *string
	if prev != "" {
		from = &prev
	}
	return e.repo.InsertStateTransition(ctx, domain.StateTransition{
		ID:             uuid.New(),
		TenantID:       tenantID,
		ShipmentID:     shipmentID,
		TransitionType: transitionType,
		FromStatus:     from,
		ToStatus:       newStatus,
		Metadata:       map[string]any{},
		OccurredAt:     time.Now().UTC(),
	})
}

func transitionFor(from, to string) string {
	if from == "" && (to == domain.TrackingStatusActive || to == domain.TrackingStatusAwaitingData) {
		return domain.TransitionTrackingStarted
	}
	switch {
	case from == domain.TrackingStatusActive && to == domain.TrackingStatusStale:
		return domain.TransitionTrackingBecameStale
	case (from == domain.TrackingStatusStale || from == domain.TrackingStatusActive) && to == domain.TrackingStatusLost:
		return domain.TransitionTrackingLost
	case (from == domain.TrackingStatusStale || from == domain.TrackingStatusLost) && to == domain.TrackingStatusActive:
		return domain.TransitionTrackingRestored
	case to == domain.TrackingStatusEnded:
		return domain.TransitionTrackingEnded
	default:
		return ""
	}
}
