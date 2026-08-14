package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/tracking-service/internal/domain"
	"github.com/freight-platform/tracking-service/internal/metrics"
	apperrors "github.com/freight-platform/tracking-service/internal/platform/errors"
	"github.com/freight-platform/tracking-service/internal/provider"
	"github.com/freight-platform/tracking-service/internal/repository"
)

type SlotIngestService struct {
	trackingRepo *repository.TrackingRepository
	slotRepo     *repository.SlotRepository
	registry     *provider.SlotRegistry
	evaluator    *SlotStateEvaluator
	log          *slog.Logger
	metrics      *metrics.Collector
}

func NewSlotIngestService(
	trackingRepo *repository.TrackingRepository,
	slotRepo *repository.SlotRepository,
	registry *provider.SlotRegistry,
	evaluator *SlotStateEvaluator,
	log *slog.Logger,
	m *metrics.Collector,
) *SlotIngestService {
	return &SlotIngestService{
		trackingRepo: trackingRepo,
		slotRepo:     slotRepo,
		registry:     registry,
		evaluator:    evaluator,
		log:          log,
		metrics:      m,
	}
}

type SlotIngestResult struct {
	Received     int `json:"received"`
	Accepted     int `json:"accepted"`
	Deduplicated int `json:"deduplicated"`
	Rejected     int `json:"rejected"`
}

func (s *SlotIngestService) IngestProviderSlots(ctx context.Context, providerCode string, payload provider.ProviderPayload) (SlotIngestResult, error) {
	adapter, ok := s.registry.Get(providerCode)
	if !ok {
		return SlotIngestResult{}, apperrors.Validation("unsupported provider", map[string]any{"provider": providerCode})
	}
	normalized, err := adapter.NormalizeSlot(ctx, payload)
	if err != nil {
		s.metrics.IncSlotRejected()
		return SlotIngestResult{}, apperrors.Validation("invalid provider slot payload", map[string]any{"provider": providerCode})
	}

	result := SlotIngestResult{Received: len(normalized)}
	now := time.Now().UTC()

	for _, item := range normalized {
		if item.ProviderDeviceID == "" {
			result.Rejected++
			s.metrics.IncSlotRejected()
			continue
		}
		if _, err := repository.ParseSlotType(item.SlotType); err != nil {
			result.Rejected++
			s.metrics.IncSlotRejected()
			continue
		}
		if !domain.IsEnabledSlotSourceType(item.SourceType) {
			result.Rejected++
			s.metrics.IncSlotRejected()
			continue
		}
		if item.SourceObservedAt.After(now.Add(5 * time.Minute)) {
			result.Rejected++
			s.metrics.IncSlotRejected()
			continue
		}
		if !item.WindowStart.Before(item.WindowEnd) {
			result.Rejected++
			s.metrics.IncSlotRejected()
			continue
		}

		binding, err := s.trackingRepo.FindActiveBindingByDeviceAnyTenant(ctx, providerCode, item.ProviderDeviceID)
		if errors.Is(err, pgx.ErrNoRows) {
			result.Rejected++
			s.metrics.IncSlotRejected()
			continue
		}
		if err != nil {
			result.Rejected++
			s.metrics.IncSlotRejected()
			continue
		}

		quality, reasons := EvaluateSlotQuality(item.SourceType, item.SlotStatus)
		dedupKey := repository.BuildSlotDedupKey(providerCode, item.SlotType, binding.ShipmentID, item.WindowStart, item.WindowEnd, item.SourceObservedAt, item.ProviderSlotID, item.ProviderVersion)
		providerCodeCopy := providerCode
		var facilityID *uuid.UUID
		if item.FacilityID != nil && *item.FacilityID != "" {
			if id, parseErr := repository.ParseUUID(*item.FacilityID); parseErr == nil {
				facilityID = &id
			}
		}
		var locationID *uuid.UUID
		if item.LocationID != nil && *item.LocationID != "" {
			if id, parseErr := repository.ParseUUID(*item.LocationID); parseErr == nil {
				locationID = &id
			}
		}

		rev := domain.SlotRevision{
			ID:               uuid.New(),
			TenantID:         binding.TenantID,
			ShipmentID:       binding.ShipmentID,
			SlotType:         item.SlotType,
			FacilityID:       facilityID,
			LocationID:       locationID,
			WindowStart:      item.WindowStart.UTC(),
			WindowEnd:        item.WindowEnd.UTC(),
			Timezone:         item.Timezone,
			SlotStatus:       item.SlotStatus,
			SourceType:       item.SourceType,
			ProviderCode:     &providerCodeCopy,
			ProviderSlotID:   item.ProviderSlotID,
			ProviderVersion:  item.ProviderVersion,
			DedupKey:         dedupKey,
			SourceObservedAt: item.SourceObservedAt.UTC(),
			ReceivedAt:       now,
			QualityStatus:    quality,
			QualityReasons:   reasons,
			BookedAt:         item.BookedAt,
			ConfirmedAt:      item.ConfirmedAt,
			CancelledAt:      item.CancelledAt,
		}

		inserted, err := s.slotRepo.InsertSlotRevision(ctx, rev)
		if err != nil {
			result.Rejected++
			s.metrics.IncSlotRejected()
			continue
		}
		if !inserted {
			result.Deduplicated++
			s.metrics.IncSlotDeduplicated()
			continue
		}
		result.Accepted++
		s.metrics.IncSlotReceived()

		current, _ := s.slotRepo.GetSlotState(ctx, binding.TenantID, binding.ShipmentID, item.SlotType)
		candidate := s.evaluator.BuildStateFromRevision(
			binding.TenantID, binding.ShipmentID, item.SlotType,
			item.WindowStart, item.WindowEnd, item.SourceObservedAt, now,
			item.Timezone, item.SlotStatus, item.SourceType, providerCode,
			facilityID, locationID, item.ProviderSlotID,
			quality, item.BookedAt, item.ConfirmedAt, now,
		)
		replace := s.evaluator.ShouldReplaceCurrent(current, candidate)
		if err := s.slotRepo.UpsertSlotStateIfNewer(ctx, candidate, replace); err != nil {
			s.log.Warn("slot state upsert failed", slog.String("shipment_id", binding.ShipmentID.String()))
		} else if replace {
			fromStatus := (*string)(nil)
			if current != nil && current.SlotStatus != nil {
				fromStatus = current.SlotStatus
			}
			transition := domain.TransitionSlotBecameAvailable
			if current != nil && current.WindowStatus == domain.SlotWindowAvailable {
				transition = domain.TransitionSlotRescheduled
			}
			if item.SlotStatus == domain.SlotStatusCancelled {
				transition = domain.TransitionSlotCancelled
			}
			toStatus := item.SlotStatus
			s.evaluator.RecordTransitionIfNeeded(ctx, binding.TenantID, binding.ShipmentID, item.SlotType, transition, toStatus, fromStatus, map[string]any{
				"windowStart": item.WindowStart.UTC().Format(time.RFC3339),
				"windowEnd":   item.WindowEnd.UTC().Format(time.RFC3339),
			})
			s.metrics.IncSlotReschedule()
		}
	}

	return result, nil
}
