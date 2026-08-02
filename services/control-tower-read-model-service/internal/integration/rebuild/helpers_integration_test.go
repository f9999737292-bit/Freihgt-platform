//go:build integration

package rebuild

import (
	"bytes"
	"context"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	apprebuild "github.com/freight-platform/control-tower-read-model-service/internal/rebuild"
	"github.com/freight-platform/control-tower-read-model-service/internal/repository"
	"github.com/freight-platform/statussnapshot"
)

type projectionSnapshot struct {
	TenantID          uuid.UUID
	ShipmentID        uuid.UUID
	ShipmentVersion   int
	CurrentStatus     string
	PreviousStatus    *string
	LastEventID       uuid.UUID
	LastSourceEventID uuid.UUID
	LastEventType     *string
	LastOccurredAt    time.Time
	LastConsumedAt    time.Time
	Complete          bool
	GapDetected       bool
	GapFromVersion    *int
	GapToVersion      *int
	ProjectionSource  string
	SnapshotID        *uuid.UUID
	AuthoritativeAsOf *time.Time
	RebuiltAt         *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func testStatusChangedEventType() string {
	return domain.EventTypeStatusChanged
}

func addLastEventTypeToRecord(rec *statussnapshot.ShipmentRecord) {
	if rec.LastEventID != nil {
		et := testStatusChangedEventType()
		rec.LastEventType = &et
	}
}

func snapshotProjectionRow(t *testing.T, pool *pgxpool.Pool, tenantID, shipmentID uuid.UUID) (projectionSnapshot, bool) {
	t.Helper()
	ctx := context.Background()
	var snap projectionSnapshot
	err := pool.QueryRow(ctx, `
SELECT tenant_id, shipment_id, shipment_version, current_status, previous_status,
       last_event_id, last_source_event_id, last_event_type,
       last_occurred_at, last_consumed_at, complete, gap_detected,
       gap_from_version, gap_to_version,
       projection_source, snapshot_id, authoritative_as_of, rebuilt_at,
       created_at, updated_at
FROM control_tower.shipment_status_projection
WHERE tenant_id=$1 AND shipment_id=$2`, tenantID, shipmentID).Scan(
		&snap.TenantID, &snap.ShipmentID, &snap.ShipmentVersion, &snap.CurrentStatus, &snap.PreviousStatus,
		&snap.LastEventID, &snap.LastSourceEventID, &snap.LastEventType,
		&snap.LastOccurredAt, &snap.LastConsumedAt, &snap.Complete, &snap.GapDetected,
		&snap.GapFromVersion, &snap.GapToVersion,
		&snap.ProjectionSource, &snap.SnapshotID, &snap.AuthoritativeAsOf, &snap.RebuiltAt,
		&snap.CreatedAt, &snap.UpdatedAt,
	)
	if err != nil {
		return projectionSnapshot{}, false
	}
	return snap, true
}

func snapshotAllProjections(t *testing.T, pool *pgxpool.Pool, tenantID uuid.UUID) map[uuid.UUID]projectionSnapshot {
	t.Helper()
	ctx := context.Background()
	rows, err := pool.Query(ctx, `
SELECT tenant_id, shipment_id, shipment_version, current_status, previous_status,
       last_event_id, last_source_event_id, last_event_type,
       last_occurred_at, last_consumed_at, complete, gap_detected,
       gap_from_version, gap_to_version,
       projection_source, snapshot_id, authoritative_as_of, rebuilt_at,
       created_at, updated_at
FROM control_tower.shipment_status_projection
WHERE tenant_id=$1
ORDER BY shipment_id`, tenantID)
	require.NoError(t, err)
	defer rows.Close()

	out := map[uuid.UUID]projectionSnapshot{}
	for rows.Next() {
		var snap projectionSnapshot
		require.NoError(t, rows.Scan(
			&snap.TenantID, &snap.ShipmentID, &snap.ShipmentVersion, &snap.CurrentStatus, &snap.PreviousStatus,
			&snap.LastEventID, &snap.LastSourceEventID, &snap.LastEventType,
			&snap.LastOccurredAt, &snap.LastConsumedAt, &snap.Complete, &snap.GapDetected,
			&snap.GapFromVersion, &snap.GapToVersion,
			&snap.ProjectionSource, &snap.SnapshotID, &snap.AuthoritativeAsOf, &snap.RebuiltAt,
			&snap.CreatedAt, &snap.UpdatedAt,
		))
		out[snap.ShipmentID] = snap
	}
	require.NoError(t, rows.Err())
	return out
}

func countInbox(t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()
	ctx := context.Background()
	var n int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM control_tower.shipment_status_event_inbox`).Scan(&n))
	return n
}

func countDeadLetter(t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()
	ctx := context.Background()
	var n int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM control_tower.shipment_status_event_dead_letter`).Scan(&n))
	return n
}

func countBackupRows(t *testing.T, pool *pgxpool.Pool, snapshotID uuid.UUID) int64 {
	t.Helper()
	ctx := context.Background()
	var n int64
	require.NoError(t, pool.QueryRow(ctx, `
SELECT COUNT(*) FROM control_tower.shipment_status_projection_rebuild_backup WHERE snapshot_id=$1`, snapshotID).Scan(&n))
	return n
}

func requireProjectionsEqual(t *testing.T, want, got map[uuid.UUID]projectionSnapshot) {
	t.Helper()
	require.Equal(t, len(want), len(got), "projection row count")
	for shipID, w := range want {
		g, ok := got[shipID]
		require.True(t, ok, "missing shipment %s", shipID)
		requireProjectionSnapshotEqual(t, w, g)
	}
}

func requireProjectionSnapshotEqual(t *testing.T, want, got projectionSnapshot) {
	t.Helper()
	require.Equal(t, want.TenantID, got.TenantID)
	require.Equal(t, want.ShipmentID, got.ShipmentID)
	require.Equal(t, want.ShipmentVersion, got.ShipmentVersion)
	require.Equal(t, want.CurrentStatus, got.CurrentStatus)
	require.Equal(t, want.PreviousStatus, got.PreviousStatus)
	require.Equal(t, want.LastEventID, got.LastEventID)
	require.Equal(t, want.LastSourceEventID, got.LastSourceEventID)
	require.Equal(t, want.LastEventType, got.LastEventType)
	require.True(t, want.LastOccurredAt.Equal(got.LastOccurredAt))
	require.True(t, want.LastConsumedAt.Equal(got.LastConsumedAt))
	require.Equal(t, want.Complete, got.Complete)
	require.Equal(t, want.GapDetected, got.GapDetected)
	require.Equal(t, want.GapFromVersion, got.GapFromVersion)
	require.Equal(t, want.GapToVersion, got.GapToVersion)
	require.Equal(t, want.ProjectionSource, got.ProjectionSource)
	if want.SnapshotID == nil {
		require.Nil(t, got.SnapshotID)
	} else {
		require.NotNil(t, got.SnapshotID)
		require.Equal(t, *want.SnapshotID, *got.SnapshotID)
	}
	if want.AuthoritativeAsOf == nil {
		require.Nil(t, got.AuthoritativeAsOf)
	} else {
		require.NotNil(t, got.AuthoritativeAsOf)
		require.True(t, want.AuthoritativeAsOf.Equal(*got.AuthoritativeAsOf))
	}
	if want.RebuiltAt == nil {
		require.Nil(t, got.RebuiltAt)
	} else {
		require.NotNil(t, got.RebuiltAt)
		require.True(t, want.RebuiltAt.Equal(*got.RebuiltAt))
	}
}

func buildTenantScopedStream(t *testing.T, tenantID uuid.UUID, rows int) []byte {
	return buildTenantScopedStreamAtVersion(t, tenantID, rows, 2)
}

func buildTenantScopedStreamAtVersion(t *testing.T, tenantID uuid.UUID, rows int, aggregateVersion int64) []byte {
	t.Helper()
	id := uuid.New()
	checksum := statussnapshot.NewChecksummer()
	var buf bytes.Buffer
	m := statussnapshot.ManifestRecord{
		RecordType: statussnapshot.RecordTypeManifest, SchemaVersion: 1, SnapshotID: id,
		Scope: statussnapshot.ScopeTenant, TenantID: &tenantID,
		Ordering:  statussnapshot.OrderingTenantIDShipmentID,
		StartedAt: time.Now().UTC(), TransactionIsolation: statussnapshot.IsolationRepeatableRead,
		Source: statussnapshot.SourceShipmentService,
	}
	line, _ := statussnapshot.MarshalNDJSON(m)
	buf.Write(line)
	shipIDs := make([]uuid.UUID, rows)
	for i := 0; i < rows; i++ {
		shipIDs[i] = uuid.New()
	}
	sort.Slice(shipIDs, func(i, j int) bool { return shipIDs[i].String() < shipIDs[j].String() })
	for _, shipID := range shipIDs {
		prev := domain.StatusCarrierAssigned
		eventID, sourceID := uuid.New(), uuid.New()
		rec := statussnapshot.ShipmentRecord{
			RecordType: statussnapshot.RecordTypeShipment, SchemaVersion: 1, SnapshotID: id,
			TenantID: tenantID, ShipmentID: shipID, CurrentStatus: domain.StatusInTransit, PreviousStatus: &prev,
			AggregateVersion: aggregateVersion, LastEventID: &eventID, LastSourceEventID: &sourceID, SourceUpdatedAt: time.Now().UTC(),
		}
		addLastEventTypeToRecord(&rec)
		_ = checksum.AddCanonicalShipment(rec)
		sline, _ := statussnapshot.MarshalNDJSON(rec)
		buf.Write(sline)
	}
	c := statussnapshot.CompletionRecord{
		RecordType: statussnapshot.RecordTypeCompletion, SchemaVersion: 1, SnapshotID: id,
		RowCount: int64(rows), TenantCount: 1, SHA256: checksum.SumHex(), CompletedAt: time.Now().UTC(),
	}
	cline, _ := statussnapshot.MarshalNDJSON(c)
	buf.Write(cline)
	return buf.Bytes()
}

func buildEmptyTenantStream(t *testing.T, tenantID uuid.UUID) []byte {
	t.Helper()
	id := uuid.New()
	var buf bytes.Buffer
	m := statussnapshot.ManifestRecord{
		RecordType: statussnapshot.RecordTypeManifest, SchemaVersion: 1, SnapshotID: id,
		Scope: statussnapshot.ScopeTenant, TenantID: &tenantID, Ordering: statussnapshot.OrderingTenantIDShipmentID,
		StartedAt: time.Now().UTC(), TransactionIsolation: statussnapshot.IsolationRepeatableRead,
		Source: statussnapshot.SourceShipmentService,
	}
	line, _ := statussnapshot.MarshalNDJSON(m)
	buf.Write(line)
	c := statussnapshot.CompletionRecord{
		RecordType: statussnapshot.RecordTypeCompletion, SchemaVersion: 1, SnapshotID: id,
		RowCount: 0, TenantCount: 0, SHA256: statussnapshot.EmptyStreamChecksumSHA256, CompletedAt: time.Now().UTC(),
	}
	cline, _ := statussnapshot.MarshalNDJSON(c)
	buf.Write(cline)
	return buf.Bytes()
}

type legacyProjectionOpts struct {
	previousStatus    *string
	eventID           uuid.UUID
	sourceEventID     uuid.UUID
	lastEventType     string
	projectionSource  string
	snapshotID        *uuid.UUID
	authoritativeAsOf *time.Time
	rebuiltAt         *time.Time
	complete          *bool
	gapDetected       *bool
	gapFrom           *int
	gapTo             *int
}

func insertLegacyProjection(t *testing.T, pool *pgxpool.Pool, tenantID, shipmentID uuid.UUID, version int, status string, opts ...legacyProjectionOpts) {
	t.Helper()
	ctx := context.Background()
	o := legacyProjectionOpts{
		eventID:          uuid.New(),
		sourceEventID:    uuid.New(),
		lastEventType:    domain.EventTypeCreated,
		projectionSource: apprebuild.ProjectionSourceLiveEvent,
	}
	if len(opts) > 0 {
		merge := opts[0]
		if merge.previousStatus != nil {
			o.previousStatus = merge.previousStatus
		}
		if merge.eventID != uuid.Nil {
			o.eventID = merge.eventID
		}
		if merge.sourceEventID != uuid.Nil {
			o.sourceEventID = merge.sourceEventID
		}
		if merge.lastEventType != "" {
			o.lastEventType = merge.lastEventType
		}
		if merge.projectionSource != "" {
			o.projectionSource = merge.projectionSource
		}
		if merge.snapshotID != nil {
			o.snapshotID = merge.snapshotID
		}
		o.authoritativeAsOf = merge.authoritativeAsOf
		o.rebuiltAt = merge.rebuiltAt
		o.complete = merge.complete
		o.gapDetected = merge.gapDetected
		o.gapFrom = merge.gapFrom
		o.gapTo = merge.gapTo
	}
	complete := true
	if o.complete != nil {
		complete = *o.complete
	}
	gapDetected := false
	if o.gapDetected != nil {
		gapDetected = *o.gapDetected
	}
	now := time.Now().UTC()
	_, err := pool.Exec(ctx, `
INSERT INTO control_tower.shipment_status_projection (
    tenant_id, shipment_id, shipment_version, current_status, previous_status,
    last_event_id, last_source_event_id, last_event_type,
    last_occurred_at, last_consumed_at, complete, gap_detected,
    gap_from_version, gap_to_version,
    projection_source, snapshot_id, authoritative_as_of, rebuilt_at,
    created_at, updated_at
) VALUES (
    $1,$2,$3,$4,$5,
    $6,$7,$8,
    $9,$10,$11,$12,
    $13,$14,
    $15,$16,$17,$18,
    $19,$20
)`,
		tenantID, shipmentID, version, status, o.previousStatus,
		o.eventID, o.sourceEventID, o.lastEventType,
		now, now, complete, gapDetected,
		o.gapFrom, o.gapTo,
		o.projectionSource, o.snapshotID, o.authoritativeAsOf, o.rebuiltAt,
		now, now,
	)
	require.NoError(t, err)
}

func extractSnapshotID(t *testing.T, stream []byte) uuid.UUID {
	t.Helper()
	dec := statussnapshot.NewDecoder(bytes.NewReader(stream), statussnapshot.DecoderOptions{})
	rec, err := dec.Next()
	require.NoError(t, err)
	return rec.(statussnapshot.ManifestRecord).SnapshotID
}

func importSnapshot(t *testing.T, pool *pgxpool.Pool, stream []byte) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	repo := apprebuild.NewRepository(pool)
	require.NoError(t, apprebuild.NewImporter(repo).Import(ctx, bytes.NewReader(stream), 500))
	return extractSnapshotID(t, stream)
}

func buildIntegrationStream(t *testing.T, rows int) []byte {
	t.Helper()
	id := uuid.New()
	tenantID := uuid.New()
	checksum := statussnapshot.NewChecksummer()
	var buf bytes.Buffer
	m := statussnapshot.ManifestRecord{
		RecordType: statussnapshot.RecordTypeManifest, SchemaVersion: 1, SnapshotID: id, Scope: statussnapshot.ScopeAll,
		Ordering:  statussnapshot.OrderingTenantIDShipmentID,
		StartedAt: time.Now().UTC(), TransactionIsolation: statussnapshot.IsolationRepeatableRead,
		Source: statussnapshot.SourceShipmentService,
	}
	line, _ := statussnapshot.MarshalNDJSON(m)
	buf.Write(line)
	shipIDs := make([]uuid.UUID, rows)
	for i := 0; i < rows; i++ {
		shipIDs[i] = uuid.New()
	}
	sort.Slice(shipIDs, func(i, j int) bool { return shipIDs[i].String() < shipIDs[j].String() })
	for _, shipID := range shipIDs {
		prev := domain.StatusCarrierAssigned
		eventID, sourceID := uuid.New(), uuid.New()
		rec := statussnapshot.ShipmentRecord{
			RecordType: statussnapshot.RecordTypeShipment, SchemaVersion: 1, SnapshotID: id,
			TenantID: tenantID, ShipmentID: shipID, CurrentStatus: domain.StatusInTransit, PreviousStatus: &prev,
			AggregateVersion: 2, LastEventID: &eventID, LastSourceEventID: &sourceID, SourceUpdatedAt: time.Now().UTC(),
		}
		addLastEventTypeToRecord(&rec)
		_ = checksum.AddCanonicalShipment(rec)
		sline, _ := statussnapshot.MarshalNDJSON(rec)
		buf.Write(sline)
	}
	c := statussnapshot.CompletionRecord{
		RecordType: statussnapshot.RecordTypeCompletion, SchemaVersion: 1, SnapshotID: id,
		RowCount: int64(rows), TenantCount: 1, SHA256: checksum.SumHex(), CompletedAt: time.Now().UTC(),
	}
	cline, _ := statussnapshot.MarshalNDJSON(c)
	buf.Write(cline)
	return buf.Bytes()
}

func buildIntegrationStreamForTenant(t *testing.T, tenantID uuid.UUID, rows int) []byte {
	t.Helper()
	id := uuid.New()
	checksum := statussnapshot.NewChecksummer()
	var buf bytes.Buffer
	m := statussnapshot.ManifestRecord{
		RecordType: statussnapshot.RecordTypeManifest, SchemaVersion: 1, SnapshotID: id, Scope: statussnapshot.ScopeAll,
		Ordering: statussnapshot.OrderingTenantIDShipmentID, StartedAt: time.Now().UTC(),
		TransactionIsolation: statussnapshot.IsolationRepeatableRead, Source: statussnapshot.SourceShipmentService,
	}
	line, _ := statussnapshot.MarshalNDJSON(m)
	buf.Write(line)
	shipIDs := make([]uuid.UUID, rows)
	for i := 0; i < rows; i++ {
		shipIDs[i] = uuid.New()
	}
	sort.Slice(shipIDs, func(i, j int) bool { return shipIDs[i].String() < shipIDs[j].String() })
	for _, shipID := range shipIDs {
		prev := domain.StatusCarrierAssigned
		eventID, sourceID := uuid.New(), uuid.New()
		rec := statussnapshot.ShipmentRecord{
			RecordType: statussnapshot.RecordTypeShipment, SchemaVersion: 1, SnapshotID: id,
			TenantID: tenantID, ShipmentID: shipID, CurrentStatus: domain.StatusInTransit, PreviousStatus: &prev,
			AggregateVersion: 2, LastEventID: &eventID, LastSourceEventID: &sourceID, SourceUpdatedAt: time.Now().UTC(),
		}
		addLastEventTypeToRecord(&rec)
		_ = checksum.AddCanonicalShipment(rec)
		sline, _ := statussnapshot.MarshalNDJSON(rec)
		buf.Write(sline)
	}
	c := statussnapshot.CompletionRecord{
		RecordType: statussnapshot.RecordTypeCompletion, SchemaVersion: 1, SnapshotID: id,
		RowCount: int64(rows), TenantCount: 1, SHA256: checksum.SumHex(), CompletedAt: time.Now().UTC(),
	}
	cline, _ := statussnapshot.MarshalNDJSON(c)
	buf.Write(cline)
	return buf.Bytes()
}

func buildEmptyAllStream(t *testing.T) []byte {
	t.Helper()
	id := uuid.New()
	var buf bytes.Buffer
	m := statussnapshot.ManifestRecord{
		RecordType: statussnapshot.RecordTypeManifest, SchemaVersion: 1, SnapshotID: id, Scope: statussnapshot.ScopeAll,
		Ordering: statussnapshot.OrderingTenantIDShipmentID, StartedAt: time.Now().UTC(),
		TransactionIsolation: statussnapshot.IsolationRepeatableRead, Source: statussnapshot.SourceShipmentService,
	}
	line, _ := statussnapshot.MarshalNDJSON(m)
	buf.Write(line)
	c := statussnapshot.CompletionRecord{
		RecordType: statussnapshot.RecordTypeCompletion, SchemaVersion: 1, SnapshotID: id,
		RowCount: 0, TenantCount: 0, SHA256: statussnapshot.EmptyStreamChecksumSHA256, CompletedAt: time.Now().UTC(),
	}
	cline, _ := statussnapshot.MarshalNDJSON(c)
	buf.Write(cline)
	return buf.Bytes()
}

func setupLiveConsumerEnv(t *testing.T) (*pgxpool.Pool, *repository.ProjectionRepository, apprebuild.ActivationRepository) {
	t.Helper()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)
	return pool, repository.NewProjectionRepository(pool), apprebuild.NewActivationRepository(pool)
}

func waitForJobState(t *testing.T, pool *pgxpool.Pool, snapshotID uuid.UUID, state string, timeout time.Duration) {
	t.Helper()
	ctx := context.Background()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var current string
		err := pool.QueryRow(ctx, `
SELECT state FROM control_tower.shipment_status_projection_rebuild_job WHERE snapshot_id=$1`, snapshotID).Scan(&current)
		if err == nil && current == state {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("job %s did not reach state %s within %s", snapshotID, state, timeout)
}

func buildStatusChangedInput(tenantID, shipmentID uuid.UUID, version int, topic string, offset int64) repository.ProcessInput {
	now := time.Now().UTC()
	prev := domain.StatusCarrierAssigned
	eventID := uuid.New()
	sourceEventID := uuid.New()
	return repository.ProcessInput{
		Event: domain.ShipmentStatusEvent{
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
				FromStatus: &prev,
				ToStatus:   domain.StatusInTransit,
				ActorType:  "SYSTEM",
			},
		},
		Meta: domain.KafkaRecordMeta{
			Topic:     topic,
			Partition: 0,
			Offset:    offset,
			Key:       shipmentID.String(),
		},
		ReceivedAt: now,
	}
}
