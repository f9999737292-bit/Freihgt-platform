//go:build integration

package rebuild

import (
	"bytes"
	"context"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	apprebuild "github.com/freight-platform/control-tower-read-model-service/internal/rebuild"
	"github.com/freight-platform/statussnapshot"
)

func TestDryRunDoesNotModifyDatabase(t *testing.T) {
	ctx := context.Background()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)

	var jobsBefore, stageBefore int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM control_tower.shipment_status_projection_rebuild_job`).Scan(&jobsBefore))
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM control_tower.shipment_status_projection_rebuild_stage`).Scan(&stageBefore))

	tenantID, shipmentID := uuid.New(), uuid.New()
	eventID, sourceID := uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `
INSERT INTO control_tower.shipment_status_projection (
    tenant_id, shipment_id, shipment_version, current_status, last_event_id, last_source_event_id,
    last_event_type, last_occurred_at, last_consumed_at, complete, gap_detected, created_at, updated_at
) VALUES ($1,$2,1,'CARRIER_ASSIGNED',$3,$4,'shipment.created',NOW(),NOW(),TRUE,FALSE,NOW(),NOW())`,
		tenantID, shipmentID, eventID, sourceID)
	require.NoError(t, err)

	report, err := apprebuild.NewImporter(nil).DryRun(ctx, bytes.NewReader(buildIntegrationStream(t, 1)))
	require.NoError(t, err)
	require.Equal(t, "VALID", report.ValidationResult)

	var jobsAfter, stageAfter int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM control_tower.shipment_status_projection_rebuild_job`).Scan(&jobsAfter))
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM control_tower.shipment_status_projection_rebuild_stage`).Scan(&stageAfter))
	require.Equal(t, jobsBefore, jobsAfter)
	require.Equal(t, stageBefore, stageAfter)

	var projectionCount int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM control_tower.shipment_status_projection WHERE tenant_id=$1`, tenantID).Scan(&projectionCount))
	require.Equal(t, int64(1), projectionCount)
}

func TestInvalidDryRunDoesNotModifyDatabase(t *testing.T) {
	ctx := context.Background()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)

	var jobsBefore int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM control_tower.shipment_status_projection_rebuild_job`).Scan(&jobsBefore))

	_, err := apprebuild.NewImporter(nil).DryRun(ctx, bytes.NewReader([]byte(`{"recordType":"shipment"}`)))
	require.Error(t, err)

	var jobsAfter int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM control_tower.shipment_status_projection_rebuild_job`).Scan(&jobsAfter))
	require.Equal(t, jobsBefore, jobsAfter)
}

func TestPersistentImportValidatedState(t *testing.T) {
	ctx := context.Background()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)

	repo := apprebuild.NewRepository(pool)
	stream := buildIntegrationStream(t, 2)
	require.NoError(t, apprebuild.NewImporter(repo).Import(ctx, bytes.NewReader(stream), 100))

	id := extractSnapshotID(t, stream)
	job, err := repo.GetJobStatus(ctx, id)
	require.NoError(t, err)
	require.Equal(t, apprebuild.StateValidated, job.State)
	require.NotNil(t, job.ExpectedRows)
	require.Equal(t, int64(2), *job.ExpectedRows)
	require.Equal(t, int64(2), job.ImportedRows)
	require.True(t, job.ChecksumMatched)

	var stageCount int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM control_tower.shipment_status_projection_rebuild_stage WHERE snapshot_id=$1`, id).Scan(&stageCount))
	require.Equal(t, int64(2), stageCount)
}

func buildIntegrationStream(t *testing.T, rows int) []byte {
	t.Helper()
	id := uuid.New()
	tenantID := uuid.New()
	checksum := statussnapshot.NewChecksummer()
	var buf bytes.Buffer
	m := statussnapshot.ManifestRecord{
		RecordType: statussnapshot.RecordTypeManifest, SchemaVersion: 1, SnapshotID: id, Scope: statussnapshot.ScopeAll,
		Ordering: statussnapshot.OrderingTenantIDShipmentID,
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
		prev := "CARRIER_ASSIGNED"
		eventID, sourceID := uuid.New(), uuid.New()
		rec := statussnapshot.ShipmentRecord{
			RecordType: statussnapshot.RecordTypeShipment, SchemaVersion: 1, SnapshotID: id,
			TenantID: tenantID, ShipmentID: shipID, CurrentStatus: "IN_TRANSIT", PreviousStatus: &prev,
			AggregateVersion: 2, LastEventID: &eventID, LastSourceEventID: &sourceID, SourceUpdatedAt: time.Now().UTC(),
		}
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

func extractSnapshotID(t *testing.T, stream []byte) uuid.UUID {
	t.Helper()
	dec := statussnapshot.NewDecoder(bytes.NewReader(stream), statussnapshot.DecoderOptions{})
	rec, err := dec.Next()
	require.NoError(t, err)
	return rec.(statussnapshot.ManifestRecord).SnapshotID
}

func TestPersistentImportEmptyTenantScope(t *testing.T) {
	ctx := context.Background()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)
	tenantID := uuid.New()
	stream := buildTenantEmptyStream(t, tenantID)
	repo := apprebuild.NewRepository(pool)
	require.NoError(t, apprebuild.NewImporter(repo).Import(ctx, bytes.NewReader(stream), 100))
	id := extractSnapshotID(t, stream)
	job, err := repo.GetJobStatus(ctx, id)
	require.NoError(t, err)
	require.Equal(t, apprebuild.StateValidated, job.State)
	require.Equal(t, int64(0), *job.ExpectedRows)
}

func TestPersistentImportDuplicateSnapshotID(t *testing.T) {
	ctx := context.Background()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)
	repo := apprebuild.NewRepository(pool)
	stream := buildIntegrationStream(t, 1)
	require.NoError(t, apprebuild.NewImporter(repo).Import(ctx, bytes.NewReader(stream), 100))
	err := apprebuild.NewImporter(repo).Import(ctx, bytes.NewReader(stream), 100)
	require.Error(t, err)
	require.Equal(t, apprebuild.CodeSnapshotAlreadyImported, apprebuild.ImportErrorCode(err))
}

func TestPersistentImportBrokenStream(t *testing.T) {
	ctx := context.Background()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)
	repo := apprebuild.NewRepository(pool)
	stream := buildIntegrationStream(t, 1)
	lines := bytes.Split(bytes.TrimSpace(stream), []byte("\n"))
	partial := bytes.Join(lines[:2], []byte("\n"))
	err := apprebuild.NewImporter(repo).Import(ctx, bytes.NewReader(append(partial, '\n')), 100)
	require.Error(t, err)
	id := extractSnapshotID(t, stream)
	job, err2 := repo.GetJobStatus(ctx, id)
	require.NoError(t, err2)
	require.Equal(t, apprebuild.StateFailed, job.State)
}

func buildTenantEmptyStream(t *testing.T, tenantID uuid.UUID) []byte {
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
