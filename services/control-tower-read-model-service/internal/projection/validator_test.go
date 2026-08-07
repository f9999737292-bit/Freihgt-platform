package projection

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
)

const configuredTopic = "shipment.status.v1"

func validEventIDs() (eventID, tenantID, shipmentID, sourceEventID uuid.UUID) {
	eventID = uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	tenantID = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	shipmentID = uuid.MustParse("22222222-2222-2222-2222-222222222222")
	sourceEventID = uuid.MustParse("33333333-3333-3333-3333-333333333333")
	return
}

func buildValidEnvelope(t *testing.T, mutate func(map[string]any)) []byte {
	t.Helper()
	eventID, tenantID, shipmentID, sourceEventID := validEventIDs()
	env := map[string]any{
		"eventId":       eventID.String(),
		"eventType":     domain.EventTypeCreated,
		"schemaVersion": domain.SchemaVersionV1,
		"occurredAt":    "2026-08-01T12:00:00.000000000Z",
		"tenantId":      tenantID.String(),
		"aggregate": map[string]any{
			"type":    domain.AggregateTypeShipment,
			"id":      shipmentID.String(),
			"version": 1,
		},
		"sourceEventId": sourceEventID.String(),
		"data": map[string]any{
			"toStatus":  domain.StatusCarrierAssigned,
			"actorType": "SYSTEM",
		},
	}
	if mutate != nil {
		mutate(env)
	}
	raw, err := json.Marshal(env)
	require.NoError(t, err)
	return raw
}

func validMeta(shipmentID uuid.UUID) domain.KafkaRecordMeta {
	return domain.KafkaRecordMeta{
		Topic:     configuredTopic,
		Partition: 0,
		Offset:    42,
		Key:       shipmentID.String(),
	}
}

func TestParseAndValidateValidShipmentCreated(t *testing.T) {
	t.Parallel()
	_, tenantID, shipmentID, sourceEventID := validEventIDs()
	payload := buildValidEnvelope(t, nil)
	meta := validMeta(shipmentID)

	event, permErr := ParseAndValidate(payload, meta, configuredTopic)
	require.Nil(t, permErr)
	assert.Equal(t, domain.EventTypeCreated, event.EventType)
	assert.Equal(t, tenantID, event.TenantID)
	assert.Equal(t, shipmentID, event.Aggregate.ID)
	assert.Equal(t, sourceEventID, event.SourceEventID)
	assert.Nil(t, event.Data.FromStatus)
	assert.Equal(t, domain.StatusCarrierAssigned, event.Data.ToStatus)
}

func TestParseAndValidateValidGenericStatusChange(t *testing.T) {
	t.Parallel()
	from := domain.StatusCarrierAssigned
	payload := buildValidEnvelope(t, func(env map[string]any) {
		env["eventType"] = domain.EventTypeStatusChanged
		env["aggregate"].(map[string]any)["version"] = 2
		data := env["data"].(map[string]any)
		data["fromStatus"] = from
		data["toStatus"] = domain.StatusInTransit
	})
	_, _, shipmentID, _ := validEventIDs()
	meta := validMeta(shipmentID)

	event, permErr := ParseAndValidate(payload, meta, configuredTopic)
	require.Nil(t, permErr)
	assert.Equal(t, domain.EventTypeStatusChanged, event.EventType)
	require.NotNil(t, event.Data.FromStatus)
	assert.Equal(t, from, *event.Data.FromStatus)
	assert.Equal(t, domain.StatusInTransit, event.Data.ToStatus)
}

func TestParseAndValidateValidCancelled(t *testing.T) {
	t.Parallel()
	payload := buildValidEnvelope(t, func(env map[string]any) {
		env["eventType"] = domain.EventTypeCancelled
		env["aggregate"].(map[string]any)["version"] = 3
		env["data"].(map[string]any)["toStatus"] = domain.StatusCancelled
	})
	_, _, shipmentID, _ := validEventIDs()

	event, permErr := ParseAndValidate(payload, validMeta(shipmentID), configuredTopic)
	require.Nil(t, permErr)
	assert.Equal(t, domain.EventTypeCancelled, event.EventType)
	assert.Equal(t, domain.StatusCancelled, event.Data.ToStatus)
}

func TestParseAndValidateValidBillingMilestone(t *testing.T) {
	t.Parallel()
	cases := []struct {
		eventType string
		toStatus  string
	}{
		{domain.EventTypeReadyForBilling, domain.StatusReadyForBilling},
		{domain.EventTypeDocumentsCompleted, domain.StatusDocumentsCompleted},
		{domain.EventTypeFinanciallyClosed, domain.StatusFinanciallyClosed},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.eventType, func(t *testing.T) {
			t.Parallel()
			payload := buildValidEnvelope(t, func(env map[string]any) {
				env["eventType"] = tc.eventType
				env["data"].(map[string]any)["toStatus"] = tc.toStatus
			})
			_, _, shipmentID, _ := validEventIDs()
			_, permErr := ParseAndValidate(payload, validMeta(shipmentID), configuredTopic)
			require.Nil(t, permErr)
		})
	}
}

func TestParseAndValidateInvalidJSON(t *testing.T) {
	t.Parallel()
	_, permErr := ParseAndValidate([]byte("{"), domain.KafkaRecordMeta{}, configuredTopic)
	require.NotNil(t, permErr)
	assert.Equal(t, domain.ErrorInvalidJSON, permErr.Code)
}

func TestParseAndValidateMissingEventID(t *testing.T) {
	t.Parallel()
	payload := buildValidEnvelope(t, func(env map[string]any) { delete(env, "eventId") })
	_, _, shipmentID, _ := validEventIDs()
	_, permErr := ParseAndValidate(payload, validMeta(shipmentID), configuredTopic)
	require.NotNil(t, permErr)
	assert.Equal(t, domain.ErrorInvalidEventID, permErr.Code)
}

func TestParseAndValidateZeroEventID(t *testing.T) {
	t.Parallel()
	payload := buildValidEnvelope(t, func(env map[string]any) {
		env["eventId"] = "00000000-0000-0000-0000-000000000000"
	})
	_, _, shipmentID, _ := validEventIDs()
	_, permErr := ParseAndValidate(payload, validMeta(shipmentID), configuredTopic)
	require.NotNil(t, permErr)
	assert.Equal(t, domain.ErrorInvalidEventID, permErr.Code)
}

func TestParseAndValidateMissingSourceEventID(t *testing.T) {
	t.Parallel()
	payload := buildValidEnvelope(t, func(env map[string]any) { delete(env, "sourceEventId") })
	_, _, shipmentID, _ := validEventIDs()
	_, permErr := ParseAndValidate(payload, validMeta(shipmentID), configuredTopic)
	require.NotNil(t, permErr)
	assert.Equal(t, domain.ErrorInvalidSourceEventID, permErr.Code)
}

func TestParseAndValidateMissingTenant(t *testing.T) {
	t.Parallel()
	payload := buildValidEnvelope(t, func(env map[string]any) { delete(env, "tenantId") })
	_, _, shipmentID, _ := validEventIDs()
	_, permErr := ParseAndValidate(payload, validMeta(shipmentID), configuredTopic)
	require.NotNil(t, permErr)
	assert.Equal(t, domain.ErrorInvalidTenantID, permErr.Code)
}

func TestParseAndValidateInvalidAggregateType(t *testing.T) {
	t.Parallel()
	payload := buildValidEnvelope(t, func(env map[string]any) {
		env["aggregate"].(map[string]any)["type"] = "ORDER"
	})
	_, _, shipmentID, _ := validEventIDs()
	_, permErr := ParseAndValidate(payload, validMeta(shipmentID), configuredTopic)
	require.NotNil(t, permErr)
	assert.Equal(t, domain.ErrorInvalidAggregate, permErr.Code)
}

func TestParseAndValidateInvalidAggregateID(t *testing.T) {
	t.Parallel()
	payload := buildValidEnvelope(t, func(env map[string]any) {
		env["aggregate"].(map[string]any)["id"] = "not-a-uuid"
	})
	meta := domain.KafkaRecordMeta{Topic: configuredTopic, Key: "not-a-uuid"}
	_, permErr := ParseAndValidate(payload, meta, configuredTopic)
	require.NotNil(t, permErr)
	assert.Equal(t, domain.ErrorInvalidAggregate, permErr.Code)
}

func TestParseAndValidateVersionZero(t *testing.T) {
	t.Parallel()
	payload := buildValidEnvelope(t, func(env map[string]any) {
		env["aggregate"].(map[string]any)["version"] = 0
	})
	_, _, shipmentID, _ := validEventIDs()
	_, permErr := ParseAndValidate(payload, validMeta(shipmentID), configuredTopic)
	require.NotNil(t, permErr)
	assert.Equal(t, domain.ErrorInvalidAggregate, permErr.Code)
}

func TestParseAndValidateUnsupportedSchemaVersion(t *testing.T) {
	t.Parallel()
	payload := buildValidEnvelope(t, func(env map[string]any) { env["schemaVersion"] = 2 })
	_, _, shipmentID, _ := validEventIDs()
	_, permErr := ParseAndValidate(payload, validMeta(shipmentID), configuredTopic)
	require.NotNil(t, permErr)
	assert.Equal(t, domain.ErrorUnsupportedSchemaVersion, permErr.Code)
}

func TestParseAndValidateUnknownEventType(t *testing.T) {
	t.Parallel()
	payload := buildValidEnvelope(t, func(env map[string]any) { env["eventType"] = "shipment.unknown" })
	_, _, shipmentID, _ := validEventIDs()
	_, permErr := ParseAndValidate(payload, validMeta(shipmentID), configuredTopic)
	require.NotNil(t, permErr)
	assert.Equal(t, domain.ErrorInvalidEventType, permErr.Code)
}

func TestParseAndValidateInvalidStatus(t *testing.T) {
	t.Parallel()
	payload := buildValidEnvelope(t, func(env map[string]any) {
		env["data"].(map[string]any)["toStatus"] = "NOT_A_REAL_STATUS"
	})
	_, _, shipmentID, _ := validEventIDs()
	_, permErr := ParseAndValidate(payload, validMeta(shipmentID), configuredTopic)
	require.NotNil(t, permErr)
	assert.Equal(t, domain.ErrorInvalidEventData, permErr.Code)
}

func TestParseAndValidateEventTypeStatusMismatch(t *testing.T) {
	t.Parallel()
	payload := buildValidEnvelope(t, func(env map[string]any) {
		env["eventType"] = domain.EventTypeCancelled
		env["data"].(map[string]any)["toStatus"] = domain.StatusInTransit
	})
	_, _, shipmentID, _ := validEventIDs()
	_, permErr := ParseAndValidate(payload, validMeta(shipmentID), configuredTopic)
	require.NotNil(t, permErr)
	assert.Equal(t, domain.ErrorEventSchemaViolation, permErr.Code)
}

func TestParseAndValidateCreatedWithFromStatusRejected(t *testing.T) {
	t.Parallel()
	from := domain.StatusCarrierAssigned
	payload := buildValidEnvelope(t, func(env map[string]any) {
		env["data"].(map[string]any)["fromStatus"] = from
	})
	_, _, shipmentID, _ := validEventIDs()
	_, permErr := ParseAndValidate(payload, validMeta(shipmentID), configuredTopic)
	require.NotNil(t, permErr)
	assert.Equal(t, domain.ErrorEventSchemaViolation, permErr.Code)
}

func TestParseAndValidateKafkaKeyMismatch(t *testing.T) {
	t.Parallel()
	payload := buildValidEnvelope(t, nil)
	meta := domain.KafkaRecordMeta{
		Topic: configuredTopic,
		Key:   uuid.NewString(),
	}
	_, permErr := ParseAndValidate(payload, meta, configuredTopic)
	require.NotNil(t, permErr)
	assert.Equal(t, domain.ErrorEventSchemaViolation, permErr.Code)
}

func TestParseAndValidateKafkaTopicMismatch(t *testing.T) {
	t.Parallel()
	_, _, shipmentID, _ := validEventIDs()
	payload := buildValidEnvelope(t, nil)
	meta := domain.KafkaRecordMeta{Topic: "other.topic", Key: shipmentID.String()}
	_, permErr := ParseAndValidate(payload, meta, configuredTopic)
	require.NotNil(t, permErr)
	assert.Equal(t, domain.ErrorEventSchemaViolation, permErr.Code)
}

func TestParseAndValidateEnvelopeIsSourceOfTruthNotHeaders(t *testing.T) {
	t.Parallel()
	eventID, tenantID, shipmentID, sourceEventID := validEventIDs()
	payload := buildValidEnvelope(t, nil)
	meta := validMeta(shipmentID)

	event, permErr := ParseAndValidate(payload, meta, configuredTopic)
	require.Nil(t, permErr)
	assert.Equal(t, eventID, event.EventID)
	assert.Equal(t, tenantID, event.TenantID)
	assert.Equal(t, shipmentID, event.Aggregate.ID)
	assert.Equal(t, sourceEventID, event.SourceEventID)
	assert.True(t, strings.HasPrefix(event.OccurredAt.Format(time.RFC3339Nano), "2026-08-01T12:00:00"))
}

func TestParseAndValidatePayloadHashStable(t *testing.T) {
	t.Parallel()
	payload := buildValidEnvelope(t, nil)
	assert.Equal(t, PayloadSHA256(payload), PayloadSHA256(payload))
}
