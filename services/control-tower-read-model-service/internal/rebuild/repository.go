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
	now := time.Now().UTC()
	_, err := r.pool.Exec(ctx, `
INSERT INTO control_tower.shipment_status_projection_rebuild_job (
    snapshot_id, schema_version, scope, state, started_at, import_started_at, created_at, updated_at
) VALUES ($1,$2,$3,$4,$5,$5,$5,$5)`,
		manifest.SnapshotID, manifest.SchemaVersion, string(manifest.Scope), StateImporting, now)
	return err
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
			return err
		}
	}
	return br.Close()
}

func (r *pgRepository) MarkValidated(ctx context.Context, result ValidationResult) error {
	validated := time.Now().UTC()
	_, err := r.pool.Exec(ctx, `
UPDATE control_tower.shipment_status_projection_rebuild_job
SET state=$2, validated_at=$3, expected_rows=$4, imported_rows=$4, tenant_count=$5,
    expected_sha256=$6, actual_sha256=$7, error_code=NULL, updated_at=$3
WHERE snapshot_id=$1`,
		result.SnapshotID, StateValidated, validated, result.ExpectedRows, result.TenantCount,
		result.ExpectedSHA256, result.ActualSHA256)
	return err
}

func (r *pgRepository) MarkFailed(ctx context.Context, snapshotID uuid.UUID, code string) error {
	failed := time.Now().UTC()
	_, err := r.pool.Exec(ctx, `
UPDATE control_tower.shipment_status_projection_rebuild_job
SET state=$2, failed_at=$3, error_code=$4, updated_at=$3
WHERE snapshot_id=$1`,
		snapshotID, StateFailed, failed, code)
	return err
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
		return JobStatus{}, err
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
	return "IMPORT_FAILED"
}
