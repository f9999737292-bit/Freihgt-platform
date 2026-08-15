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

type IngestService struct {
	repo      *repository.TrackingRepository
	registry  *provider.Registry
	policy    domain.FreshnessPolicy
	evaluator *StateEvaluator
	log       *slog.Logger
	metrics   *metrics.Collector
}

func NewIngestService(repo *repository.TrackingRepository, registry *provider.Registry, cfg config.Config, evaluator *StateEvaluator, log *slog.Logger, m *metrics.Collector) *IngestService {
	return &IngestService{
		repo:      repo,
		registry:  registry,
		policy:    domain.FreshnessPolicy{FreshThreshold: cfg.FreshnessPolicy.FreshThreshold, StaleThreshold: cfg.FreshnessPolicy.StaleThreshold},
		evaluator: evaluator,
		log:       log,
		metrics:   m,
	}
}

type IngestResult struct {
	Received   int `json:"received"`
	Accepted   int `json:"accepted"`
	Deduplicated int `json:"deduplicated"`
	Rejected   int `json:"rejected"`
}

func (s *IngestService) IngestDriverMobileLocation(
	ctx context.Context,
	tenantID, shipmentID, driverID uuid.UUID,
	vehicleID *uuid.UUID,
	payload provider.ProviderPayload,
) (IngestResult, error) {
	if err := s.repo.EnsureActiveDriverMobileBinding(ctx, tenantID, shipmentID, driverID, vehicleID); err != nil {
		return IngestResult{}, apperrors.Validation("tracking binding failed", map[string]any{"reason": err.Error()})
	}
	return s.IngestProviderLocations(ctx, "driver_mobile", payload)
}

func (s *IngestService) IngestProviderLocations(ctx context.Context, providerCode string, payload provider.ProviderPayload) (IngestResult, error) {
	adapter, ok := s.registry.Get(providerCode)
	if !ok {
		return IngestResult{}, apperrors.Validation("unsupported provider", map[string]any{"provider": providerCode})
	}
	normalized, err := adapter.Normalize(ctx, payload)
	if err != nil {
		s.metrics.IncRejected()
		return IngestResult{}, apperrors.Validation("invalid provider payload", map[string]any{"provider": providerCode})
	}

	result := IngestResult{Received: len(normalized)}
	now := time.Now().UTC()

	for _, item := range normalized {
		if item.ProviderDeviceID == "" {
			result.Rejected++
			s.metrics.IncRejected()
			continue
		}
		if err := domain.ValidateCoordinates(item.Latitude, item.Longitude); err != nil {
			result.Rejected++
			s.metrics.IncRejected()
			continue
		}
		if item.RecordedAt.After(now.Add(5 * time.Minute)) {
			result.Rejected++
			s.metrics.IncRejected()
			continue
		}

		binding, err := s.repo.FindActiveBindingByDeviceAnyTenant(ctx, providerCode, item.ProviderDeviceID)
		if errors.Is(err, pgx.ErrNoRows) {
			result.Rejected++
			s.metrics.IncRejected()
			continue
		}
		if err != nil {
			result.Rejected++
			s.metrics.IncRejected()
			s.log.Warn("tracking binding lookup failed", slog.String("provider", providerCode), slog.String("device", item.ProviderDeviceID))
			continue
		}

		prev, _ := s.repo.GetPreviousLocationEvent(ctx, binding.TenantID, binding.ShipmentID, item.RecordedAt)
		var prevLat, prevLon *float64
		var prevRecorded *time.Time
		if prev != nil {
			prevLat = &prev.Latitude
			prevLon = &prev.Longitude
			prevRecorded = &prev.RecordedAt
		}
		receiptDelay := now.Sub(item.RecordedAt)
		freshness, _ := domain.EvaluateFreshness(&item.RecordedAt, now, s.policy)
		quality, qualityReason := domain.EvaluateQuality(
			freshness, item.AccuracyMeters, receiptDelay,
			prevLat, prevLon, prevRecorded,
			item.Latitude, item.Longitude, item.RecordedAt,
		)

		dedupKey := repository.BuildDedupKey(providerCode, item.ProviderDeviceID, item.RecordedAt, item.Latitude, item.Longitude)
		event := domain.LocationEvent{
			ID:               uuid.New(),
			TenantID:         binding.TenantID,
			ShipmentID:       binding.ShipmentID,
			VehicleID:        binding.VehicleID,
			DriverID:         binding.DriverID,
			ProviderCode:     providerCode,
			ProviderDeviceID: item.ProviderDeviceID,
			ProviderEventID:  item.ProviderEventID,
			DedupKey:         dedupKey,
			Latitude:         item.Latitude,
			Longitude:        item.Longitude,
			RecordedAt:       item.RecordedAt.UTC(),
			ReceivedAt:       now,
			SpeedKph:         item.SpeedKph,
			HeadingDegrees:   item.HeadingDegrees,
			AccuracyMeters:   item.AccuracyMeters,
			AltitudeMeters:   item.AltitudeMeters,
			SourceType:       item.SourceType,
			QualityStatus:    quality,
			QualityReason:    qualityReason,
		}

		inserted, err := s.repo.InsertLocationEvent(ctx, event)
		if err != nil {
			result.Rejected++
			s.metrics.IncRejected()
			continue
		}
		if !inserted {
			result.Deduplicated++
			s.metrics.IncDeduplicated()
			continue
		}
		result.Accepted++
		s.metrics.IncReceived()
		s.metrics.ObserveIngestionLag(now.Sub(item.RecordedAt))

		state := s.evaluator.BuildStateFromEvent(binding, event, quality, now)
		if err := s.repo.UpsertTrackingStateIfNewer(ctx, state); err != nil {
			s.log.Warn("tracking state upsert failed", slog.String("shipment_id", binding.ShipmentID.String()))
		}
		_ = s.evaluator.RecordTransitionIfNeeded(ctx, binding.TenantID, binding.ShipmentID, state.TrackingStatus)
	}

	return result, nil
}
