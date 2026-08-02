//go:build integration

package projectionrebuild

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type pipelineResult struct {
	ExporterExit int
	ImporterExit int
	ExporterErr  string
	ImporterErr  string
}

type stageRowSnapshot struct {
	TenantID          uuid.UUID
	ShipmentID        uuid.UUID
	CurrentStatus     string
	PreviousStatus    *string
	AggregateVersion  int64
	LastEventID       *uuid.UUID
	LastSourceEventID *uuid.UUID
	SourceUpdatedAt   time.Time
}

func execCommand(t *testing.T, name string, args ...string) *exec.Cmd {
	t.Helper()
	return exec.Command(name, args...)
}

func runExportImportPipeline(t *testing.T, env *dualDBEnv, exporterPath, importerPath, tenantID string, batchSize int, streamMutator func([]byte) []byte) pipelineResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	exporterArgs := []string{"--scope", "all", "--output", "-"}
	if tenantID != "" {
		exporterArgs = []string{"--tenant", tenantID, "--output", "-"}
	}
	if batchSize > 0 {
		exporterArgs = append(exporterArgs, "--batch-size", fmt.Sprintf("%d", batchSize))
	}

	exporter := exec.CommandContext(ctx, exporterPath, exporterArgs...)
	exporter.Env = envWith(env.shipmentDBURL)
	exporterStdout, err := exporter.StdoutPipe()
	require.NoError(t, err)
	var exporterStderr bytes.Buffer
	exporter.Stderr = &exporterStderr

	importer := exec.CommandContext(ctx, importerPath, "--stdin")
	if batchSize > 0 {
		importer.Args = append(importer.Args, "--batch-size", fmt.Sprintf("%d", batchSize))
	}
	importer.Env = envWith(env.readModelDBURL, "CONFIRM_PROJECTION_REBUILD_IMPORT=true")
	var importerStderr bytes.Buffer
	importer.Stderr = &importerStderr

	streamReader := io.Reader(exporterStdout)
	if streamMutator != nil {
		pr, pw := io.Pipe()
		go func() {
			raw, _ := io.ReadAll(exporterStdout)
			raw = streamMutator(raw)
			_, _ = pw.Write(raw)
			_ = pw.Close()
		}()
		streamReader = pr
	}
	importer.Stdin = streamReader

	require.NoError(t, exporter.Start())
	importerErr := importer.Run()
	exporterErr := exporter.Wait()

	return pipelineResult{
		ExporterExit: processExitCode(exporterErr),
		ImporterExit: processExitCode(importerErr),
		ExporterErr:  exporterStderr.String(),
		ImporterErr:  importerStderr.String(),
	}
}

func runExportOnly(t *testing.T, env *dualDBEnv, exporterPath, tenantID string) (stdout []byte, exitCode int, stderr string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, exporterPath, "--tenant", tenantID, "--output", "-")
	cmd.Env = envWith(env.shipmentDBURL)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	return outBuf.Bytes(), processExitCode(err), errBuf.String()
}

func runImportStream(t *testing.T, env *dualDBEnv, importerPath string, stream []byte, batchSize int) (exit int, stderr string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	args := []string{"--stdin"}
	if batchSize > 0 {
		args = append(args, "--batch-size", fmt.Sprintf("%d", batchSize))
	}
	cmd := exec.CommandContext(ctx, importerPath, args...)
	cmd.Env = envWith(env.readModelDBURL, "CONFIRM_PROJECTION_REBUILD_IMPORT=true")
	cmd.Stdin = bytes.NewReader(stream)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	err := cmd.Run()
	return processExitCode(err), errBuf.String()
}

func processExitCode(err error) int {
	if err == nil {
		return 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	return 1
}

func envWith(dbURL string, extra ...string) []string {
	out := make([]string, 0, len(os.Environ())+len(extra)+1)
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "DATABASE_URL=") {
			continue
		}
		out = append(out, kv)
	}
	out = append(out, "DATABASE_URL="+dbURL)
	out = append(out, extra...)
	return out
}

func fetchLatestValidatedSnapshotID(t *testing.T, env *dualDBEnv) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := env.readModelPool.QueryRow(context.Background(), `
SELECT snapshot_id FROM control_tower.shipment_status_projection_rebuild_job
WHERE state='VALIDATED' ORDER BY validated_at DESC LIMIT 1`).Scan(&id)
	require.NoError(t, err)
	return id
}

func fetchStageRows(t *testing.T, env *dualDBEnv, snapshotID uuid.UUID) []stageRowSnapshot {
	t.Helper()
	rows, err := env.readModelPool.Query(context.Background(), `
SELECT tenant_id, shipment_id, current_status, previous_status, aggregate_version,
       last_event_id, last_source_event_id, source_updated_at
FROM control_tower.shipment_status_projection_rebuild_stage
WHERE snapshot_id=$1
ORDER BY tenant_id, shipment_id`, snapshotID)
	require.NoError(t, err)
	defer rows.Close()
	var out []stageRowSnapshot
	for rows.Next() {
		var row stageRowSnapshot
		require.NoError(t, rows.Scan(
			&row.TenantID, &row.ShipmentID, &row.CurrentStatus, &row.PreviousStatus,
			&row.AggregateVersion, &row.LastEventID, &row.LastSourceEventID, &row.SourceUpdatedAt,
		))
		out = append(out, row)
	}
	require.NoError(t, rows.Err())
	return out
}

func fetchSourceRows(t *testing.T, env *dualDBEnv, tenantID uuid.UUID) []stageRowSnapshot {
	t.Helper()
	rows, err := env.shipmentPool.Query(context.Background(), `
WITH ranked AS (
  SELECT h.*, ROW_NUMBER() OVER (
    PARTITION BY h.tenant_id, h.shipment_id
    ORDER BY h.shipment_version DESC, h.occurred_at DESC, h.id DESC
  ) rn FROM transport.shipment_status_history h WHERE h.tenant_id=$1
)
SELECT s.tenant_id, s.id, s.status, r.from_status, s.version, o.id, r.id, r.occurred_at
FROM transport.shipments s
JOIN ranked r ON r.shipment_id=s.id AND r.tenant_id=s.tenant_id AND r.rn=1
LEFT JOIN transport.shipment_event_outbox o ON o.source_event_id=r.id
WHERE s.deleted_at IS NULL AND s.tenant_id=$1
ORDER BY s.tenant_id, s.id`, tenantID)
	require.NoError(t, err)
	defer rows.Close()
	var out []stageRowSnapshot
	for rows.Next() {
		var row stageRowSnapshot
		require.NoError(t, rows.Scan(
			&row.TenantID, &row.ShipmentID, &row.CurrentStatus, &row.PreviousStatus,
			&row.AggregateVersion, &row.LastEventID, &row.LastSourceEventID, &row.SourceUpdatedAt,
		))
		out = append(out, row)
	}
	require.NoError(t, rows.Err())
	return out
}

func tamperChecksumLine(stream []byte) []byte {
	lines := bytes.Split(bytes.TrimSpace(stream), []byte("\n"))
	if len(lines) == 0 {
		return stream
	}
	var completion map[string]any
	if err := json.Unmarshal(lines[len(lines)-1], &completion); err != nil {
		return stream
	}
	completion["sha256"] = "0000000000000000000000000000000000000000000000000000000000000000"
	fixed, _ := json.Marshal(completion)
	lines[len(lines)-1] = fixed
	return append(bytes.Join(lines, []byte("\n")), '\n')
}

func runExportOnlyRetry(t *testing.T, env *dualDBEnv, exporterPath, tenantID string) ([]byte, int, string) {
	t.Helper()
	var stdout []byte
	var code int
	var stderr string
	for attempt := 0; attempt < 5; attempt++ {
		stdout, code, stderr = runExportOnly(t, env, exporterPath, tenantID)
		if code == 0 {
			return stdout, code, stderr
		}
		time.Sleep(300 * time.Millisecond)
	}
	return stdout, code, stderr
}

func truncateBeforeCompletion(stream []byte) []byte {
	lines := bytes.Split(bytes.TrimSpace(stream), []byte("\n"))
	if len(lines) <= 1 {
		return stream
	}
	return append(bytes.Join(lines[:len(lines)-1], []byte("\n")), '\n')
}
