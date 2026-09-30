package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/tracking-service/internal/config"
	"github.com/freight-platform/tracking-service/internal/domain"
	"github.com/freight-platform/tracking-service/internal/metrics"
	apperrors "github.com/freight-platform/tracking-service/internal/platform/errors"
	"github.com/freight-platform/tracking-service/internal/provider"
	"github.com/freight-platform/tracking-service/internal/repository"
)

type ETAIngestService struct {
	trackingRepo *repository.TrackingRepository
	etaRepo      *repository.ETARepository
	execution    *repository.ExecutionTrackingRepository
	registry     *provider.ETARegistry
	evaluator    *ETAStateEvaluator
	log          *slog.Logger
	metrics      *metrics.Collector
}

func NewETAIngestService(
	trackingRepo *repository.TrackingRepository,
	etaRepo *repository.ETARepository,
	registry *provider.ETARegistry,
	cfg config.Config,
	evaluator *ETAStateEvaluator,
	log *slog.Logger,
	m *metrics.Collector,
) *ETAIngestService {
	return &ETAIngestService{
		trackingRepo: trackingRepo,
		etaRepo:      etaRepo,
		registry:     registry,
		evaluator:    evaluator,
		log:          log,
		metrics:      m,
	}
}

type ETAIngestResult struct {
	Received     int `json:"received"`
	Accepted     int `json:"accepted"`
	Deduplicated int `json:"deduplicated"`
	Rejected     int `json:"rejected"`
}

func (s *ETAIngestService) SetExecutionTracking(repo *repository.ExecutionTrackingRepository) {
	s.execution = repo
}

func (s *ETAIngestService) IngestProviderETA(ctx context.Context, providerCode string, payload provider.ProviderPayload) (ETAIngestResult, error) {
	adapter, ok := s.registry.Get(providerCode)
	if !ok {
		return ETAIngestResult{}, apperrors.Validation("unsupported provider", map[string]any{"provider": providerCode})
	}
	normalized, err := adapter.NormalizeETA(ctx, payload)
	if err != nil {
		s.metrics.IncETARejected()
		return ETAIngestResult{}, apperrors.Validation("invalid provider ETA payload", map[string]any{"provider": providerCode})
	}

	result := ETAIngestResult{Received: len(normalized)}
	now := time.Now().UTC()

	for _, item := range normalized {
		if item.ProviderDeviceID == "" {
			result.Rejected++
			s.metrics.IncETARejected()
			continue
		}
		if _, err := repository.ParseTargetType(item.TargetType); err != nil {
			result.Rejected++
			s.metrics.IncETARejected()
			continue
		}
		if !domain.IsEnabledSourceType(item.SourceType) {
			result.Rejected++
			s.metrics.IncETARejected()
			continue
		}
		if item.SourceObservedAt.After(now.Add(5 * time.Minute)) {
			result.Rejected++
			s.metrics.IncETARejected()
			continue
		}
		if item.TargetType == domain.TargetExecutionStop {
			s.ingestExecutionStopETA(ctx, providerCode, item, now, &result)
			continue
		}

		binding, err := s.trackingRepo.FindActiveBindingByDeviceAnyTenant(ctx, providerCode, item.ProviderDeviceID)
		if errors.Is(err, pgx.ErrNoRows) {
			result.Rejected++
			s.metrics.IncETARejected()
			continue
		}
		if err != nil {
			result.Rejected++
			s.metrics.IncETARejected()
			continue
		}

		freshness, _ := domain.EvaluateETAFreshness(&item.SourceObservedAt, now, s.evaluator.Policy)
		lag := now.Sub(item.SourceObservedAt)
		quality, reasons := domain.EvaluateETAQuality(freshness, item.SourceType, lag, item.ProviderConfidence)

		dedupKey := repository.BuildETADedupKey(providerCode, item.TargetType, binding.ShipmentID, item.EstimatedArrivalAt, item.SourceObservedAt, item.ProviderEventID)
		providerCodeCopy := providerCode
		obs := domain.ETAObservation{
			ID:                 uuid.New(),
			TenantID:           binding.TenantID,
			ShipmentID:         binding.ShipmentID,
			TargetType:         item.TargetType,
			TargetReference:    item.TargetReference,
			EstimatedArrivalAt: item.EstimatedArrivalAt.UTC(),
			SourceType:         item.SourceType,
			ProviderCode:       &providerCodeCopy,
			ProviderEventID:    item.ProviderEventID,
			DedupKey:           dedupKey,
			SourceObservedAt:   item.SourceObservedAt.UTC(),
			ReceivedAt:         now,
			QualityStatus:      quality,
			QualityReasons:     reasons,
			ProviderConfidence: item.ProviderConfidence,
		}

		inserted, err := s.etaRepo.InsertETAObservation(ctx, obs)
		if err != nil {
			result.Rejected++
			s.metrics.IncETARejected()
			continue
		}
		if !inserted {
			result.Deduplicated++
			s.metrics.IncETADeduplicated()
			continue
		}
		result.Accepted++
		s.metrics.IncETAReceived()
		s.metrics.ObserveETAIngestionLag(now.Sub(item.SourceObservedAt))

		current, _ := s.etaRepo.GetETAState(ctx, binding.TenantID, binding.ShipmentID, item.TargetType)
		replace := s.evaluator.ShouldReplaceCurrent(current, item.SourceType, item.SourceObservedAt, now)
		state := s.evaluator.BuildStateFromObservation(
			binding.TenantID, binding.ShipmentID, item.TargetType,
			item.EstimatedArrivalAt, item.SourceObservedAt, now,
			item.SourceType, providerCode, quality, now, false,
		)
		if err := s.etaRepo.UpsertETAStateIfNewer(ctx, state, replace); err != nil {
			s.log.Warn("eta state upsert failed", slog.String("shipment_id", binding.ShipmentID.String()))
		} else if replace {
			s.evaluator.RecordTransitionIfNeeded(ctx, binding.TenantID, binding.ShipmentID, item.TargetType, state.Status)
		}
	}

	return result, nil
}

func (s *ETAIngestService) ingestExecutionStopETA(ctx context.Context, providerCode string, item provider.NormalizedETAInput, now time.Time, result *ETAIngestResult) {
	reject := func() {
		result.Rejected++
		s.metrics.IncETARejected()
	}
	if s.execution == nil || item.ExecutionStopID == nil || *item.ExecutionStopID == uuid.Nil {
		reject()
		return
	}
	bindings, err := s.trackingRepo.ListActiveBindingsByDevice(ctx, providerCode, item.ProviderDeviceID)
	if err != nil || len(bindings) == 0 {
		reject()
		return
	}
	driverIDs := make([]uuid.UUID, 0, len(bindings))
	vehicleIDs := make([]uuid.UUID, 0, len(bindings))
	for _, binding := range bindings {
		if binding.DriverID != nil {
			driverIDs = append(driverIDs, *binding.DriverID)
		}
		if binding.VehicleID != nil {
			vehicleIDs = append(vehicleIDs, *binding.VehicleID)
		}
	}
	targets, err := s.execution.ListActiveTargetsForActors(ctx, driverIDs, vehicleIDs)
	if err != nil || len(targets) == 0 {
		reject()
		return
	}
	seen := map[uuid.UUID]repository.ExecutionTrackingState{}
	for _, target := range targets {
		seen[target.ExecutionID] = target
	}
	if len(seen) != 1 {
		reject()
		return
	}
	var target repository.ExecutionTrackingState
	for _, value := range seen {
		target = value
	}
	if target.LiveETAStopID == nil || *target.LiveETAStopID != *item.ExecutionStopID {
		reject()
		return
	}
	if !actorMatchesTarget(target, driverIDs, vehicleIDs) {
		reject()
		return
	}
	freshness, _ := domain.EvaluateETAFreshness(&item.SourceObservedAt, now, s.evaluator.Policy)
	lag := now.Sub(item.SourceObservedAt)
	quality, reasons := domain.EvaluateETAQuality(freshness, item.SourceType, lag, item.ProviderConfidence)
	status := domain.DeriveETAStatus(true, freshness, false)
	age := int64(lag.Seconds())
	if age < 0 {
		age = 0
	}
	dedupKey := repository.BuildExecutionStopETADedupKey(providerCode, item.TargetType, *item.ExecutionStopID, item.EstimatedArrivalAt, item.SourceObservedAt, item.ProviderEventID)
	providerCodeCopy := providerCode
	stopID := *item.ExecutionStopID
	executionID := target.ExecutionID
	obs := domain.ETAObservation{
		ID:                 uuid.New(),
		TenantID:           target.OperatingTenantID,
		TargetType:         domain.TargetExecutionStop,
		TargetReference:    item.TargetReference,
		EstimatedArrivalAt: item.EstimatedArrivalAt.UTC(),
		SourceType:         item.SourceType,
		ProviderCode:       &providerCodeCopy,
		ProviderEventID:    item.ProviderEventID,
		DedupKey:           dedupKey,
		SourceObservedAt:   item.SourceObservedAt.UTC(),
		ReceivedAt:         now,
		QualityStatus:      quality,
		QualityReasons:     reasons,
		ProviderConfidence: item.ProviderConfidence,
		ExecutionID:        &executionID,
		ExecutionStopID:    &stopID,
	}
	tx, err := s.execution.Begin(ctx)
	if err != nil {
		reject()
		return
	}
	defer tx.Rollback(ctx)
	inserted, err := s.etaRepo.InsertETAObservationTx(ctx, tx, obs)
	if err != nil {
		reject()
		return
	}
	if !inserted {
		result.Deduplicated++
		s.metrics.IncETADeduplicated()
		return
	}
	etaState := repository.ExecutionStopETAState{
		OperatingTenantID:  target.OperatingTenantID,
		ExecutionID:        target.ExecutionID,
		ExecutionStopID:    stopID,
		Status:             status,
		EstimatedArrivalAt: &obs.EstimatedArrivalAt,
		SourceType:         &item.SourceType,
		ProviderCode:       &providerCodeCopy,
		SourceObservedAt:   &obs.SourceObservedAt,
		ReceivedAt:         &now,
		FreshnessStatus:    freshness,
		QualityStatus:      quality,
		QualityReasons:     reasons,
		AgeSeconds:         &age,
		PlannedArrival:     target.PlannedArrival,
	}
	if err := s.execution.UpsertExecutionStopETATx(ctx, tx, etaState); err != nil {
		reject()
		return
	}
	if err := tx.Commit(ctx); err != nil {
		reject()
		return
	}
	result.Accepted++
	s.metrics.IncETAReceived()
	s.metrics.ObserveETAIngestionLag(now.Sub(item.SourceObservedAt))
}

func actorMatchesTarget(target repository.ExecutionTrackingState, driverIDs, vehicleIDs []uuid.UUID) bool {
	if target.DriverID != nil {
		for _, id := range driverIDs {
			if id == *target.DriverID {
				return true
			}
		}
	}
	if target.VehicleID != nil {
		for _, id := range vehicleIDs {
			if id == *target.VehicleID {
				return true
			}
		}
	}
	return false
}
