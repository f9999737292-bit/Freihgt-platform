package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/tracking-service/internal/outbox"
	"github.com/freight-platform/tracking-service/internal/repository"
)

const (
	driverEventTypeTrackingLost     = "driver.tracking.lost"
	driverEventTypeTrackingRestored = "driver.tracking.restored"
)

type TrackingLossDetector struct {
	repo      *repository.TrackingRepository
	outbox    *outbox.Publisher
	threshold time.Duration
	interval  time.Duration
	batchSize int
	log       *slog.Logger
}

func NewTrackingLossDetector(
	repo *repository.TrackingRepository,
	outboxPublisher *outbox.Publisher,
	threshold, interval time.Duration,
	batchSize int,
	log *slog.Logger,
) *TrackingLossDetector {
	if batchSize <= 0 {
		batchSize = 100
	}
	return &TrackingLossDetector{
		repo: repo, outbox: outboxPublisher, threshold: threshold, interval: interval, batchSize: batchSize, log: log,
	}
}

func (d *TrackingLossDetector) Start(ctx context.Context) {
	if d.outbox == nil || d.threshold <= 0 {
		return
	}
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()
	d.runOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.runOnce(ctx)
		}
	}
}

func (d *TrackingLossDetector) runOnce(ctx context.Context) {
	candidates, err := d.repo.ListTrackingAutomationCandidates(ctx, d.batchSize)
	if err != nil {
		d.log.Warn("tracking loss detector query failed", slog.String("error", err.Error()))
		return
	}
	now := time.Now().UTC()
	for _, item := range candidates {
		lost := isTrackingLost(item.LastLocationRecorded, now, d.threshold, item.TrackingStatus)
		current := item.AutomationState
		if current == "" {
			current = repository.TrackingAutomationOK
		}
		switch {
		case lost && current != repository.TrackingAutomationLost:
			if err := d.emitTransition(ctx, item, driverEventTypeTrackingLost, repository.TrackingAutomationLost, now); err != nil {
				d.log.Warn("emit tracking lost failed", slog.String("shipment_id", item.ShipmentID.String()), slog.String("error", err.Error()))
			}
		case !lost && current == repository.TrackingAutomationLost:
			if err := d.emitTransition(ctx, item, driverEventTypeTrackingRestored, repository.TrackingAutomationOK, now); err != nil {
				d.log.Warn("emit tracking restored failed", slog.String("shipment_id", item.ShipmentID.String()), slog.String("error", err.Error()))
			}
		}
	}
}

func isTrackingLost(lastRecorded *time.Time, now time.Time, threshold time.Duration, trackingStatus string) bool {
	if trackingStatus == "lost" {
		return true
	}
	if lastRecorded == nil {
		return false
	}
	return now.Sub(lastRecorded.UTC()) > threshold
}

func (d *TrackingLossDetector) emitTransition(
	ctx context.Context,
	item repository.TrackingAutomationCandidate,
	eventType, nextState string,
	now time.Time,
) error {
	sourceEventID := uuid.New()
	eventID := uuid.New()
	driverID := uuid.Nil
	if item.DriverID != nil {
		driverID = *item.DriverID
	}
	payload, err := json.Marshal(map[string]any{
		"eventId":       eventID.String(),
		"eventType":     eventType,
		"schemaVersion": 1,
		"occurredAt":    now.Format(time.RFC3339Nano),
		"tenantId":      item.TenantID.String(),
		"shipmentId":    item.ShipmentID.String(),
		"driverId":      driverID.String(),
		"source":        "tracking-service",
		"sourceEventId": sourceEventID.String(),
		"aggregate": map[string]any{
			"type": "SHIPMENT", "id": item.ShipmentID.String(), "version": item.ShipmentVersion,
		},
	})
	if err != nil {
		return err
	}
	if err := d.outbox.InsertPending(ctx, outbox.DriverEventParams{
		EventID: eventID, EventType: eventType, TenantID: item.TenantID, ShipmentID: item.ShipmentID,
		ShipmentVersion: item.ShipmentVersion, DriverID: driverID, SourceEventID: sourceEventID,
		OccurredAt: now, Payload: payload,
	}); err != nil {
		return err
	}
	return d.repo.UpsertTrackingAutomationState(ctx, item.TenantID, item.ShipmentID, nextState, item.LastLocationRecorded)
}
