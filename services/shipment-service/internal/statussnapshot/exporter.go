package statussnapshot

import (
	"context"
	"io"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/freight-platform/statussnapshot"
)

type Exporter struct {
	repo   SnapshotRepository
	stdout io.Writer
	stderr io.Writer
	log    *slog.Logger
}

func NewExporter(repo SnapshotRepository, stdout, stderr io.Writer, log *slog.Logger) *Exporter {
	if log == nil {
		log = slog.New(slog.NewTextHandler(stderr, nil))
	}
	return &Exporter{repo: repo, stdout: stdout, stderr: stderr, log: log}
}

type ExportResult struct {
	SnapshotID uuid.UUID
	Stats      SnapshotStats
	SHA256     string
}

func (e *Exporter) Export(ctx context.Context, cfg Config) (ExportResult, error) {
	if err := cfg.Validate(); err != nil {
		return ExportResult{}, err
	}

	snapshotID := uuid.New()
	startedAt := time.Now().UTC()
	manifest := statussnapshot.ManifestRecord{
		RecordType:           statussnapshot.RecordTypeManifest,
		SchemaVersion:        statussnapshot.SchemaVersionV1,
		SnapshotID:           snapshotID,
		Scope:                cfg.Scope(),
		Ordering:             statussnapshot.OrderingTenantIDShipmentID,
		StartedAt:            startedAt,
		TransactionIsolation: statussnapshot.IsolationRepeatableRead,
		Source:               statussnapshot.SourceShipmentService,
	}
	if cfg.Scope() == statussnapshot.ScopeTenant {
		manifest.TenantID = cfg.ParsedTenantID()
	}
	manifestLine, err := statussnapshot.MarshalNDJSON(manifest)
	if err != nil {
		return ExportResult{}, err
	}
	if _, err := e.stdout.Write(manifestLine); err != nil {
		return ExportResult{}, err
	}

	checksum := statussnapshot.NewChecksummer()
	var (
		seq         int64
		tenantCount int64
		prevKey     *statussnapshot.ShipmentKey
		hasPrevious bool
		lastTenant  uuid.UUID
		hasTenant   bool
	)

	stats, err := e.repo.StreamShipmentStatusSnapshot(ctx, SnapshotRequest{
		Scope:    string(cfg.Scope()),
		TenantID: cfg.ParsedTenantID(),
	}, func(row ShipmentSnapshotRow) error {
		if err := ValidateSnapshotRow(row); err != nil {
			return err
		}
		key := statussnapshot.ShipmentKey{TenantID: row.TenantID, ShipmentID: row.ShipmentID}
		if err := validateExporterOrder(prevKey, key, hasPrevious); err != nil {
			return err
		}
		prevKey = &key
		hasPrevious = true

		seq++
		rec := statussnapshot.ShipmentRecord{
			RecordType:        statussnapshot.RecordTypeShipment,
			SchemaVersion:     statussnapshot.SchemaVersionV1,
			SnapshotID:        snapshotID,
			TenantID:          row.TenantID,
			ShipmentID:        row.ShipmentID,
			CurrentStatus:     row.CurrentStatus,
			PreviousStatus:    row.PreviousStatus,
			AggregateVersion:  row.AggregateVersion,
			LastEventID:       row.LastEventID,
			LastSourceEventID: row.LastSourceEventID,
			LastEventType:     row.LastEventType,
			SourceUpdatedAt:   row.SourceUpdatedAt.UTC(),
		}
		if err := statussnapshot.ValidateShipment(rec, manifest); err != nil {
			return err
		}
		if err := checksum.AddCanonicalShipment(rec); err != nil {
			return err
		}
		line, err := statussnapshot.MarshalNDJSON(rec)
		if err != nil {
			return err
		}
		if _, err := e.stdout.Write(line); err != nil {
			return err
		}
		if !hasTenant {
			lastTenant = row.TenantID
			hasTenant = true
			tenantCount = 1
		} else if row.TenantID != lastTenant {
			lastTenant = row.TenantID
			tenantCount++
		}
		return nil
	})
	if err != nil {
		return ExportResult{}, err
	}

	sha := checksum.SumHex()
	if seq == 0 {
		sha = statussnapshot.EmptyStreamChecksumSHA256
	}
	complete := statussnapshot.CompletionRecord{
		RecordType:    statussnapshot.RecordTypeCompletion,
		SchemaVersion: statussnapshot.SchemaVersionV1,
		SnapshotID:    snapshotID,
		RowCount:      seq,
		TenantCount:   tenantCount,
		SHA256:        sha,
		CompletedAt:   time.Now().UTC(),
	}
	if stats.RowCount > 0 && complete.RowCount == 0 {
		complete.RowCount = stats.RowCount
	}
	if stats.TenantCount > 0 && complete.TenantCount == 0 {
		complete.TenantCount = stats.TenantCount
	}
	completeLine, err := statussnapshot.MarshalNDJSON(complete)
	if err != nil {
		return ExportResult{}, err
	}
	if _, err := e.stdout.Write(completeLine); err != nil {
		return ExportResult{}, err
	}

	e.log.Info("snapshot export completed",
		slog.String("scope", string(cfg.Scope())),
		slog.Int64("row_count", complete.RowCount),
		slog.Int64("tenant_count", complete.TenantCount),
	)

	return ExportResult{SnapshotID: snapshotID, Stats: stats, SHA256: sha}, nil
}

func validateExporterOrder(previous *statussnapshot.ShipmentKey, current statussnapshot.ShipmentKey, hasPrevious bool) error {
	if !hasPrevious {
		return nil
	}
	switch statussnapshot.CompareShipmentKeys(current, *previous) {
	case -1:
		return &statussnapshot.ValidationError{Code: statussnapshot.CodeRecordOrderViolation}
	case 0:
		return &statussnapshot.ValidationError{Code: statussnapshot.CodeDuplicateShipment}
	default:
		return nil
	}
}
