//go:build integration

package rebuild

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	apprebuild "github.com/freight-platform/control-tower-read-model-service/internal/rebuild"
	"github.com/freight-platform/statussnapshot"
)

func TestImportWrongChecksum(t *testing.T) {
	ctx := context.Background()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)
	repo := apprebuild.NewRepository(pool)
	stream := buildIntegrationStream(t, 1)
	id := extractSnapshotID(t, stream)
	tampered := tamperCompletionSHA256(stream)
	err := apprebuild.NewImporter(repo).Import(ctx, bytes.NewReader(tampered), 100)
	require.Error(t, err)
	require.Equal(t, statussnapshot.CodeChecksumMismatch, statussnapshot.ValidationCode(err))
	job, err := repo.GetJobStatus(ctx, id)
	require.NoError(t, err)
	require.Equal(t, apprebuild.StateFailed, job.State)
}

func TestImportWrongRowCount(t *testing.T) {
	ctx := context.Background()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)
	repo := apprebuild.NewRepository(pool)
	stream := buildIntegrationStream(t, 2)
	tampered := tamperCompletionField(stream, "rowCount", float64(99))
	err := apprebuild.NewImporter(repo).Import(ctx, bytes.NewReader(tampered), 100)
	require.Error(t, err)
}

func TestImportWrongTenantCount(t *testing.T) {
	ctx := context.Background()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)
	repo := apprebuild.NewRepository(pool)
	stream := buildIntegrationStream(t, 1)
	tampered := tamperCompletionField(stream, "tenantCount", float64(99))
	err := apprebuild.NewImporter(repo).Import(ctx, bytes.NewReader(tampered), 100)
	require.Error(t, err)
}

func TestImportDuplicateShipmentAcrossBatches(t *testing.T) {
	ctx := context.Background()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)
	repo := apprebuild.NewRepository(pool)
	stream := buildIntegrationStream(t, 2)
	lines := bytes.Split(bytes.TrimSpace(stream), []byte("\n"))
	dup := append([]byte(nil), lines[1]...)
	merged := append([][]byte{lines[0], lines[1], dup}, lines[2:]...)
	tampered := append(bytes.Join(merged, []byte("\n")), '\n')
	err := apprebuild.NewImporter(repo).Import(ctx, bytes.NewReader(tampered), 1)
	require.Error(t, err)
	require.Equal(t, statussnapshot.CodeDuplicateShipment, statussnapshot.ValidationCode(err))
}

func TestImportProtocolTenantScopeMismatch(t *testing.T) {
	ctx := context.Background()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)
	repo := apprebuild.NewRepository(pool)
	tenantA := uuid.New()
	stream := buildIntegrationStream(t, 1)
	lines := bytes.Split(bytes.TrimSpace(stream), []byte("\n"))
	var manifest map[string]any
	require.NoError(t, json.Unmarshal(lines[0], &manifest))
	manifest["scope"] = "TENANT"
	manifest["tenantId"] = tenantA.String()
	mline, _ := json.Marshal(manifest)
	lines[0] = mline
	adjusted := append(bytes.Join(lines, []byte("\n")), '\n')
	err := apprebuild.NewImporter(repo).Import(ctx, bytes.NewReader(adjusted), 100)
	require.Error(t, err)
	require.Equal(t, statussnapshot.CodeTenantScopeMismatch, statussnapshot.ValidationCode(err))
}

func TestMarkValidatedStageScopeMismatch(t *testing.T) {
	ctx := context.Background()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)
	repo := apprebuild.NewRepository(pool)
	tenantA, tenantB := uuid.New(), uuid.New()
	snapshotID := uuid.New()
	require.NoError(t, repo.CreateImportJob(ctx, apprebuild.Manifest{
		SnapshotID: snapshotID, SchemaVersion: 1, Scope: statussnapshot.ScopeTenant,
		TenantID: &tenantA, StartedAt: time.Now().UTC(),
	}))
	require.NoError(t, repo.InsertStageBatch(ctx, []apprebuild.StageRow{{
		SnapshotID: snapshotID, TenantID: tenantB, ShipmentID: uuid.New(),
		CurrentStatus: "CARRIER_ASSIGNED", AggregateVersion: 1, SourceUpdatedAt: time.Now().UTC(), RecordSequence: 1,
	}}))
	require.NoError(t, repo.UpdateImportProgress(ctx, snapshotID, 1))
	err := repo.MarkValidated(ctx, apprebuild.ValidationResult{
		SnapshotID: snapshotID, ExpectedRows: 1, TenantCount: 1,
		ExpectedSHA256: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		ActualSHA256:   "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
	})
	require.Error(t, err)
	require.Equal(t, apprebuild.CodeStageScopeMismatch, apprehendImportCode(err))
}

func apprehendImportCode(err error) string {
	if code := apprebuild.ImportErrorCode(err); code != "" {
		return code
	}
	return statussnapshot.ValidationCode(err)
}

func buildTenantScopeMismatchStream(t *testing.T, manifestTenant, rowTenant uuid.UUID) []byte {
	t.Helper()
	id := uuid.New()
	checksum := statussnapshot.NewChecksummer()
	var buf bytes.Buffer
	m := statussnapshot.ManifestRecord{
		RecordType: statussnapshot.RecordTypeManifest, SchemaVersion: 1, SnapshotID: id,
		Scope: statussnapshot.ScopeTenant, TenantID: &manifestTenant,
		Ordering:  statussnapshot.OrderingTenantIDShipmentID,
		StartedAt: time.Now().UTC(), TransactionIsolation: statussnapshot.IsolationRepeatableRead,
		Source: statussnapshot.SourceShipmentService,
	}
	line, _ := statussnapshot.MarshalNDJSON(m)
	buf.Write(line)
	shipID := uuid.New()
	prev := "CARRIER_ASSIGNED"
	eventID, sourceID := uuid.New(), uuid.New()
	rec := statussnapshot.ShipmentRecord{
		RecordType: statussnapshot.RecordTypeShipment, SchemaVersion: 1, SnapshotID: id,
		TenantID: rowTenant, ShipmentID: shipID, CurrentStatus: "IN_TRANSIT", PreviousStatus: &prev,
		AggregateVersion: 2, LastEventID: &eventID, LastSourceEventID: &sourceID, SourceUpdatedAt: time.Now().UTC(),
	}
	addLastEventTypeToRecord(&rec)
	_ = checksum.AddCanonicalShipment(rec)
	sline, _ := statussnapshot.MarshalNDJSON(rec)
	buf.Write(sline)
	c := statussnapshot.CompletionRecord{
		RecordType: statussnapshot.RecordTypeCompletion, SchemaVersion: 1, SnapshotID: id,
		RowCount: 1, TenantCount: 1, SHA256: checksum.SumHex(), CompletedAt: time.Now().UTC(),
	}
	cline, _ := statussnapshot.MarshalNDJSON(c)
	buf.Write(cline)
	return buf.Bytes()
}

func TestImportContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)
	repo := apprebuild.NewRepository(pool)
	stream := buildIntegrationStream(t, 50)
	err := apprebuild.NewImporter(repo).Import(ctx, bytes.NewReader(stream), 1)
	require.Error(t, err)
}

func TestImportBatchFailureMidImport(t *testing.T) {
	ctx := context.Background()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)
	repo := &failSecondBatchRepo{inner: apprebuild.NewRepository(pool)}
	stream := buildIntegrationStream(t, 3)
	id := extractSnapshotID(t, stream)
	err := apprebuild.NewImporter(repo).Import(ctx, bytes.NewReader(stream), 1)
	require.Error(t, err)
	job, err2 := repo.inner.GetJobStatus(ctx, id)
	require.NoError(t, err2)
	require.Equal(t, apprebuild.StateFailed, job.State)
}

type failSecondBatchRepo struct {
	inner   apprebuild.RebuildRepository
	batches int
}

func (f *failSecondBatchRepo) CreateImportJob(ctx context.Context, m apprebuild.Manifest) error {
	return f.inner.CreateImportJob(ctx, m)
}
func (f *failSecondBatchRepo) InsertStageBatch(ctx context.Context, rows []apprebuild.StageRow) error {
	f.batches++
	if f.batches >= 2 {
		return &apprebuild.ImportError{Code: apprebuild.CodeDatabaseConstraintViolation, Err: errors.New("simulated batch failure")}
	}
	return f.inner.InsertStageBatch(ctx, rows)
}
func (f *failSecondBatchRepo) UpdateImportProgress(ctx context.Context, id uuid.UUID, n int64) error {
	return f.inner.UpdateImportProgress(ctx, id, n)
}
func (f *failSecondBatchRepo) MarkValidated(ctx context.Context, r apprebuild.ValidationResult) error {
	return f.inner.MarkValidated(ctx, r)
}
func (f *failSecondBatchRepo) MarkFailed(ctx context.Context, id uuid.UUID, code string) error {
	return f.inner.MarkFailed(ctx, id, code)
}
func (f *failSecondBatchRepo) GetJobStatus(ctx context.Context, id uuid.UUID) (apprebuild.JobStatus, error) {
	return f.inner.GetJobStatus(ctx, id)
}

func TestStatusResponseValidated(t *testing.T) {
	ctx := context.Background()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)
	repo := apprebuild.NewRepository(pool)
	stream := buildIntegrationStream(t, 1)
	require.NoError(t, apprebuild.NewImporter(repo).Import(ctx, bytes.NewReader(stream), 100))
	id := extractSnapshotID(t, stream)
	job, err := repo.GetJobStatus(ctx, id)
	require.NoError(t, err)
	resp := apprebuild.JobStatusToResponse(job)
	require.Equal(t, apprebuild.StateValidated, resp.State)
	require.True(t, resp.ChecksumMatched)
}

func TestStatusResponseFailed(t *testing.T) {
	ctx := context.Background()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)
	repo := apprebuild.NewRepository(pool)
	stream := buildIntegrationStream(t, 1)
	lines := bytes.Split(bytes.TrimSpace(stream), []byte("\n"))
	partial := append(bytes.Join(lines[:2], []byte("\n")), '\n')
	id := extractSnapshotID(t, stream)
	err := apprebuild.NewImporter(repo).Import(ctx, bytes.NewReader(partial), 100)
	require.Error(t, err)
	job, err2 := repo.GetJobStatus(ctx, id)
	require.NoError(t, err2)
	resp := apprebuild.JobStatusToResponse(job)
	require.Equal(t, apprebuild.StateFailed, resp.State)
	require.NotNil(t, resp.ErrorCode)
}

func TestConcurrentImportSameSnapshotID(t *testing.T) {
	ctx := context.Background()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)
	stream := buildIntegrationStream(t, 5)
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			repo := apprebuild.NewRepository(pool)
			errs[idx] = apprebuild.NewImporter(repo).Import(ctx, bytes.NewReader(stream), 2)
		}(i)
	}
	wg.Wait()
	successes := 0
	for _, err := range errs {
		if err == nil {
			successes++
			continue
		}
		code := apprebuild.ImportErrorCode(err)
		require.Contains(t, []string{
			apprebuild.CodeSnapshotImportInProgress,
			apprebuild.CodeSnapshotAlreadyImported,
			apprebuild.CodeDatabaseConstraintViolation,
		}, code)
	}
	require.Equal(t, 1, successes)
	id := extractSnapshotID(t, stream)
	var jobCount, stageCount int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM control_tower.shipment_status_projection_rebuild_job WHERE snapshot_id=$1`, id).Scan(&jobCount))
	require.Equal(t, int64(1), jobCount)
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM control_tower.shipment_status_projection_rebuild_stage WHERE snapshot_id=$1`, id).Scan(&stageCount))
	require.Equal(t, int64(5), stageCount)
}

func TestDryRunPersistentParityValid(t *testing.T) {
	ctx := context.Background()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)
	stream := buildIntegrationStream(t, 2)
	report, err := apprebuild.NewImporter(nil).DryRun(ctx, bytes.NewReader(stream))
	require.NoError(t, err)
	require.Equal(t, "VALID", report.ValidationResult)
	repo := apprebuild.NewRepository(pool)
	require.NoError(t, apprebuild.NewImporter(repo).Import(ctx, bytes.NewReader(stream), 100))
}

func TestDryRunPersistentParityInvalid(t *testing.T) {
	ctx := context.Background()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)
	stream := buildIntegrationStream(t, 1)
	tampered := tamperCompletionSHA256(stream)
	_, dryErr := apprebuild.NewImporter(nil).DryRun(ctx, bytes.NewReader(tampered))
	require.Error(t, dryErr)
	require.NotEmpty(t, statussnapshot.ValidationCode(dryErr))
	repo := apprebuild.NewRepository(pool)
	require.Error(t, apprebuild.NewImporter(repo).Import(ctx, bytes.NewReader(tampered), 100))
}

func TestMarkFailedDoesNotOverwriteValidated(t *testing.T) {
	ctx := context.Background()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)
	repo := apprebuild.NewRepository(pool)
	stream := buildIntegrationStream(t, 1)
	require.NoError(t, apprebuild.NewImporter(repo).Import(ctx, bytes.NewReader(stream), 100))
	id := extractSnapshotID(t, stream)
	require.NoError(t, repo.MarkFailed(ctx, id, "SHOULD_NOT_APPLY"))
	job, err := repo.GetJobStatus(ctx, id)
	require.NoError(t, err)
	require.Equal(t, apprebuild.StateValidated, job.State)
}

func TestUpdateImportProgressGuard(t *testing.T) {
	ctx := context.Background()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)
	repo := apprebuild.NewRepository(pool)
	stream := buildIntegrationStream(t, 1)
	require.NoError(t, apprebuild.NewImporter(repo).Import(ctx, bytes.NewReader(stream), 100))
	id := extractSnapshotID(t, stream)
	require.NoError(t, repo.UpdateImportProgress(ctx, id, 0))
	job, err := repo.GetJobStatus(ctx, id)
	require.NoError(t, err)
	require.Equal(t, int64(1), job.ImportedRows)
}

func TestMarkValidatedConcurrent(t *testing.T) {
	ctx := context.Background()
	pool := setupMigrationDB(t)
	t.Cleanup(pool.Close)
	repo := apprebuild.NewRepository(pool)
	stream := buildIntegrationStream(t, 1)
	id := extractSnapshotID(t, stream)
	dec := statussnapshot.NewDecoder(bytes.NewReader(stream), statussnapshot.DecoderOptions{})
	rec, err := dec.Next()
	require.NoError(t, err)
	m := rec.(statussnapshot.ManifestRecord)
	require.NoError(t, repo.CreateImportJob(ctx, apprebuild.Manifest{
		SnapshotID: m.SnapshotID, SchemaVersion: m.SchemaVersion, Scope: m.Scope, StartedAt: m.StartedAt,
	}))
	for {
		rec, err := dec.Next()
		if err != nil {
			break
		}
		if ship, ok := rec.(statussnapshot.ShipmentRecord); ok {
			require.NoError(t, repo.InsertStageBatch(ctx, []apprebuild.StageRow{{
				SnapshotID: ship.SnapshotID, TenantID: ship.TenantID, ShipmentID: ship.ShipmentID,
				CurrentStatus: ship.CurrentStatus, AggregateVersion: ship.AggregateVersion,
				SourceUpdatedAt: ship.SourceUpdatedAt, RecordSequence: 1,
			}}))
		}
	}
	stats := dec.Stats()
	result := apprebuild.ValidationResult{
		SnapshotID: id, ExpectedRows: 1, TenantCount: 1,
		ExpectedSHA256: stats.ChecksumHex, ActualSHA256: stats.ChecksumHex,
	}
	require.NoError(t, repo.UpdateImportProgress(ctx, id, 1))
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			errs[idx] = repo.MarkValidated(ctx, result)
		}(i)
	}
	wg.Wait()
	successes := 0
	for _, err := range errs {
		if err == nil {
			successes++
		}
	}
	require.Equal(t, 1, successes)
}

func tamperCompletionSHA256(stream []byte) []byte {
	lines := bytes.Split(bytes.TrimSpace(stream), []byte("\n"))
	lines[len(lines)-1] = bytes.ReplaceAll(lines[len(lines)-1], []byte(`"sha256":"`), []byte(`"sha256":"bad`))
	return append(bytes.Join(lines, []byte("\n")), '\n')
}

func tamperCompletionField(stream []byte, field string, value any) []byte {
	lines := bytes.Split(bytes.TrimSpace(stream), []byte("\n"))
	var completion map[string]any
	_ = json.Unmarshal(lines[len(lines)-1], &completion)
	completion[field] = value
	fixed, _ := json.Marshal(completion)
	lines[len(lines)-1] = fixed
	return append(bytes.Join(lines, []byte("\n")), '\n')
}
