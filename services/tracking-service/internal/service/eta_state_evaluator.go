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

type ETAStateEvaluator struct {
	repo   *repository.ETARepository
	Policy domain.ETAFreshnessPolicy
	mu     sync.Mutex
	last   map[string]string
}

func NewETAStateEvaluator(repo *repository.ETARepository, cfg config.Config) *ETAStateEvaluator {
	return &ETAStateEvaluator{
		repo: repo,
		Policy: domain.ETAFreshnessPolicy{
			FreshThreshold: cfg.ETAFreshnessPolicy.FreshThreshold,
			StaleThreshold: cfg.ETAFreshnessPolicy.StaleThreshold,
		},
		last: map[string]string{},
	}
}

func (e *ETAStateEvaluator) BuildStateFromObservation(
	tenantID, shipmentID uuid.UUID,
	targetType string,
	estimatedAt, sourceObservedAt, receivedAt time.Time,
	sourceType, providerCode string,
	qualityStatus string,
	now time.Time,
	completed bool,
) domain.ShipmentETAState {
	freshness, ageSeconds := domain.EvaluateETAFreshness(&sourceObservedAt, now, e.Policy)
	lag := receivedAt.Sub(sourceObservedAt)
	lagSeconds := int64(lag.Seconds())
	if lagSeconds < 0 {
		lagSeconds = 0
	}
	status := domain.DeriveETAStatus(true, freshness, completed)
	srcType := sourceType
	provider := providerCode
	return domain.ShipmentETAState{
		TenantID:           tenantID,
		ShipmentID:         shipmentID,
		TargetType:         targetType,
		Status:             status,
		EstimatedArrivalAt: &estimatedAt,
		SourceType:         &srcType,
		ProviderCode:       &provider,
		SourceObservedAt:   &sourceObservedAt,
		ReceivedAt:         &receivedAt,
		FreshnessStatus:    freshness,
		QualityStatus:      qualityStatus,
		AgeSeconds:         &ageSeconds,
		DeliveryLagSeconds: &lagSeconds,
		UpdatedAt:          now,
	}
}

func (e *ETAStateEvaluator) RefreshComputedState(state domain.ShipmentETAState, completed bool, now time.Time) domain.ShipmentETAState {
	hasObs := state.SourceObservedAt != nil
	freshness, ageSeconds := domain.EvaluateETAFreshness(state.SourceObservedAt, now, e.Policy)
	state.FreshnessStatus = freshness
	state.AgeSeconds = &ageSeconds
	state.Status = domain.DeriveETAStatus(hasObs, freshness, completed)
	if state.SourceObservedAt != nil && state.ReceivedAt != nil {
		lag := state.ReceivedAt.Sub(*state.SourceObservedAt)
		lagSeconds := int64(lag.Seconds())
		if lagSeconds < 0 {
			lagSeconds = 0
		}
		state.DeliveryLagSeconds = &lagSeconds
	}
	state.UpdatedAt = now
	return state
}

func (e *ETAStateEvaluator) ShouldReplaceCurrent(current *domain.ShipmentETAState, incomingSource string, incomingObserved, incomingReceived time.Time) bool {
	if current == nil || current.SourceObservedAt == nil || current.SourceType == nil || current.ReceivedAt == nil {
		return true
	}
	return repository.ShouldReplaceETAObservation(*current.SourceType, incomingSource, *current.SourceObservedAt, incomingObserved, *current.ReceivedAt, incomingReceived)
}

func (e *ETAStateEvaluator) RecordTransitionIfNeeded(ctx context.Context, tenantID, shipmentID uuid.UUID, targetType, newStatus string) {
	key := tenantID.String() + ":" + shipmentID.String() + ":" + targetType
	e.mu.Lock()
	prev := e.last[key]
	if prev == newStatus {
		e.mu.Unlock()
		return
	}
	e.last[key] = newStatus
	e.mu.Unlock()

	transitionType := etaTransitionFor(prev, newStatus)
	if transitionType == "" {
		return
	}
	var from *string
	if prev != "" {
		from = &prev
	}
	_ = e.repo.InsertETATransition(ctx, domain.ETAStateTransition{
		ID:             uuid.New(),
		TenantID:       tenantID,
		ShipmentID:     shipmentID,
		TargetType:     targetType,
		TransitionType: transitionType,
		FromStatus:     from,
		ToStatus:       newStatus,
		Metadata:       map[string]any{},
		OccurredAt:     time.Now().UTC(),
	})
}

func etaTransitionFor(from, to string) string {
	if from == "" && (to == domain.ETAStatusAvailable || to == domain.ETAStatusStale) {
		return domain.TransitionETABecameAvailable
	}
	switch {
	case from == domain.ETAStatusAvailable && to == domain.ETAStatusStale:
		return domain.TransitionETABecameStale
	case (from == domain.ETAStatusStale || from == domain.ETAStatusAvailable) && to == domain.ETAStatusExpired:
		return domain.TransitionETAExpired
	case (from == domain.ETAStatusStale || from == domain.ETAStatusExpired) && to == domain.ETAStatusAvailable:
		return domain.TransitionETARestored
	case to == domain.ETAStatusCompleted:
		return domain.TransitionETACompleted
	default:
		return ""
	}
}
