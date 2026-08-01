package projection

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
)

func testEvent(version int, toStatus string) domain.ShipmentStatusEvent {
	eventID := uuid.New()
	sourceEventID := uuid.New()
	tenantID := uuid.New()
	shipmentID := uuid.New()
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	return domain.ShipmentStatusEvent{
		EventID:       eventID,
		EventType:     domain.EventTypeStatusChanged,
		SchemaVersion: domain.SchemaVersionV1,
		OccurredAt:    now,
		TenantID:      tenantID,
		Aggregate: domain.ShipmentAggregate{
			Type:    domain.AggregateTypeShipment,
			ID:      shipmentID,
			Version: version,
		},
		SourceEventID: sourceEventID,
		Data: domain.ShipmentStatusEventData{
			ToStatus:  toStatus,
			ActorType: "SYSTEM",
		},
	}
}

func existingProjection(version int, status string, complete, gap bool, gapFrom, gapTo *int) *domain.ShipmentStatusProjection {
	now := time.Date(2026, 8, 1, 11, 0, 0, 0, time.UTC)
	return &domain.ShipmentStatusProjection{
		TenantID:          uuid.New(),
		ShipmentID:        uuid.New(),
		ShipmentVersion:   version,
		CurrentStatus:     status,
		LastEventID:       uuid.New(),
		LastSourceEventID: uuid.New(),
		LastEventType:     domain.EventTypeCreated,
		LastOccurredAt:    now,
		LastConsumedAt:    now,
		Complete:          complete,
		GapDetected:       gap,
		GapFromVersion:    gapFrom,
		GapToVersion:      gapTo,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

func TestApplyEventNewProjectionVersionOneComplete(t *testing.T) {
	t.Parallel()
	event := testEvent(1, domain.StatusCarrierAssigned)
	now := time.Date(2026, 8, 1, 12, 1, 0, 0, time.UTC)

	result := ApplyEvent(ApplyInput{Event: event, Existing: nil, Now: now})
	assert.Equal(t, domain.OutcomeApplied, result.Outcome)
	assert.True(t, result.Updated)
	assert.True(t, result.Projection.Complete)
	assert.False(t, result.Projection.GapDetected)
	assert.Nil(t, result.Projection.GapFromVersion)
	assert.Nil(t, result.Projection.GapToVersion)
	assert.Equal(t, 1, result.Projection.ShipmentVersion)
}

func TestApplyEventNewProjectionFirstVersionGreaterThanOneMarksGap(t *testing.T) {
	t.Parallel()
	event := testEvent(3, domain.StatusInTransit)
	now := time.Date(2026, 8, 1, 12, 1, 0, 0, time.UTC)

	result := ApplyEvent(ApplyInput{Event: event, Existing: nil, Now: now})
	assert.Equal(t, domain.OutcomeGapApplied, result.Outcome)
	assert.True(t, result.Updated)
	assert.False(t, result.Projection.Complete)
	assert.True(t, result.Projection.GapDetected)
	require.NotNil(t, result.Projection.GapFromVersion)
	require.NotNil(t, result.Projection.GapToVersion)
	assert.Equal(t, 1, *result.Projection.GapFromVersion)
	assert.Equal(t, 2, *result.Projection.GapToVersion)
	assert.Equal(t, 3, result.Projection.ShipmentVersion)
}

func TestApplyEventNextVersionUpdatesProjection(t *testing.T) {
	t.Parallel()
	existing := existingProjection(2, domain.StatusCarrierAssigned, true, false, nil, nil)
	event := testEvent(3, domain.StatusInTransit)
	event.TenantID = existing.TenantID
	event.Aggregate.ID = existing.ShipmentID
	now := time.Date(2026, 8, 1, 12, 2, 0, 0, time.UTC)

	result := ApplyEvent(ApplyInput{Event: event, Existing: existing, Now: now})
	assert.Equal(t, domain.OutcomeApplied, result.Outcome)
	assert.True(t, result.Updated)
	assert.Equal(t, 3, result.Projection.ShipmentVersion)
	assert.Equal(t, domain.StatusInTransit, result.Projection.CurrentStatus)
	require.NotNil(t, result.Projection.PreviousStatus)
	assert.Equal(t, domain.StatusCarrierAssigned, *result.Projection.PreviousStatus)
}

func TestApplyEventStaleVersionDoesNotUpdate(t *testing.T) {
	t.Parallel()
	existing := existingProjection(3, domain.StatusInTransit, true, false, nil, nil)
	event := testEvent(2, domain.StatusCarrierAssigned)
	event.TenantID = existing.TenantID
	event.Aggregate.ID = existing.ShipmentID

	result := ApplyEvent(ApplyInput{Event: event, Existing: existing, Now: time.Now().UTC()})
	assert.Equal(t, domain.OutcomeStale, result.Outcome)
	assert.False(t, result.Updated)
	assert.Equal(t, 3, result.Projection.ShipmentVersion)
	assert.Equal(t, domain.StatusInTransit, result.Projection.CurrentStatus)
}

func TestApplyEventEqualVersionIsStale(t *testing.T) {
	t.Parallel()
	existing := existingProjection(3, domain.StatusInTransit, true, false, nil, nil)
	event := testEvent(3, domain.StatusDelivered)
	event.TenantID = existing.TenantID
	event.Aggregate.ID = existing.ShipmentID

	result := ApplyEvent(ApplyInput{Event: event, Existing: existing, Now: time.Now().UTC()})
	assert.Equal(t, domain.OutcomeStale, result.Outcome)
	assert.False(t, result.Updated)
	assert.Equal(t, domain.StatusInTransit, result.Projection.CurrentStatus)
}

func TestApplyEventVersionGapAppliedWithMarkers(t *testing.T) {
	t.Parallel()
	existing := existingProjection(1, domain.StatusCarrierAssigned, true, false, nil, nil)
	event := testEvent(3, domain.StatusInTransit)
	event.TenantID = existing.TenantID
	event.Aggregate.ID = existing.ShipmentID

	result := ApplyEvent(ApplyInput{Event: event, Existing: existing, Now: time.Now().UTC()})
	assert.Equal(t, domain.OutcomeGapApplied, result.Outcome)
	assert.True(t, result.Updated)
	assert.False(t, result.Projection.Complete)
	assert.True(t, result.Projection.GapDetected)
	require.NotNil(t, result.Projection.GapFromVersion)
	require.NotNil(t, result.Projection.GapToVersion)
	assert.Equal(t, 2, *result.Projection.GapFromVersion)
	assert.Equal(t, 2, *result.Projection.GapToVersion)
	assert.Equal(t, 3, result.Projection.ShipmentVersion)
}

func TestApplyEventExistingGapNotSilentlyClearedOnNextVersion(t *testing.T) {
	t.Parallel()
	gapFrom, gapTo := 2, 2
	existing := existingProjection(3, domain.StatusInTransit, false, true, &gapFrom, &gapTo)
	event := testEvent(4, domain.StatusDelivered)
	event.TenantID = existing.TenantID
	event.Aggregate.ID = existing.ShipmentID

	result := ApplyEvent(ApplyInput{Event: event, Existing: existing, Now: time.Now().UTC()})
	assert.Equal(t, domain.OutcomeApplied, result.Outcome)
	assert.True(t, result.Updated)
	assert.False(t, result.Projection.Complete)
	assert.True(t, result.Projection.GapDetected)
	require.NotNil(t, result.Projection.GapFromVersion)
	require.NotNil(t, result.Projection.GapToVersion)
	assert.Equal(t, 2, *result.Projection.GapFromVersion)
	assert.Equal(t, 2, *result.Projection.GapToVersion)
	assert.Equal(t, 4, result.Projection.ShipmentVersion)
}
