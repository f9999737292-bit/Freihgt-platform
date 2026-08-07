//go:build integration

package statussnapshot

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/freight-platform/shipment-service/internal/domain"
	snap "github.com/freight-platform/shipment-service/internal/statussnapshot"
)

func TestOutboxSourceEventIDUniqueConstraint(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	var conname, def string
	err := env.pool.QueryRow(ctx, `
SELECT conname, pg_get_constraintdef(oid)
FROM pg_constraint
WHERE conrelid = 'transport.shipment_event_outbox'::regclass
  AND contype = 'u'`).Scan(&conname, &def)
	require.NoError(t, err)
	require.Contains(t, def, "source_event_id")
}

func TestKafkaEventIDMatchesSnapshotLastEventID(t *testing.T) {
	env := setupTestEnv(t)
	f := env.seedFixtures(t)
	ctx := context.Background()
	user := uuid.New()
	shipment, err := env.repo.CreateShipment(ctx, createParams(f, f.TenantA, f.ShipperA, f.ConsigneeA, f.CarrierA, f.OriginA, f.DestA, f.TransportOrderA, "SHP-KAFKA"), userTransition(user))
	require.NoError(t, err)

	var history domain.ShipmentStatusHistory
	err = env.pool.QueryRow(ctx, `
SELECT id, tenant_id, shipment_id, shipment_version, from_status, to_status, occurred_at
FROM transport.shipment_status_history WHERE shipment_id=$1 ORDER BY shipment_version DESC LIMIT 1`, shipment.ID).Scan(
		&history.ID, &history.TenantID, &history.ShipmentID, &history.ShipmentVersion,
		&history.FromStatus, &history.ToStatus, &history.OccurredAt)
	require.NoError(t, err)

	var outboxEvent domain.ShipmentOutboxEvent
	err = env.pool.QueryRow(ctx, `
SELECT id, tenant_id, aggregate_id, aggregate_version, event_type, source_event_id, payload, status
FROM transport.shipment_event_outbox WHERE source_event_id=$1`, history.ID).Scan(
		&outboxEvent.ID, &outboxEvent.TenantID, &outboxEvent.AggregateID, &outboxEvent.AggregateVersion,
		&outboxEvent.EventType, &outboxEvent.SourceEventID, &outboxEvent.Payload, &outboxEvent.Status)
	require.NoError(t, err)

	var envelope domain.ShipmentStatusEventEnvelope
	require.NoError(t, json.Unmarshal(outboxEvent.Payload, &envelope))
	require.Equal(t, outboxEvent.ID.String(), envelope.EventID)

	var row snap.ShipmentSnapshotRow
	repo := snap.NewPostgresSnapshotRepository(env.pool)
	_, err = repo.StreamShipmentStatusSnapshot(ctx, snap.SnapshotRequest{Scope: "TENANT", TenantID: &f.TenantA}, func(r snap.ShipmentSnapshotRow) error {
		if r.ShipmentID == shipment.ID {
			row = r
		}
		return nil
	})
	require.NoError(t, err)
	require.NotNil(t, row.LastEventID)
	require.Equal(t, outboxEvent.ID, *row.LastEventID)
	require.Equal(t, envelope.EventID, row.LastEventID.String())
}

func TestOutboxStatusSemantics(t *testing.T) {
	env := setupTestEnv(t)
	f := env.seedFixtures(t)
	ctx := context.Background()
	user := uuid.New()
	shipment, err := env.repo.CreateShipment(ctx, createParams(f, f.TenantA, f.ShipperA, f.ConsigneeA, f.CarrierA, f.OriginA, f.DestA, f.TransportOrderA, "SHP-OST"), userTransition(user))
	require.NoError(t, err)

	var historyID uuid.UUID
	require.NoError(t, env.pool.QueryRow(ctx, `
SELECT id FROM transport.shipment_status_history WHERE shipment_id=$1 LIMIT 1`, shipment.ID).Scan(&historyID))

	for _, status := range []string{"PENDING", "PUBLISHED", "FAILED"} {
		t.Run(status, func(t *testing.T) {
			_, err := env.pool.Exec(ctx, `UPDATE transport.shipment_event_outbox SET status=$1 WHERE source_event_id=$2`, status, historyID)
			require.NoError(t, err)
			var row snap.ShipmentSnapshotRow
			repo := snap.NewPostgresSnapshotRepository(env.pool)
			_, err = repo.StreamShipmentStatusSnapshot(ctx, snap.SnapshotRequest{Scope: "TENANT", TenantID: &f.TenantA}, func(r snap.ShipmentSnapshotRow) error {
				if r.ShipmentID == shipment.ID {
					row = r
				}
				return nil
			})
			require.NoError(t, err)
			require.NotNil(t, row.LastEventID, "lastEventId must be present for %s", status)
		})
	}
}

func TestOutboxAggregateIDMismatchRejected(t *testing.T) {
	env := setupTestEnv(t)
	f := env.seedFixtures(t)
	ctx := context.Background()
	user := uuid.New()
	shipment, err := env.repo.CreateShipment(ctx, createParams(f, f.TenantA, f.ShipperA, f.ConsigneeA, f.CarrierA, f.OriginA, f.DestA, f.TransportOrderA, "SHP-OID"), userTransition(user))
	require.NoError(t, err)
	var historyID uuid.UUID
	require.NoError(t, env.pool.QueryRow(ctx, `SELECT id FROM transport.shipment_status_history WHERE shipment_id=$1 LIMIT 1`, shipment.ID).Scan(&historyID))
	_, err = env.pool.Exec(ctx, `UPDATE transport.shipment_event_outbox SET aggregate_id=$1 WHERE source_event_id=$2`, uuid.New(), historyID)
	require.NoError(t, err)
	repo := snap.NewPostgresSnapshotRepository(env.pool)
	_, err = repo.StreamShipmentStatusSnapshot(ctx, snap.SnapshotRequest{Scope: "TENANT", TenantID: &f.TenantA}, func(r snap.ShipmentSnapshotRow) error { return nil })
	require.Error(t, err)
	require.Equal(t, snap.CodeOutboxAggregateIDMismatch, snap.ExportErrorCode(err))
}

func TestOutboxAggregateVersionMismatchRejected(t *testing.T) {
	env := setupTestEnv(t)
	f := env.seedFixtures(t)
	ctx := context.Background()
	user := uuid.New()
	shipment, err := env.repo.CreateShipment(ctx, createParams(f, f.TenantA, f.ShipperA, f.ConsigneeA, f.CarrierA, f.OriginA, f.DestA, f.TransportOrderA, "SHP-OVER"), userTransition(user))
	require.NoError(t, err)
	var historyID uuid.UUID
	require.NoError(t, env.pool.QueryRow(ctx, `SELECT id FROM transport.shipment_status_history WHERE shipment_id=$1 LIMIT 1`, shipment.ID).Scan(&historyID))
	_, err = env.pool.Exec(ctx, `UPDATE transport.shipment_event_outbox SET aggregate_version=999 WHERE source_event_id=$1`, historyID)
	require.NoError(t, err)
	repo := snap.NewPostgresSnapshotRepository(env.pool)
	_, err = repo.StreamShipmentStatusSnapshot(ctx, snap.SnapshotRequest{Scope: "TENANT", TenantID: &f.TenantA}, func(r snap.ShipmentSnapshotRow) error { return nil })
	require.Error(t, err)
	require.Equal(t, snap.CodeOutboxAggregateVersionMismatch, snap.ExportErrorCode(err))
}

func TestExportContextCancellation(t *testing.T) {
	env := setupTestEnv(t)
	f := env.seedFixtures(t)
	ctx := context.Background()
	user := uuid.New()
	for i := 0; i < 100; i++ {
		_, err := env.repo.CreateShipment(ctx, createParams(f, f.TenantA, f.ShipperA, f.ConsigneeA, f.CarrierA, f.OriginA, f.DestA, f.TransportOrderA, fmt.Sprintf("SHP-CAN-%d", i)), userTransition(user))
		require.NoError(t, err)
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	repo := snap.NewPostgresSnapshotRepository(env.pool)
	called := 0
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()
	_, err := repo.StreamShipmentStatusSnapshot(cancelCtx, snap.SnapshotRequest{Scope: "TENANT", TenantID: &f.TenantA}, func(r snap.ShipmentSnapshotRow) error {
		called++
		time.Sleep(5 * time.Millisecond)
		return nil
	})
	require.Error(t, err)
	require.Greater(t, called, 0)
	require.Equal(t, snap.CodeExportCancelled, snap.ExportErrorCode(err))
}

func TestExportCallbackFailure(t *testing.T) {
	env := setupTestEnv(t)
	f := env.seedFixtures(t)
	ctx := context.Background()
	user := uuid.New()
	_, err := env.repo.CreateShipment(ctx, createParams(f, f.TenantA, f.ShipperA, f.ConsigneeA, f.CarrierA, f.OriginA, f.DestA, f.TransportOrderA, "SHP-CB"), userTransition(user))
	require.NoError(t, err)
	repo := snap.NewPostgresSnapshotRepository(env.pool)
	called := 0
	_, err = repo.StreamShipmentStatusSnapshot(ctx, snap.SnapshotRequest{Scope: "TENANT", TenantID: &f.TenantA}, func(r snap.ShipmentSnapshotRow) error {
		called++
		if called >= 1 {
			return context.Canceled
		}
		return nil
	})
	require.Error(t, err)
	require.Equal(t, 1, called)
}

func TestOneHistoryMapsToAtMostOneOutbox(t *testing.T) {
	env := setupTestEnv(t)
	ctx := context.Background()
	var count int64
	err := env.pool.QueryRow(ctx, `
SELECT COUNT(*) FROM (
  SELECT source_event_id FROM transport.shipment_event_outbox GROUP BY source_event_id HAVING COUNT(*) > 1
) dup`).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, int64(0), count)
}
