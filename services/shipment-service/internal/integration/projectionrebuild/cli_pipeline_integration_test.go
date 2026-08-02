//go:build integration

package projectionrebuild

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCLIPipelineExportImportValidated(t *testing.T) {
	env := setupDualDB(t)
	exporterPath, importerPath := buildCLIBinaries(t)

	tenantID, shipper, consignee, carrier, origin, dest, transportOrder := seedTenant(context.Background(), env.shipmentPool, "CLI")
	createShipment(t, env, tenantID, shipper, consignee, carrier, origin, dest, transportOrder, "CLI-1")
	createShipment(t, env, tenantID, shipper, consignee, carrier, origin, dest, transportOrder, "CLI-2")

	seedReadModelActiveRow(t, env.readModelPool)
	before := snapshotActiveState(t, env.readModelPool)

	res := runExportImportPipeline(t, env, exporterPath, importerPath, tenantID.String(), 500, nil)
	for attempt := 0; attempt < 4 && (res.ExporterExit != 0 || res.ImporterExit != 0); attempt++ {
		time.Sleep(300 * time.Millisecond)
		res = runExportImportPipeline(t, env, exporterPath, importerPath, tenantID.String(), 500, nil)
	}
	require.Equal(t, 0, res.ExporterExit, res.ExporterErr)
	require.Equal(t, 0, res.ImporterExit, res.ImporterErr)

	snapshotID := fetchLatestValidatedSnapshotID(t, env)
	var state string
	var expectedRows, importedRows int64
	var expectedSHA, actualSHA *string
	err := env.readModelPool.QueryRow(context.Background(), `
SELECT state, expected_rows, imported_rows, expected_sha256, actual_sha256
FROM control_tower.shipment_status_projection_rebuild_job WHERE snapshot_id=$1`, snapshotID).Scan(
		&state, &expectedRows, &importedRows, &expectedSHA, &actualSHA)
	require.NoError(t, err)
	require.Equal(t, "VALIDATED", state)
	require.Equal(t, int64(2), expectedRows)
	require.Equal(t, int64(2), importedRows)
	require.NotNil(t, expectedSHA)
	require.NotNil(t, actualSHA)
	require.Equal(t, *expectedSHA, *actualSHA)

	stage := fetchStageRows(t, env, snapshotID)
	source := fetchSourceRows(t, env, tenantID)
	require.Len(t, stage, 2)
	require.Len(t, source, 2)
	for i := range stage {
		require.Equal(t, source[i].TenantID, stage[i].TenantID)
		require.Equal(t, source[i].ShipmentID, stage[i].ShipmentID)
		require.Equal(t, source[i].CurrentStatus, stage[i].CurrentStatus)
		require.Equal(t, source[i].AggregateVersion, stage[i].AggregateVersion)
		if source[i].LastEventID != nil {
			require.NotNil(t, stage[i].LastEventID)
			require.Equal(t, *source[i].LastEventID, *stage[i].LastEventID)
		}
		if source[i].LastSourceEventID != nil {
			require.NotNil(t, stage[i].LastSourceEventID)
			require.Equal(t, *source[i].LastSourceEventID, *stage[i].LastSourceEventID)
		}
	}

	after := snapshotActiveState(t, env.readModelPool)
	require.Equal(t, before.Projection, after.Projection)
	require.Equal(t, before.Inbox, after.Inbox)
	require.Equal(t, before.DeadLetter, after.DeadLetter)
}

func TestCLIPipelineBrokenExporterNoCompletion(t *testing.T) {
	env := setupDualDB(t)
	exporterPath, importerPath := buildCLIBinaries(t)
	tenantID, shipper, consignee, carrier, origin, dest, transportOrder := seedTenant(context.Background(), env.shipmentPool, "BRK")
	createShipment(t, env, tenantID, shipper, consignee, carrier, origin, dest, transportOrder, "OK-1")

	seedReadModelActiveRow(t, env.readModelPool)
	before := snapshotActiveState(t, env.readModelPool)

	stdout, exitCode, expErr := runExportOnlyRetry(t, env, exporterPath, tenantID.String())
	require.Equal(t, 0, exitCode, expErr)
	require.NotEmpty(t, stdout)
	lines := bytes.Split(bytes.TrimSpace(stdout), []byte("\n"))
	require.GreaterOrEqual(t, len(lines), 2)
	partial := append(bytes.Join(lines[:len(lines)-1], []byte("\n")), '\n')

	importExit, importerErr := runImportStream(t, env, importerPath, partial, 100)
	require.NotEqual(t, 0, importExit, importerErr)

	var failed int64
	require.NoError(t, env.readModelPool.QueryRow(context.Background(), `
SELECT COUNT(*) FROM control_tower.shipment_status_projection_rebuild_job WHERE state='FAILED'`).Scan(&failed))
	require.GreaterOrEqual(t, failed, int64(1))

	after := snapshotActiveState(t, env.readModelPool)
	require.Equal(t, before.Projection, after.Projection)
	require.Equal(t, before.Inbox, after.Inbox)
	require.Equal(t, before.DeadLetter, after.DeadLetter)
}

func TestCLIPipelineWrongChecksum(t *testing.T) {
	env := setupDualDB(t)
	exporterPath, importerPath := buildCLIBinaries(t)
	tenantID, shipper, consignee, carrier, origin, dest, transportOrder := seedTenant(context.Background(), env.shipmentPool, "CSM")
	createShipment(t, env, tenantID, shipper, consignee, carrier, origin, dest, transportOrder, "CSM-1")

	seedReadModelActiveRow(t, env.readModelPool)
	before := snapshotActiveState(t, env.readModelPool)

	var stdout []byte
	var expExit int
	stdout, expExit, _ = runExportOnlyRetry(t, env, exporterPath, tenantID.String())
	require.Equal(t, 0, expExit)
	importExit, _ := runImportStream(t, env, importerPath, tamperChecksumLine(stdout), 100)
	require.NotEqual(t, 0, importExit)

	var failed int64
	require.NoError(t, env.readModelPool.QueryRow(context.Background(), `
SELECT COUNT(*) FROM control_tower.shipment_status_projection_rebuild_job WHERE state='FAILED'`).Scan(&failed))
	require.Equal(t, int64(1), failed)

	after := snapshotActiveState(t, env.readModelPool)
	require.Equal(t, before.Projection, after.Projection)
	require.Equal(t, before.Inbox, after.Inbox)
	require.Equal(t, before.DeadLetter, after.DeadLetter)
}

func TestCLIPipelineBrokenImporterMidStream(t *testing.T) {
	env := setupDualDB(t)
	exporterPath, importerPath := buildCLIBinaries(t)
	tenantID, shipper, consignee, carrier, origin, dest, transportOrder := seedTenant(context.Background(), env.shipmentPool, "IMP")
	createShipment(t, env, tenantID, shipper, consignee, carrier, origin, dest, transportOrder, "IMP-1")

	stdout, expExit, _ := runExportOnlyRetry(t, env, exporterPath, tenantID.String())
	require.Equal(t, 0, expExit)
	partial := truncateBeforeCompletion(stdout)
	importExit, _ := runImportStream(t, env, importerPath, partial, 100)
	require.NotEqual(t, 0, importExit)

	var state string
	require.NoError(t, env.readModelPool.QueryRow(context.Background(), `
SELECT state FROM control_tower.shipment_status_projection_rebuild_job ORDER BY created_at DESC LIMIT 1`).Scan(&state))
	require.Equal(t, "FAILED", state)
}

func TestCLIStatusCommandValidated(t *testing.T) {
	env := setupDualDB(t)
	exporterPath, importerPath := buildCLIBinaries(t)
	tenantID, shipper, consignee, carrier, origin, dest, transportOrder := seedTenant(context.Background(), env.shipmentPool, "STS")
	createShipment(t, env, tenantID, shipper, consignee, carrier, origin, dest, transportOrder, "STS-1")

	res := runExportImportPipeline(t, env, exporterPath, importerPath, tenantID.String(), 100, nil)
	for attempt := 0; attempt < 4 && (res.ExporterExit != 0 || res.ImporterExit != 0); attempt++ {
		time.Sleep(300 * time.Millisecond)
		res = runExportImportPipeline(t, env, exporterPath, importerPath, tenantID.String(), 100, nil)
	}
	require.Equal(t, 0, res.ExporterExit, res.ExporterErr)
	require.Equal(t, 0, res.ImporterExit, res.ImporterErr)

	snapshotID := fetchLatestValidatedSnapshotID(t, env)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, importerPath, "--status", "--snapshot-id", snapshotID.String())
	cmd.Env = envWith(env.readModelDBURL)
	out, err := cmd.Output()
	require.NoError(t, err)

	var status map[string]any
	require.NoError(t, json.Unmarshal(out, &status))
	require.Equal(t, "VALIDATED", status["state"])
	require.Equal(t, true, status["checksumMatched"])
}
