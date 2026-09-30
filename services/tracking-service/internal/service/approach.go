package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/tracking-service/internal/domain"
	"github.com/freight-platform/tracking-service/internal/repository"
)

type StopApproachService struct {
	tracking  *repository.TrackingRepository
	execution *repository.ExecutionTrackingRepository
	enabled   bool
	radiusM   float64
}

func NewStopApproachService(tracking *repository.TrackingRepository, execution *repository.ExecutionTrackingRepository, enabled bool, radiusMeters float64) *StopApproachService {
	return &StopApproachService{tracking: tracking, execution: execution, enabled: enabled, radiusM: radiusMeters}
}

func (s *StopApproachService) Enabled() bool {
	return s != nil && s.enabled && s.radiusM > 0
}

func (s *StopApproachService) AcceptLocation(ctx context.Context, event domain.LocationEvent, freshness, quality string) (bool, error) {
	tx, err := s.execution.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	inserted, err := s.tracking.InsertLocationEventTx(ctx, tx, event)
	if err != nil {
		return false, err
	}
	if !inserted {
		if err := tx.Commit(ctx); err != nil {
			return false, err
		}
		return false, nil
	}
	if err := s.emitApproach(ctx, tx, event, freshness, quality); err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

func (s *StopApproachService) emitApproach(ctx context.Context, tx pgx.Tx, event domain.LocationEvent, freshness, quality string) error {
	if !approachUsable(freshness, quality) || event.DriverID == nil && event.VehicleID == nil {
		return nil
	}
	var driverIDs, vehicleIDs []uuid.UUID
	if event.DriverID != nil {
		driverIDs = []uuid.UUID{*event.DriverID}
	}
	if event.VehicleID != nil {
		vehicleIDs = []uuid.UUID{*event.VehicleID}
	}
	targets, err := s.execution.ListActiveTargetsForActors(ctx, driverIDs, vehicleIDs)
	if err != nil || len(targets) == 0 {
		return err
	}
	matched := make([]repository.ExecutionTrackingState, 0, 1)
	for _, target := range targets {
		if !actorMatchesTarget(target, driverIDs, vehicleIDs) {
			continue
		}
		if target.LiveETAStopID == nil || target.TargetLatitude == nil || target.TargetLongitude == nil {
			continue
		}
		matched = append(matched, target)
	}
	if len(matched) != 1 {
		return nil
	}
	target := matched[0]
	if target.TargetLatitude == nil || target.TargetLongitude == nil || target.LiveETAStopID == nil {
		return nil
	}
	distanceM := domain.HaversineDistanceKm(event.Latitude, event.Longitude, *target.TargetLatitude, *target.TargetLongitude) * 1000
	if distanceM > s.radiusM {
		return nil
	}
	marked, err := repository.MarkApproachEmitted(ctx, tx, target.ExecutionID, *target.LiveETAStopID, event.RecordedAt)
	if err != nil || !marked {
		return err
	}
	ordinal := 0
	if target.LiveETAOrdinal != nil {
		ordinal = *target.LiveETAOrdinal
	}
	payload, err := json.Marshal(map[string]any{
		"eventId":           uuid.NewString(),
		"eventType":         domain.EventStopApproaching,
		"operatingTenantId": target.OperatingTenantID.String(),
		"executionId":       target.ExecutionID.String(),
		"revisionId":        target.RevisionID.String(),
		"executionStopId":   target.LiveETAStopID.String(),
		"ordinal":           ordinal,
		"occurredAt":        event.RecordedAt.UTC().Format(time.RFC3339Nano),
		"distanceMeters":    distanceM,
		"trackingFreshness": freshness,
		"trackingQuality":   quality,
	})
	if err != nil {
		return err
	}
	headers, _ := json.Marshal(map[string]string{"contentType": "application/json", "eventType": domain.EventStopApproaching})
	version := int(target.LastExecutionEventSequence)
	if version < 1 {
		version = 1
	}
	sourceID := uuid.New()
	return repository.InsertTrackingOutbox(ctx, tx, repository.TrackingOutboxEvent{
		ID:               sourceID,
		TenantID:         target.OperatingTenantID,
		AggregateType:    "EXECUTION_TRACKING",
		AggregateID:      target.ExecutionID,
		AggregateVersion: version,
		EventType:        domain.EventStopApproaching,
		SchemaVersion:    1,
		SourceEventID:    sourceID,
		Payload:          payload,
		Headers:          headers,
		AvailableAt:      time.Now().UTC(),
	})
}

func approachUsable(freshness, quality string) bool {
	if freshness != domain.FreshnessFresh {
		return false
	}
	return quality == domain.QualityGood || quality == domain.QualityDegraded
}
