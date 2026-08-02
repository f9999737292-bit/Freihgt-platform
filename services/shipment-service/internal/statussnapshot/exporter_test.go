package statussnapshot

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"

	snap "github.com/freight-platform/statussnapshot"
)

type fakeRepo struct {
	rows     []ShipmentSnapshotRow
	err      error
	preserve bool
}

func (f fakeRepo) StreamShipmentStatusSnapshot(ctx context.Context, request SnapshotRequest, consume func(ShipmentSnapshotRow) error) (SnapshotStats, error) {
	if f.err != nil {
		return SnapshotStats{}, f.err
	}
	ordered := append([]ShipmentSnapshotRow(nil), f.rows...)
	if !f.preserve {
		sort.Slice(ordered, func(i, j int) bool {
			if ordered[i].TenantID != ordered[j].TenantID {
				return ordered[i].TenantID.String() < ordered[j].TenantID.String()
			}
			return ordered[i].ShipmentID.String() < ordered[j].ShipmentID.String()
		})
	}
	tenants := map[uuid.UUID]struct{}{}
	for _, row := range ordered {
		if err := consume(row); err != nil {
			return SnapshotStats{}, err
		}
		tenants[row.TenantID] = struct{}{}
	}
	return SnapshotStats{RowCount: int64(len(ordered)), TenantCount: int64(len(tenants))}, nil
}

func sampleRow(tenantID uuid.UUID) ShipmentSnapshotRow {
	prev := "CARRIER_ASSIGNED"
	eventID := uuid.New()
	sourceID := uuid.New()
	eventType := "shipment.status.changed"
	return ShipmentSnapshotRow{
		TenantID: tenantID, ShipmentID: uuid.New(), CurrentStatus: "IN_TRANSIT", PreviousStatus: &prev,
		AggregateVersion: 2, LastEventID: &eventID, LastSourceEventID: &sourceID, LastEventType: &eventType,
		SourceUpdatedAt: time.Now().UTC(),
	}
}

func TestExporterManifestFirstCompletionLast(t *testing.T) {
	tenantID := uuid.New()
	repo := fakeRepo{rows: []ShipmentSnapshotRow{sampleRow(tenantID)}}
	var stdout bytes.Buffer
	exporter := NewExporter(repo, &stdout, io.Discard, slog.Default())
	cfg, err := LoadConfig(true, "", DefaultBatchSize, DefaultFormat, "-")
	if err != nil {
		t.Fatal(err)
	}
	_, err = exporter.Export(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	lines := bytes.Split(bytes.TrimSpace(stdout.Bytes()), []byte("\n"))
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	if !bytes.Contains(lines[0], []byte(`"recordType":"manifest"`)) {
		t.Fatal("manifest not first")
	}
	if !bytes.Contains(lines[2], []byte(`"recordType":"complete"`)) {
		t.Fatal("completion not last")
	}
}

func TestExporterEmptySnapshot(t *testing.T) {
	repo := fakeRepo{}
	var stdout bytes.Buffer
	exporter := NewExporter(repo, &stdout, io.Discard, nil)
	cfg, _ := LoadConfig(true, "", DefaultBatchSize, DefaultFormat, "-")
	_, err := exporter.Export(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"rowCount":0`)) {
		t.Fatal("expected zero row count")
	}
}

func TestExporterRepositoryErrorNoCompletion(t *testing.T) {
	repo := fakeRepo{err: errors.New("db failed")}
	var stdout bytes.Buffer
	exporter := NewExporter(repo, &stdout, io.Discard, nil)
	cfg, _ := LoadConfig(true, "", DefaultBatchSize, DefaultFormat, "-")
	_, err := exporter.Export(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error")
	}
	if bytes.Contains(stdout.Bytes(), []byte(`"recordType":"complete"`)) {
		t.Fatal("completion must not be written on failure")
	}
}

func TestExporterUnknownStatusRejected(t *testing.T) {
	row := sampleRow(uuid.New())
	row.CurrentStatus = "CREATED"
	repo := fakeRepo{rows: []ShipmentSnapshotRow{row}}
	var stdout bytes.Buffer
	exporter := NewExporter(repo, &stdout, io.Discard, nil)
	cfg, _ := LoadConfig(true, "", DefaultBatchSize, DefaultFormat, "-")
	_, err := exporter.Export(context.Background(), cfg)
	if !errors.Is(err, ErrUnknownStatus) {
		t.Fatalf("expected unknown status, got %v", err)
	}
}

func TestExporterTenantScopeConfig(t *testing.T) {
	id := uuid.New()
	_, err := LoadConfig(false, id.String(), DefaultBatchSize, DefaultFormat, "-")
	if err != nil {
		t.Fatal(err)
	}
}

func TestExporterBrokenWriter(t *testing.T) {
	repo := fakeRepo{rows: []ShipmentSnapshotRow{sampleRow(uuid.New())}}
	exporter := NewExporter(repo, brokenWriter{}, io.Discard, nil)
	cfg, _ := LoadConfig(true, "", DefaultBatchSize, DefaultFormat, "-")
	_, err := exporter.Export(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected write error")
	}
}

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, errors.New("broken") }

func TestExporterContextCancellation(t *testing.T) {
	t.Skip("fake repository does not honor context cancellation yet")
}

func TestExporterStdoutOnlyProtocol(t *testing.T) {
	repo := fakeRepo{}
	var stdout, stderr bytes.Buffer
	exporter := NewExporter(repo, &stdout, &stderr, slog.New(slog.NewTextHandler(&stderr, nil)))
	cfg, _ := LoadConfig(true, "", DefaultBatchSize, DefaultFormat, "-")
	_, _ = exporter.Export(context.Background(), cfg)
	if stderr.String() != "" && bytes.Contains(stderr.Bytes(), []byte(`"recordType"`)) {
		t.Fatal("protocol leaked to stderr")
	}
}

func TestExporterChecksumPresent(t *testing.T) {
	repo := fakeRepo{rows: []ShipmentSnapshotRow{sampleRow(uuid.New())}}
	var stdout bytes.Buffer
	exporter := NewExporter(repo, &stdout, io.Discard, nil)
	cfg, _ := LoadConfig(true, "", DefaultBatchSize, DefaultFormat, "-")
	_, err := exporter.Export(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"sha256":"`)) {
		t.Fatal("missing checksum")
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"ordering":"TENANT_ID_SHIPMENT_ID"`)) {
		t.Fatal("missing ordering in manifest")
	}
	_, err = snap.ValidateStream(bytes.NewReader(stdout.Bytes()), snap.DecoderOptions{})
	if err != nil {
		t.Fatalf("exported stream invalid: %v", err)
	}
}

func TestExporterBrokenWriterMidSnapshot(t *testing.T) {
	repo := fakeRepo{rows: []ShipmentSnapshotRow{sampleRow(uuid.New()), sampleRow(uuid.New())}}
	exporter := NewExporter(repo, &brokenAfterFirstWriter{}, io.Discard, nil)
	cfg, _ := LoadConfig(true, "", DefaultBatchSize, DefaultFormat, "-")
	_, err := exporter.Export(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected write error")
	}
}

type brokenAfterFirstWriter struct{ n int }

func (b *brokenAfterFirstWriter) Write(p []byte) (int, error) {
	b.n++
	if b.n > 1 {
		return 0, errors.New("broken")
	}
	return len(p), nil
}

func TestExporterRecordOrderViolation(t *testing.T) {
	tenantA, tenantB := uuid.New(), uuid.New()
	if tenantA.String() > tenantB.String() {
		tenantA, tenantB = tenantB, tenantA
	}
	repo := fakeRepo{rows: []ShipmentSnapshotRow{
		sampleRowWithIDs(tenantB, uuid.New()),
		sampleRowWithIDs(tenantA, uuid.New()),
	}, preserve: true}
	var stdout bytes.Buffer
	exporter := NewExporter(repo, &stdout, io.Discard, nil)
	cfg, _ := LoadConfig(true, "", DefaultBatchSize, DefaultFormat, "-")
	_, err := exporter.Export(context.Background(), cfg)
	if err == nil || snap.ValidationCode(err) != snap.CodeRecordOrderViolation {
		t.Fatalf("expected order violation, got %v", err)
	}
	if bytes.Contains(stdout.Bytes(), []byte(`"recordType":"complete"`)) {
		t.Fatal("completion must not be written on order violation")
	}
}

func TestExporterTenantScopeManifestTenantID(t *testing.T) {
	tenantID := uuid.New()
	repo := fakeRepo{}
	var stdout bytes.Buffer
	exporter := NewExporter(repo, &stdout, io.Discard, nil)
	cfg, _ := LoadConfig(false, tenantID.String(), DefaultBatchSize, DefaultFormat, "-")
	_, err := exporter.Export(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"scope":"TENANT"`)) || !bytes.Contains(stdout.Bytes(), []byte(tenantID.String())) {
		t.Fatal("tenant manifest must include tenantId")
	}
}

func sampleRowWithIDs(tenantID, shipmentID uuid.UUID) ShipmentSnapshotRow {
	prev := "CARRIER_ASSIGNED"
	eventID := uuid.New()
	sourceID := uuid.New()
	eventType := "shipment.status.changed"
	return ShipmentSnapshotRow{
		TenantID: tenantID, ShipmentID: shipmentID, CurrentStatus: "IN_TRANSIT", PreviousStatus: &prev,
		AggregateVersion: 2, LastEventID: &eventID, LastSourceEventID: &sourceID, LastEventType: &eventType,
		SourceUpdatedAt: time.Now().UTC(),
	}
}

func TestNotImplementedRepository(t *testing.T) {
	var stdout bytes.Buffer
	exporter := NewExporter(NotImplementedRepository{}, &stdout, io.Discard, nil)
	cfg, _ := LoadConfig(true, "", DefaultBatchSize, DefaultFormat, "-")
	_, err := exporter.Export(context.Background(), cfg)
	if !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("expected not implemented, got %v", err)
	}
}

func TestConfigValidation(t *testing.T) {
	if _, err := LoadConfig(false, "", DefaultBatchSize, DefaultFormat, "-"); err == nil {
		t.Fatal("expected config error")
	}
	if _, err := LoadConfig(true, uuid.NewString(), DefaultBatchSize, DefaultFormat, "-"); err == nil {
		t.Fatal("expected conflicting scope error")
	}
}
