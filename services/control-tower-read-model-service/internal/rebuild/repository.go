package rebuild

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/statussnapshot"
)

var ErrJobNotFound = errors.New("rebuild job not found")

type pgRepository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) RebuildRepository {
	return &pgRepository{pool: pool}
}

func (r *pgRepository) CreateImportJob(ctx context.Context, manifest Manifest) error {
	var existingState string
	err := r.pool.QueryRow(ctx, `
SELECT state FROM control_tower.shipment_status_projection_rebuild_job WHERE snapshot_id=$1`,
		manifest.SnapshotID).Scan(&existingState)
	if err == nil {
		switch existingState {
		case StateValidated:
			return newImportError(CodeSnapshotAlreadyImported, nil)
		case StateImporting:
			return newImportError(CodeSnapshotImportInProgress, nil)
		case StateFailed:
			return newImportError(CodeSnapshotImportReuseForbidden, nil)
		default:
			return newImportError(CodeSnapshotImportReuseForbidden, nil)
		}
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return newImportError(CodeDatabaseUnavailable, err)
	}

	_, err = r.pool.Exec(ctx, `
INSERT INTO control_tower.shipment_status_projection_rebuild_job (
    snapshot_id, schema_version, scope, tenant_id, state, started_at, import_started_at, created_at, updated_at
) VALUES ($1,$2,$3,$4,$5,$6,$6,$6,$6)`,
		manifest.SnapshotID, manifest.SchemaVersion, string(manifest.Scope), manifest.TenantID,
		StateImporting, manifest.StartedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return newImportError(CodeSnapshotImportInProgress, err)
		}
		return newImportError(CodeDatabaseUnavailable, err)
	}
	return nil
}

func (r *pgRepository) InsertStageBatch(ctx context.Context, rows []StageRow) error {
	if len(rows) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	const q = `
INSERT INTO control_tower.shipment_status_projection_rebuild_stage (
    snapshot_id, tenant_id, shipment_id, current_status, previous_status,
    aggregate_version, last_event_id, last_source_event_id, source_updated_at, record_sequence
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`
	for _, row := range rows {
		batch.Queue(q,
			row.SnapshotID, row.TenantID, row.ShipmentID, row.CurrentStatus, row.PreviousStatus,
			row.AggregateVersion, row.LastEventID, row.LastSourceEventID, row.SourceUpdatedAt, row.RecordSequence,
		)
	}
	br := r.pool.SendBatch(ctx, batch)
	defer br.Close()
	for range rows {
		if _, err := br.Exec(); err != nil {
			if isUniqueViolation(err) {
				return &statussnapshot.ValidationError{Code: statussnapshot.CodeDuplicateShipment, Err: err}
			}
			return newImportError(CodeDatabaseConstraintViolation, err)
		}
	}
	if err := br.Close(); err != nil {
		if ctx.Err() != nil {
			return newImportError(CodeImportCancelled, ctx.Err())
		}
		return newImportError(CodeDatabaseUnavailable, err)
	}
	return nil
}

func (r *pgRepository) UpdateImportProgress(ctx context.Context, snapshotID uuid.UUID, importedRows int64) error {
	now := time.Now().UTC()
	tag, err := r.pool.Exec(ctx, `
UPDATE control_tower.shipment_status_projection_rebuild_job
SET imported_rows=GREATEST(imported_rows, $2), updated_at=$3
WHERE snapshot_id=$1 AND state=$4 AND $2 >= imported_rows`,
		snapshotID, importedRows, now, StateImporting)
	if err != nil {
		return newImportError(CodeDatabaseUnavailable, err)
	}
	if tag.RowsAffected() == 0 {
		var state string
		err := r.pool.QueryRow(ctx, `
SELECT state FROM control_tower.shipment_status_projection_rebuild_job WHERE snapshot_id=$1`,
			snapshotID).Scan(&state)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrJobNotFound
		}
		if err != nil {
			return newImportError(CodeDatabaseUnavailable, err)
		}
		if state != StateImporting {
			return nil
		}
	}
	return nil
}

func (r *pgRepository) MarkValidated(ctx context.Context, result ValidationResult) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return newImportError(CodeDatabaseUnavailable, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var scope string
	var tenantID *uuid.UUID
	var state string
	var importedRows int64
	err = tx.QueryRow(ctx, `
SELECT scope, tenant_id, state, imported_rows
FROM control_tower.shipment_status_projection_rebuild_job
WHERE snapshot_id=$1 FOR UPDATE`, result.SnapshotID).Scan(&scope, &tenantID, &state, &importedRows)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrJobNotFound
	}
	if err != nil {
		return newImportError(CodeDatabaseUnavailable, err)
	}
	if state != StateImporting {
		return newImportError(CodeSnapshotAlreadyImported, nil)
	}

	var stageRows, stageTenants int64
	if err := tx.QueryRow(ctx, `
SELECT COUNT(*)::BIGINT, COUNT(DISTINCT tenant_id)::BIGINT
FROM control_tower.shipment_status_projection_rebuild_stage
WHERE snapshot_id=$1`, result.SnapshotID).Scan(&stageRows, &stageTenants); err != nil {
		return newImportError(CodeDatabaseUnavailable, err)
	}
	if stageRows != result.ExpectedRows {
		return newImportError(CodeStageRowCountMismatch, nil)
	}
	if stageTenants != result.TenantCount {
		return newImportError(CodeStageTenantCountMismatch, nil)
	}
	if importedRows != result.ExpectedRows {
		return newImportError(CodeStageRowCountMismatch, nil)
	}

	if scope == string(statussnapshot.ScopeTenant) && tenantID != nil && stageRows > 0 {
		var foreign int64
		if err := tx.QueryRow(ctx, `
SELECT COUNT(*)::BIGINT
FROM control_tower.shipment_status_projection_rebuild_stage
WHERE snapshot_id=$1 AND tenant_id <> $2`, result.SnapshotID, *tenantID).Scan(&foreign); err != nil {
			return newImportError(CodeDatabaseUnavailable, err)
		}
		if foreign > 0 {
			return newImportError(CodeStageScopeMismatch, nil)
		}
	}

	validated := time.Now().UTC()
	_, err = tx.Exec(ctx, `
UPDATE control_tower.shipment_status_projection_rebuild_job
SET state=$2, validated_at=$3, expected_rows=$4, imported_rows=$4, tenant_count=$5,
    expected_sha256=$6, actual_sha256=$7, error_code=NULL, updated_at=$3
WHERE snapshot_id=$1`,
		result.SnapshotID, StateValidated, validated, result.ExpectedRows, result.TenantCount,
		result.ExpectedSHA256, result.ActualSHA256)
	if err != nil {
		return newImportError(CodeDatabaseUnavailable, err)
	}
	return tx.Commit(ctx)
}

func (r *pgRepository) MarkFailed(ctx context.Context, snapshotID uuid.UUID, code string) error {
	failed := time.Now().UTC()
	_, err := r.pool.Exec(ctx, `
UPDATE control_tower.shipment_status_projection_rebuild_job
SET state=$2, failed_at=$3, error_code=$4, updated_at=$3
WHERE snapshot_id=$1 AND state=$5`,
		snapshotID, StateFailed, failed, code, StateImporting)
	if err != nil {
		return newImportError(CodeDatabaseUnavailable, err)
	}
	return nil
}

func (r *pgRepository) GetJobStatus(ctx context.Context, snapshotID uuid.UUID) (JobStatus, error) {
	const q = `
SELECT snapshot_id, state, scope, expected_rows, imported_rows, tenant_count,
       expected_sha256, actual_sha256, started_at, validated_at, error_code
FROM control_tower.shipment_status_projection_rebuild_job WHERE snapshot_id=$1`
	var status JobStatus
	var expected, tenant *int64
	var expectedSHA, actualSHA *string
	var validated *time.Time
	var errCode *string
	err := r.pool.QueryRow(ctx, q, snapshotID).Scan(
		&status.SnapshotID, &status.State, &status.Scope, &expected, &status.ImportedRows, &tenant,
		&expectedSHA, &actualSHA, &status.StartedAt, &validated, &errCode,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return JobStatus{}, ErrJobNotFound
	}
	if err != nil {
		return JobStatus{}, newImportError(CodeDatabaseUnavailable, err)
	}
	status.ExpectedRows = expected
	status.TenantCount = tenant
	status.ValidatedAt = validated
	status.ErrorCode = errCode
	if expectedSHA != nil && actualSHA != nil && *expectedSHA != "" {
		status.ChecksumMatched = *expectedSHA == *actualSHA
	}
	return status, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func safeErrorCode(err error) string {
	if code := statussnapshot.ValidationCode(err); code != "" {
		return code
	}
	if code := ImportErrorCode(err); code != "" {
		return code
	}
	return "IMPORT_FAILED"
}
