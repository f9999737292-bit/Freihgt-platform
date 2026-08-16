package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestBuildDriverEventOutbox(t *testing.T) {
	eventID := uuid.New()
	sourceID := uuid.New()
	outbox, err := BuildDriverEventOutbox(BuildDriverEventParams{
		EventID: eventID, EventType: DriverEventTypeDelayReported,
		TenantID: uuid.New(), ShipmentID: uuid.New(), ShipmentVersion: 2,
		DriverID: uuid.New(), SourceEventID: sourceID, ReasonCode: "TRAFFIC",
		OccurredAt: time.Now().UTC(),
	})
	require.NoError(t, err)
	require.Equal(t, DriverEventTypeDelayReported, outbox.EventType)
	require.NotEmpty(t, outbox.Payload)
}

func TestIsDriverKafkaEventType(t *testing.T) {
	require.True(t, IsDriverKafkaEventType(DriverEventTypeTrackingLost))
	require.False(t, IsDriverKafkaEventType(OutboxEventTypeCreated))
}

func TestMapOperationalEventType(t *testing.T) {
	mapped, ok := MapOperationalEventType("ARRIVED_AT_PICKUP")
	require.True(t, ok)
	require.Equal(t, DriverEventTypeArrivedAtPickup, mapped)
}
