//go:build integration

package postgres

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	"github.com/freight-platform/control-tower-read-model-service/internal/repository"
)

func TestMigration000015CreatesControlTowerObjects(t *testing.T) {
	env := SetupTestEnv(t)
	ctx := context.Background()

	for _, obj := range []struct{ schema, table string }{
		{"control_tower", "shipment_status_event_inbox"},
		{"control_tower", "shipment_status_projection"},
		{"control_tower", "shipment_status_event_dead_letter"},
	} {
		var exists bool
		err := env.Pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM information_schema.tables
				WHERE table_schema = $1 AND table_name = $2
			)`, obj.schema, obj.table).Scan(&exists)
		require.NoError(t, err)
		assert.True(t, exists, "table %s.%s", obj.schema, obj.table)
	}
}

func TestProcessEventVersionOneCreatesCompleteProjection(t *testing.T) {
	env := SetupTestEnv(t)
	ctx := context.Background()
	tenantID := uuid.New()
	shipmentID := uuid.New()
	eventID := uuid.New()
	sourceEventID := uuid.New()

	input := sampleProcessInput(tenantID, shipmentID, 1, eventID, sourceEventID, "shipment.status.v1.test", 1)
	input.Event.EventType = domain.EventTypeCreated
	input.Event.Data.ToStatus = domain.StatusCarrierAssigned

	result, err := env.Repo.ProcessEvent(ctx, input)
	require.NoError(t, err)
	assert.Equal(t, domain.OutcomeApplied, result.Outcome)
	assert.True(t, result.Applied)

	projection, err := env.Repo.GetProjection(ctx, tenantID, shipmentID)
	require.NoError(t, err)
	require.NotNil(t, projection)
	assert.True(t, projection.Complete)
	assert.False(t, projection.GapDetected)
	assert.Equal(t, 1, projection.ShipmentVersion)
}

func TestProcessEventFirstVersionGreaterThanOneCreatesIncompleteProjection(t *testing.T) {
	env := SetupTestEnv(t)
	ctx := context.Background()
	tenantID := uuid.New()
	shipmentID := uuid.New()

	input := sampleProcessInput(tenantID, shipmentID, 3, uuid.New(), uuid.New(), "shipment.status.v1.test", 2)
	result, err := env.Repo.ProcessEvent(ctx, input)
	require.NoError(t, err)
	assert.Equal(t, domain.OutcomeGapApplied, result.Outcome)

	projection, err := env.Repo.GetProjection(ctx, tenantID, shipmentID)
	require.NoError(t, err)
	require.NotNil(t, projection)
	assert.False(t, projection.Complete)
	assert.True(t, projection.GapDetected)
	require.NotNil(t, projection.GapFromVersion)
	require.NotNil(t, projection.GapToVersion)
	assert.Equal(t, 1, *projection.GapFromVersion)
	assert.Equal(t, 2, *projection.GapToVersion)
}

func TestProcessEventDuplicateEventIDDoesNotUpdateProjection(t *testing.T) {
	env := SetupTestEnv(t)
	ctx := context.Background()
	tenantID := uuid.New()
	shipmentID := uuid.New()
	eventID := uuid.New()
	sourceEventID := uuid.New()

	first := sampleProcessInput(tenantID, shipmentID, 1, eventID, sourceEventID, "shipment.status.v1.test", 10)
	first.Event.EventType = domain.EventTypeCreated
	first.Event.Data.ToStatus = domain.StatusCarrierAssigned
	_, err := env.Repo.ProcessEvent(ctx, first)
	require.NoError(t, err)

	second := sampleProcessInput(tenantID, shipmentID, 2, eventID, uuid.New(), "shipment.status.v1.test", 11)
	result, err := env.Repo.ProcessEvent(ctx, second)
	require.NoError(t, err)
	assert.True(t, result.Duplicate)

	projection, err := env.Repo.GetProjection(ctx, tenantID, shipmentID)
	require.NoError(t, err)
	require.NotNil(t, projection)
	assert.Equal(t, 1, projection.ShipmentVersion)
}

func TestProcessEventDuplicateSourceEventIDDoesNotUpdateProjection(t *testing.T) {
	env := SetupTestEnv(t)
	ctx := context.Background()
	tenantID := uuid.New()
	shipmentID := uuid.New()
	sourceEventID := uuid.New()

	first := sampleProcessInput(tenantID, shipmentID, 1, uuid.New(), sourceEventID, "shipment.status.v1.test", 20)
	first.Event.EventType = domain.EventTypeCreated
	first.Event.Data.ToStatus = domain.StatusCarrierAssigned
	_, err := env.Repo.ProcessEvent(ctx, first)
	require.NoError(t, err)

	second := sampleProcessInput(tenantID, shipmentID, 2, uuid.New(), sourceEventID, "shipment.status.v1.test", 21)
	result, err := env.Repo.ProcessEvent(ctx, second)
	require.NoError(t, err)
	assert.True(t, result.Duplicate)

	projection, err := env.Repo.GetProjection(ctx, tenantID, shipmentID)
	require.NoError(t, err)
	require.NotNil(t, projection)
	assert.Equal(t, 1, projection.ShipmentVersion)
}

func TestProcessEventDuplicateKafkaPositionDoesNotUpdateProjection(t *testing.T) {
	env := SetupTestEnv(t)
	ctx := context.Background()
	tenantID := uuid.New()
	shipmentID := uuid.New()

	first := sampleProcessInput(tenantID, shipmentID, 1, uuid.New(), uuid.New(), "shipment.status.v1.test", 30)
	first.Event.EventType = domain.EventTypeCreated
	first.Event.Data.ToStatus = domain.StatusCarrierAssigned
	_, err := env.Repo.ProcessEvent(ctx, first)
	require.NoError(t, err)

	second := sampleProcessInput(tenantID, shipmentID, 2, uuid.New(), uuid.New(), "shipment.status.v1.test", 30)
	result, err := env.Repo.ProcessEvent(ctx, second)
	require.NoError(t, err)
	assert.True(t, result.Duplicate)
}

func TestProcessEventStaleVersionDoesNotUpdateProjection(t *testing.T) {
	env := SetupTestEnv(t)
	ctx := context.Background()
	tenantID := uuid.New()
	shipmentID := uuid.New()

	v3 := sampleProcessInput(tenantID, shipmentID, 3, uuid.New(), uuid.New(), "shipment.status.v1.test", 40)
	_, err := env.Repo.ProcessEvent(ctx, v3)
	require.NoError(t, err)

	v2 := sampleProcessInput(tenantID, shipmentID, 2, uuid.New(), uuid.New(), "shipment.status.v1.test", 41)
	result, err := env.Repo.ProcessEvent(ctx, v2)
	require.NoError(t, err)
	assert.Equal(t, domain.OutcomeStale, result.Outcome)
	assert.False(t, result.Applied)

	projection, err := env.Repo.GetProjection(ctx, tenantID, shipmentID)
	require.NoError(t, err)
	require.NotNil(t, projection)
	assert.Equal(t, 3, projection.ShipmentVersion)
}

func TestProcessEventVersionGapAppliedWithMarkers(t *testing.T) {
	env := SetupTestEnv(t)
	ctx := context.Background()
	tenantID := uuid.New()
	shipmentID := uuid.New()

	v1 := sampleProcessInput(tenantID, shipmentID, 1, uuid.New(), uuid.New(), "shipment.status.v1.test", 50)
	v1.Event.EventType = domain.EventTypeCreated
	v1.Event.Data.ToStatus = domain.StatusCarrierAssigned
	_, err := env.Repo.ProcessEvent(ctx, v1)
	require.NoError(t, err)

	v3 := sampleProcessInput(tenantID, shipmentID, 3, uuid.New(), uuid.New(), "shipment.status.v1.test", 51)
	result, err := env.Repo.ProcessEvent(ctx, v3)
	require.NoError(t, err)
	assert.Equal(t, domain.OutcomeGapApplied, result.Outcome)

	projection, err := env.Repo.GetProjection(ctx, tenantID, shipmentID)
	require.NoError(t, err)
	require.NotNil(t, projection)
	assert.False(t, projection.Complete)
	assert.True(t, projection.GapDetected)
	require.NotNil(t, projection.GapFromVersion)
	require.NotNil(t, projection.GapToVersion)
	assert.Equal(t, 2, *projection.GapFromVersion)
	assert.Equal(t, 2, *projection.GapToVersion)
}

func TestProcessEventTenantIsolation(t *testing.T) {
	env := SetupTestEnv(t)
	ctx := context.Background()
	tenantA := uuid.New()
	tenantB := uuid.New()
	shipmentID := uuid.New()

	inputA := sampleProcessInput(tenantA, shipmentID, 1, uuid.New(), uuid.New(), "shipment.status.v1.test", 60)
	inputA.Event.EventType = domain.EventTypeCreated
	inputA.Event.Data.ToStatus = domain.StatusCarrierAssigned
	_, err := env.Repo.ProcessEvent(ctx, inputA)
	require.NoError(t, err)

	foreign, err := env.Repo.GetProjection(ctx, tenantB, shipmentID)
	require.NoError(t, err)
	assert.Nil(t, foreign)
}

func TestInsertDeadLetterIsIdempotentByKafkaPosition(t *testing.T) {
	env := SetupTestEnv(t)
	ctx := context.Background()
	meta := domain.KafkaRecordMeta{Topic: "shipment.status.v1.test", Partition: 0, Offset: 70}
	input := repository.DeadLetterInput{
		Meta:          meta,
		PayloadSHA256: "abc123",
		ErrorCode:     domain.ErrorInvalidJSON,
		ReceivedAt:    time.Now().UTC(),
	}

	first, err := env.Repo.InsertDeadLetter(ctx, input)
	require.NoError(t, err)
	assert.True(t, first)

	second, err := env.Repo.InsertDeadLetter(ctx, input)
	require.NoError(t, err)
	assert.False(t, second)

	count := countRows(ctx, env.Pool, `
		SELECT COUNT(*) FROM control_tower.shipment_status_event_dead_letter
		WHERE topic = $1 AND partition_id = $2 AND message_offset = $3`,
		meta.Topic, meta.Partition, meta.Offset)
	assert.Equal(t, int64(1), count)
}

func TestGetStatusSummaryIsTenantScoped(t *testing.T) {
	env := SetupTestEnv(t)
	ctx := context.Background()
	tenantA := uuid.New()
	tenantB := uuid.New()

	for i, tenantID := range []uuid.UUID{tenantA, tenantB} {
		input := sampleProcessInput(tenantID, uuid.New(), 1, uuid.New(), uuid.New(), "shipment.status.v1.test", int64(80+i))
		input.Event.EventType = domain.EventTypeCreated
		input.Event.Data.ToStatus = domain.StatusCarrierAssigned
		_, err := env.Repo.ProcessEvent(ctx, input)
		require.NoError(t, err)
	}

	summaryA, err := env.Repo.GetStatusSummary(ctx, tenantA)
	require.NoError(t, err)
	assert.Equal(t, int64(1), summaryA.TotalShipments)

	summaryB, err := env.Repo.GetStatusSummary(ctx, tenantB)
	require.NoError(t, err)
	assert.Equal(t, int64(1), summaryB.TotalShipments)
}

func TestConcurrentProcessingSameShipmentPreservesVersion(t *testing.T) {
	env := SetupTestEnv(t)
	ctx := context.Background()
	tenantID := uuid.New()
	shipmentID := uuid.New()

	v1 := sampleProcessInput(tenantID, shipmentID, 1, uuid.New(), uuid.New(), "shipment.status.v1.test", 90)
	v1.Event.EventType = domain.EventTypeCreated
	v1.Event.Data.ToStatus = domain.StatusCarrierAssigned
	_, err := env.Repo.ProcessEvent(ctx, v1)
	require.NoError(t, err)

	var wg sync.WaitGroup
	errCh := make(chan error, 2)
	outcomes := make(chan string, 2)

	for _, version := range []int{2, 3} {
		version := version
		wg.Add(1)
		go func() {
			defer wg.Done()
			tx, txErr := env.Repo.BeginTx(ctx)
			if txErr != nil {
				errCh <- txErr
				return
			}
			defer tx.Rollback(ctx)
			input := sampleProcessInput(tenantID, shipmentID, version, uuid.New(), uuid.New(), "shipment.status.v1.test", int64(90+version))
			result, procErr := env.Repo.ProcessEventTx(ctx, tx, input)
			if procErr != nil {
				errCh <- procErr
				return
			}
			if commitErr := tx.Commit(ctx); commitErr != nil {
				errCh <- commitErr
				return
			}
			outcomes <- result.Outcome
		}()
	}
	wg.Wait()
	close(errCh)
	close(outcomes)
	for err := range errCh {
		require.NoError(t, err)
	}

	projection, err := env.Repo.GetProjection(ctx, tenantID, shipmentID)
	require.NoError(t, err)
	require.NotNil(t, projection)
	assert.Equal(t, 3, projection.ShipmentVersion)

	inboxCount := countRows(ctx, env.Pool, `
		SELECT COUNT(*) FROM control_tower.shipment_status_event_inbox
		WHERE tenant_id = $1 AND shipment_id = $2`, tenantID, shipmentID)
	assert.Equal(t, int64(3), inboxCount)
}

func TestProjectionFailureRollsBackInbox(t *testing.T) {
	env := SetupTestEnv(t)
	ctx := context.Background()
	tenantID := uuid.New()
	shipmentID := uuid.New()
	input := sampleProcessInput(tenantID, shipmentID, 1, uuid.New(), uuid.New(), "shipment.status.v1.test", 100)
	input.Event.EventType = domain.EventTypeCreated
	input.Event.Data.ToStatus = domain.StatusCarrierAssigned

	tx, err := env.Pool.Begin(ctx)
	require.NoError(t, err)
	_, err = env.Repo.ProcessEventTx(ctx, tx, input)
	require.NoError(t, err)
	require.NoError(t, tx.Rollback(ctx))

	projection, err := env.Repo.GetProjection(ctx, tenantID, shipmentID)
	require.NoError(t, err)
	assert.Nil(t, projection)

	inboxCount := countRows(ctx, env.Pool, `
		SELECT COUNT(*) FROM control_tower.shipment_status_event_inbox
		WHERE tenant_id = $1 AND shipment_id = $2`, tenantID, shipmentID)
	assert.Equal(t, int64(0), inboxCount)
}
