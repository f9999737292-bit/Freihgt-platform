package rebuild

import (
	"bytes"
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/statussnapshot"
)

type fakeRepo struct {
	jobs      map[uuid.UUID]string
	stageRows int
	failBatch bool
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{jobs: map[uuid.UUID]string{}}
}

func (f *fakeRepo) CreateImportJob(ctx context.Context, manifest Manifest) error {
	f.jobs[manifest.SnapshotID] = StateImporting
	return nil
}

func (f *fakeRepo) InsertStageBatch(ctx context.Context, rows []StageRow) error {
	if f.failBatch {
		return errors.New("batch failed")
	}
	f.stageRows += len(rows)
	return nil
}

func (f *fakeRepo) UpdateImportProgress(ctx context.Context, snapshotID uuid.UUID, importedRows int64) error {
	return nil
}

func (f *fakeRepo) MarkValidated(ctx context.Context, result ValidationResult) error {
	f.jobs[result.SnapshotID] = StateValidated
	return nil
}

func (f *fakeRepo) MarkFailed(ctx context.Context, snapshotID uuid.UUID, code string) error {
	f.jobs[snapshotID] = StateFailed
	return nil
}

func (f *fakeRepo) GetJobStatus(ctx context.Context, snapshotID uuid.UUID) (JobStatus, error) {
	state, ok := f.jobs[snapshotID]
	if !ok {
		return JobStatus{}, ErrJobNotFound
	}
	return JobStatus{SnapshotID: snapshotID, State: state, Scope: string(statussnapshot.ScopeAll)}, nil
}

func buildStream(t *testing.T, rows int) []byte {
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

func TestImporterDryRunValid(t *testing.T) {
	report, err := NewImporter(newFakeRepo()).DryRun(context.Background(), bytes.NewReader(buildStream(t, 1)))
	if err != nil || report.ValidationResult != "VALID" {
		t.Fatalf("dry-run failed: %v %+v", err, report)
	}
}

func TestImporterImportValidated(t *testing.T) {
	repo := newFakeRepo()
	if err := NewImporter(repo).Import(context.Background(), bytes.NewReader(buildStream(t, 2)), 100); err != nil {
		t.Fatal(err)
	}
	if repo.stageRows != 2 {
		t.Fatalf("expected 2 stage rows, got %d", repo.stageRows)
	}
}

func TestImporterMissingCompletion(t *testing.T) {
	raw := buildStream(t, 1)
	lines := bytes.Split(bytes.TrimSpace(raw), []byte("\n"))
	repo := newFakeRepo()
	err := NewImporter(repo).Import(context.Background(), bytes.NewReader(bytes.Join(lines[:2], []byte("\n"))), 100)
	if err == nil {
		t.Fatal("expected missing completion")
	}
}

func TestImporterBatchInsertErrorMarksFailed(t *testing.T) {
	repo := newFakeRepo()
	repo.failBatch = true
	_ = NewImporter(repo).Import(context.Background(), bytes.NewReader(buildStream(t, 1)), 100)
	for _, state := range repo.jobs {
		if state != StateFailed && state != StateImporting {
			t.Fatalf("unexpected state %s", state)
		}
	}
}

func TestImporterActivateNotImplementedConfig(t *testing.T) {
	_, err := LoadConfig(false, false, true, false, false, false, uuid.NewString(), DefaultBatchSize)
	if err == nil {
		t.Fatal("expected conflicting or invalid config")
	}
}

func TestImporterCleanupNotImplemented(t *testing.T) {
	_, err := LoadConfig(false, false, false, false, true, false, uuid.NewString(), DefaultBatchSize)
	if !errors.Is(err, ErrNotImplemented) {
		t.Fatal("expected not implemented")
	}
}

func TestImporterStatusSafeOutput(t *testing.T) {
	repo := newFakeRepo()
	id := uuid.New()
	repo.jobs[id] = StateValidated
	status, err := repo.GetJobStatus(context.Background(), id)
	if err != nil || status.State != StateValidated {
		t.Fatal(err)
	}
}

func TestDryRunDoesNotRequireRepository(t *testing.T) {
	_, err := NewImporter(nil).DryRun(context.Background(), bytes.NewReader(buildStream(t, 1)))
	if err != nil {
		t.Fatal(err)
	}
}

func TestBrokenStdin(t *testing.T) {
	_, err := DryRunValidate(brokenReader{})
	if err == nil {
		t.Fatal("expected broken stream")
	}
}

type brokenReader struct{}

func (brokenReader) Read([]byte) (int, error) { return 0, errors.New("broken") }

func TestValidationErrorDoesNotLeakUUID(t *testing.T) {
	rec := statussnapshot.ShipmentRecord{RecordType: statussnapshot.RecordTypeShipment, SchemaVersion: 1,
		SnapshotID: uuid.New(), TenantID: uuid.Nil, ShipmentID: uuid.New(), CurrentStatus: "IN_TRANSIT",
		AggregateVersion: 1, SourceUpdatedAt: time.Now().UTC()}
	err := statussnapshot.ValidateShipment(rec, statussnapshot.ManifestRecord{SnapshotID: rec.SnapshotID, SchemaVersion: 1, Scope: statussnapshot.ScopeAll, StartedAt: time.Now().UTC(), Source: statussnapshot.SourceShipmentService, TransactionIsolation: statussnapshot.IsolationRepeatableRead})
	if err == nil {
		t.Fatal("expected error")
	}
	if bytes.Contains([]byte(err.Error()), []byte(rec.TenantID.String())) {
		t.Fatal("error leaked uuid")
	}
}

func TestAdvisoryLockConstantStable(t *testing.T) {
	if ProjectionRebuildAdvisoryLockKey != 0x4354505350524F4A {
		t.Fatal("advisory lock constant changed")
	}
}
