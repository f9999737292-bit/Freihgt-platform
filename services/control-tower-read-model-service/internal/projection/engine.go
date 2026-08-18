package projection

import (
	"time"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
)

type ApplyInput struct {
	Event    domain.ShipmentStatusEvent
	Existing *domain.ShipmentStatusProjection
	Now      time.Time
}

type ApplyResult struct {
	Outcome    string
	Updated    bool
	Projection domain.ShipmentStatusProjection
}

func ApplyEvent(input ApplyInput) ApplyResult {
	event := input.Event
	now := input.Now.UTC()
	if input.Existing == nil {
		return applyNewProjection(event, now)
	}
	existing := *input.Existing
	if event.Aggregate.Version <= existing.ShipmentVersion {
		return ApplyResult{Outcome: domain.OutcomeStale, Updated: false, Projection: existing}
	}
	if event.Aggregate.Version == existing.ShipmentVersion+1 {
		updated := buildUpdatedProjection(existing, event, now, existing.Complete, existing.GapDetected, existing.GapFromVersion, existing.GapToVersion)
		return ApplyResult{Outcome: domain.OutcomeApplied, Updated: true, Projection: updated}
	}
	from := existing.ShipmentVersion + 1
	to := event.Aggregate.Version - 1
	gapFrom, gapTo := from, to
	updated := buildUpdatedProjection(existing, event, now, false, true, &gapFrom, &gapTo)
	return ApplyResult{Outcome: domain.OutcomeGapApplied, Updated: true, Projection: updated}
}

func applyNewProjection(event domain.ShipmentStatusEvent, now time.Time) ApplyResult {
	if event.Aggregate.Version == 1 {
		p := newProjectionFromEvent(event, now, true, false, nil, nil)
		return ApplyResult{Outcome: domain.OutcomeApplied, Updated: true, Projection: p}
	}
	from, to := 1, event.Aggregate.Version-1
	gapFrom, gapTo := from, to
	p := newProjectionFromEvent(event, now, false, true, &gapFrom, &gapTo)
	return ApplyResult{Outcome: domain.OutcomeGapApplied, Updated: true, Projection: p}
}

func newProjectionFromEvent(event domain.ShipmentStatusEvent, now time.Time, complete, gap bool, gapFrom, gapTo *int) domain.ShipmentStatusProjection {
	prev := event.Data.FromStatus
	p := domain.ShipmentStatusProjection{
		TenantID:          event.TenantID,
		ShipmentID:        event.Aggregate.ID,
		ShipmentVersion:   event.Aggregate.Version,
		CurrentStatus:     event.Data.ToStatus,
		PreviousStatus:    prev,
		LastEventID:       event.EventID,
		LastSourceEventID: event.SourceEventID,
		LastEventType:     event.EventType,
		LastOccurredAt:    event.OccurredAt.UTC(),
		LastConsumedAt:    now,
		Complete:          complete,
		GapDetected:       gap,
		GapFromVersion:    gapFrom,
		GapToVersion:      gapTo,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	mergeActuals(&p, event.Data)
	return p
}

func buildUpdatedProjection(existing domain.ShipmentStatusProjection, event domain.ShipmentStatusEvent, now time.Time, complete, gap bool, gapFrom, gapTo *int) domain.ShipmentStatusProjection {
	prev := existing.CurrentStatus
	updated := existing
	updated.ShipmentVersion = event.Aggregate.Version
	updated.PreviousStatus = &prev
	updated.CurrentStatus = event.Data.ToStatus
	updated.LastEventID = event.EventID
	updated.LastSourceEventID = event.SourceEventID
	updated.LastEventType = event.EventType
	updated.LastOccurredAt = event.OccurredAt.UTC()
	updated.LastConsumedAt = now
	updated.Complete = complete
	updated.GapDetected = gap
	updated.GapFromVersion = gapFrom
	updated.GapToVersion = gapTo
	updated.UpdatedAt = now
	mergeActuals(&updated, event.Data)
	return updated
}

func mergeActuals(projection *domain.ShipmentStatusProjection, data domain.ShipmentStatusEventData) {
	if projection == nil {
		return
	}
	if data.PlannedPickupAt != nil {
		projection.PlannedPickupAt = data.PlannedPickupAt
	}
	if data.PlannedDeliveryAt != nil {
		projection.PlannedDeliveryAt = data.PlannedDeliveryAt
	}
	if data.ActualPickupAt != nil && projection.ActualPickupAt == nil {
		projection.ActualPickupAt = data.ActualPickupAt
	}
	if data.ActualDeliveryAt != nil && projection.ActualDeliveryAt == nil {
		projection.ActualDeliveryAt = data.ActualDeliveryAt
	}
}
