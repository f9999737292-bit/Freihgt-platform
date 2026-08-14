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
