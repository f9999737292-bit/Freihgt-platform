package rebuild

import (
	"context"
	"io"

	"github.com/freight-platform/statussnapshot"
)

type Importer struct {
	repo RebuildRepository
}

func NewImporter(repo RebuildRepository) *Importer {
	return &Importer{repo: repo}
}

func (i *Importer) DryRun(ctx context.Context, r io.Reader) (DryRunReport, error) {
	_ = ctx
	return DryRunValidate(r)
}

func (i *Importer) Import(ctx context.Context, r io.Reader, batchSize int) error {
	dec := statussnapshot.NewDecoder(r, statussnapshot.DecoderOptions{})
	var manifest *statussnapshot.ManifestRecord
	var batch []StageRow
	var completion *statussnapshot.CompletionRecord
	var importedRows int64

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		if err := i.repo.InsertStageBatch(ctx, batch); err != nil {
			if manifest != nil {
				_ = i.repo.MarkFailed(ctx, manifest.SnapshotID, safeErrorCode(err))
			}
			return err
		}
		importedRows += int64(len(batch))
		if err := i.repo.UpdateImportProgress(ctx, manifest.SnapshotID, importedRows); err != nil {
			_ = i.repo.MarkFailed(ctx, manifest.SnapshotID, safeErrorCode(err))
			return err
		}
		batch = batch[:0]
		return nil
	}

	for {
		rec, err := dec.Next()
		if err == io.EOF {
			if manifest == nil {
				return &statussnapshot.ValidationError{Code: statussnapshot.CodeMissingManifest}
			}
			if !dec.Completed() {
				_ = i.repo.MarkFailed(ctx, manifest.SnapshotID, statussnapshot.CodeMissingCompletion)
				return &statussnapshot.ValidationError{Code: statussnapshot.CodeMissingCompletion}
			}
			break
		}
		if err != nil {
			if manifest != nil {
				_ = i.repo.MarkFailed(ctx, manifest.SnapshotID, safeErrorCode(err))
			}
			return err
		}
		switch typed := rec.(type) {
		case statussnapshot.ManifestRecord:
			manifest = &typed
			if err := i.repo.CreateImportJob(ctx, Manifest{
				SnapshotID: typed.SnapshotID, SchemaVersion: typed.SchemaVersion,
				Scope: typed.Scope, TenantID: typed.TenantID, StartedAt: typed.StartedAt,
			}); err != nil {
				return err
			}
		case statussnapshot.ShipmentRecord:
			if manifest == nil {
				return &statussnapshot.ValidationError{Code: statussnapshot.CodeMissingManifest}
			}
			batch = append(batch, StageRow{
				SnapshotID: typed.SnapshotID, TenantID: typed.TenantID, ShipmentID: typed.ShipmentID,
				CurrentStatus: typed.CurrentStatus, PreviousStatus: typed.PreviousStatus,
				AggregateVersion: typed.AggregateVersion, LastEventID: typed.LastEventID,
				LastSourceEventID: typed.LastSourceEventID, LastEventType: typed.LastEventType,
				SourceUpdatedAt: typed.SourceUpdatedAt,
				RecordSequence:  dec.Stats().RowCount,
			})
			if len(batch) >= batchSize {
				if err := flush(); err != nil {
					return err
				}
			}
		case statussnapshot.CompletionRecord:
			completion = &typed
		}
	}

	if manifest == nil {
		return &statussnapshot.ValidationError{Code: statussnapshot.CodeMissingManifest}
	}
	if completion == nil {
		_ = i.repo.MarkFailed(ctx, manifest.SnapshotID, statussnapshot.CodeMissingCompletion)
		return &statussnapshot.ValidationError{Code: statussnapshot.CodeMissingCompletion}
	}
	if err := flush(); err != nil {
		return err
	}

	stats := dec.Stats()
	if err := statussnapshot.ValidateCompletion(*completion, *manifest, stats); err != nil {
		_ = i.repo.MarkFailed(ctx, manifest.SnapshotID, safeErrorCode(err))
		return err
	}

	return i.repo.MarkValidated(ctx, ValidationResult{
		SnapshotID: manifest.SnapshotID, ExpectedRows: completion.RowCount,
		TenantCount: completion.TenantCount, ExpectedSHA256: completion.SHA256, ActualSHA256: stats.ChecksumHex,
	})
}
