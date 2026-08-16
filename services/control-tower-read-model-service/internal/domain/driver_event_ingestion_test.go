package domain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestParseAndNormalizeDelayEvent(t *testing.T) {
	eventID := uuid.New()
	tenantID := uuid.New()
	shipmentID := uuid.New()
	driverID := uuid.New()
	payload, err := json.Marshal(map[string]any{
		"eventId": eventID.String(), "eventType": "driver.delay.reported", "schemaVersion": 1,
		"occurredAt": time.Now().UTC().Format(time.RFC3339Nano),
		"tenantId": tenantID.String(), "shipmentId": shipmentID.String(), "driverId": driverID.String(),
		"source": "driver", "sourceEventId": uuid.NewString(), "reasonCode": "TRAFFIC",
		"aggregate": map[string]any{"type": "SHIPMENT", "id": shipmentID.String(), "version": 1},
	})
	require.NoError(t, err)

	env, perm := ParseDriverEventEnvelope(payload)
	require.Nil(t, perm)
	event, err := NormalizeDriverEvent(env)
	require.NoError(t, err)
	require.Equal(t, "driver.delay.reported", event.Type)

	trigger, ok := MapDriverAutomationTrigger(event, env)
	require.True(t, ok)
	require.Equal(t, DriverTriggerDelayReported, trigger.TriggerType)
}

func TestParseDriverEventEnvelopeLegacyException(t *testing.T) {
	payload := []byte(`{"eventId":"` + uuid.NewString() + `","eventType":"driver.exception_reported","schemaVersion":1,"occurredAt":"2026-01-01T00:00:00Z","tenantId":"` + uuid.NewString() + `","shipmentId":"` + uuid.NewString() + `","sourceEventId":"` + uuid.NewString() + `"}`)
	env, perm := ParseDriverEventEnvelope(payload)
	require.Nil(t, perm)
	require.Equal(t, "driver.problem.reported", env.EventType)
}
