package rebuild

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/freight-platform/control-tower-read-model-service/internal/domain"
	"github.com/freight-platform/statussnapshot"
)

// activationFailureHook is set only from integration tests to inject transaction failures.
var activationFailureHook func(point string) error

// activationPauseHook blocks activation mid-transaction for concurrent read tests.
var activationPauseHook func(point string)

const (
	failAfterBackup              = "after_backup"
	failAfterDelete              = "after_delete"
	failAfterPartialInsert       = "after_partial_insert"
	failBeforePostValidate       = "before_post_validation"
	failBeforeActive             = "before_active"
	FailPointAfterJobLock        = "after_job_lock"
	failRollbackAfterDelete      = "rollback_after_delete"
	failRollbackPartialRestore   = "rollback_partial_restore"
	failRollbackBeforeValidate   = "rollback_before_validate"
	failRollbackBeforeRolledBack = "rollback_before_rolled_back"
	failRollbackAfterState       = "rollback_after_state"
)

var nilEventUUID = uuid.Nil

func (r *pgRepository) Activate(ctx context.Context, snapshotID uuid.UUID) (result ActivationResult, err error) {
	start := time.Now().UTC()
	defer func() {
		if err != nil {
			code := ActivationErrorCode(err)
			if code != "" && code != CodeActivationCancelled {
				r.recordActivationError(ctx, snapshotID, code)
			}
		}
	}()
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return result, newActivationError(CodeDatabaseUnavailable, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := AcquireProjectionExclusiveLock(ctx, tx); err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return result, newActivationError(CodeActivationCancelled, ctx.Err())
		}
		if code := ActivationErrorCode(err); code != "" {
			return result, err
		}
		return result, newActivationError(CodeDatabaseUnavailable, err)
	}

	job, err := loadJobForUpdate(ctx, tx, snapshotID)
	if err != nil {
		return result, err
	}

	switch job.State {
	case StateActive:
		return result, newActivationError(CodeSnapshotAlreadyActive, nil)
	case StateRolledBack:
		return result, newActivationError(CodeSnapshotAlreadyRolledBack, nil)
	case StateValidated:
	default:
		return result, newActivationError(CodeInvalidRebuildState, nil)
	}

	activationStarted := time.Now().UTC()
	if _, err := tx.Exec(ctx, `
UPDATE control_tower.shipment_status_projection_rebuild_job
SET state=$2, activation_started_at=$3, updated_at=$3, activation_error_code=NULL
WHERE snapshot_id=$1 AND state=$4`,
		snapshotID, StateActivating, activationStarted, StateValidated); err != nil {
		return result, newActivationError(CodeDatabaseUnavailable, err)
	}

	if activationFailureHook != nil {
		if err := activationFailureHook(FailPointAfterJobLock); err != nil {
			return result, err
		}
	}

	if err := revalidateStageForActivation(ctx, tx, job); err != nil {
		return result, err
	}

	var backupExists int64
	if err := tx.QueryRow(ctx, `
SELECT COUNT(*)::BIGINT FROM control_tower.shipment_status_projection_rebuild_backup WHERE snapshot_id=$1`,
		snapshotID).Scan(&backupExists); err != nil {
		return result, newActivationError(CodeDatabaseUnavailable, err)
	}
	if backupExists > 0 {
		return result, newActivationError(CodeBackupAlreadyExists, nil)
	}

	activeBefore, err := countScopedActive(ctx, tx, job)
	if err != nil {
		return result, err
	}

	backupCreated := time.Now().UTC()
	if err := insertBackupFromActive(ctx, tx, job, backupCreated); err != nil {
		return result, err
	}

	var backupRows int64
	if err := tx.QueryRow(ctx, `
SELECT COUNT(*)::BIGINT FROM control_tower.shipment_status_projection_rebuild_backup WHERE snapshot_id=$1`,
		snapshotID).Scan(&backupRows); err != nil {
		return result, newActivationError(CodeDatabaseUnavailable, err)
	}
	if backupRows != activeBefore {
		return result, newActivationError(CodeBackupRowCountMismatch, nil)
	}

	if activationFailureHook != nil {
		if err := activationFailureHook(failAfterBackup); err != nil {
			return result, err
		}
	}

	if err := deleteScopedActive(ctx, tx, job); err != nil {
		return result, err
	}

	if activationFailureHook != nil {
		if err := activationFailureHook(failAfterDelete); err != nil {
			return result, err
		}
	}

	if activationPauseHook != nil {
		activationPauseHook(failAfterDelete)
	}

	activatedAt := time.Now().UTC()
	activatedRows, err := insertStageIntoActive(ctx, tx, job, activatedAt)
	if err != nil {
		return result, err
	}

	if activationFailureHook != nil {
		if err := activationFailureHook(failAfterPartialInsert); err != nil {
			return result, err
		}
	}

	if activationFailureHook != nil {
		if err := activationFailureHook(failBeforePostValidate); err != nil {
			return result, err
		}
	}

	if err := verifyActiveMatchesStage(ctx, tx, job, activatedAt); err != nil {
		return result, err
	}

	if activationFailureHook != nil {
		if err := activationFailureHook(failBeforeActive); err != nil {
			return result, err
		}
	}

	eligible := true
	_, err = tx.Exec(ctx, `
UPDATE control_tower.shipment_status_projection_rebuild_job
SET state=$2, activated_at=$3, activated_rows=$4, backup_rows=$5, backup_created_at=$6,
    rollback_eligible=$7, updated_at=$3, activation_error_code=NULL
WHERE snapshot_id=$1 AND state=$8`,
		snapshotID, StateActive, activatedAt, activatedRows, backupRows, backupCreated, eligible, StateActivating)
	if err != nil {
		return result, newActivationError(CodeDatabaseUnavailable, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return result, newActivationError(CodeDatabaseUnavailable, err)
	}

	ObserveActivation(job.Scope, "success", "", time.Since(start))
	result = ActivationResult{
		State:            StateActive,
		Scope:            job.Scope,
		ActivatedRows:    activatedRows,
		BackupRows:       backupRows,
		RollbackEligible: eligible,
		ActivatedAt:      activatedAt,
	}
	return result, nil
}

func (r *pgRepository) Rollback(ctx context.Context, snapshotID uuid.UUID) (RollbackResult, error) {
	start := time.Now().UTC()
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return RollbackResult{}, newActivationError(CodeDatabaseUnavailable, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := AcquireProjectionExclusiveLock(ctx, tx); err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return RollbackResult{}, newActivationError(CodeRollbackCancelled, ctx.Err())
		}
		if code := ActivationErrorCode(err); code != "" {
			return RollbackResult{}, err
		}
		return RollbackResult{}, newActivationError(CodeDatabaseUnavailable, err)
	}

	job, err := loadJobForUpdate(ctx, tx, snapshotID)
	if err != nil {
		return RollbackResult{}, err
	}

	switch job.State {
	case StateRolledBack:
		return RollbackResult{}, newActivationError(CodeSnapshotAlreadyRolledBack, nil)
	case StateActive:
	default:
		if job.State == StateValidated {
			return RollbackResult{}, newActivationError(CodeSnapshotNotActive, nil)
		}
		return RollbackResult{}, newActivationError(CodeInvalidRebuildState, nil)
	}

	eligibility, err := assessRollbackEligibility(ctx, tx, job)
	if err != nil {
		return RollbackResult{}, err
	}
	if !eligibility.Eligible {
		return RollbackResult{}, newActivationError(CodeRollbackWindowClosed, nil)
	}

	rollbackStarted := time.Now().UTC()
	if _, err := tx.Exec(ctx, `
UPDATE control_tower.shipment_status_projection_rebuild_job
SET state=$2, rollback_started_at=$3, updated_at=$3, rollback_error_code=NULL
WHERE snapshot_id=$1 AND state=$4`,
		snapshotID, StateRollingBack, rollbackStarted, StateActive); err != nil {
		return RollbackResult{}, newActivationError(CodeDatabaseUnavailable, err)
	}

	if activationFailureHook != nil {
		if err := activationFailureHook(failRollbackAfterState); err != nil {
			return RollbackResult{}, err
		}
	}

	if err := deleteScopedActive(ctx, tx, job); err != nil {
		return RollbackResult{}, err
	}

	if activationFailureHook != nil {
		if err := activationFailureHook(failRollbackAfterDelete); err != nil {
			return RollbackResult{}, err
		}
	}

	restoredRows, err := restoreBackupIntoActive(ctx, tx, job)
	if err != nil {
		return RollbackResult{}, err
	}

	if activationFailureHook != nil {
		if err := activationFailureHook(failRollbackPartialRestore); err != nil {
			return RollbackResult{}, err
		}
	}

	if activationFailureHook != nil {
		if err := activationFailureHook(failRollbackBeforeValidate); err != nil {
			return RollbackResult{}, err
		}
	}

	if err := verifyRestoredMatchesBackup(ctx, tx, job); err != nil {
		return RollbackResult{}, err
	}

	if activationFailureHook != nil {
		if err := activationFailureHook(failRollbackBeforeRolledBack); err != nil {
			return RollbackResult{}, err
		}
	}

	rolledBackAt := time.Now().UTC()
	_, err = tx.Exec(ctx, `
UPDATE control_tower.shipment_status_projection_rebuild_job
SET state=$2, rolled_back_at=$3, rollback_eligible=FALSE, updated_at=$3, rollback_error_code=NULL
WHERE snapshot_id=$1 AND state=$4`,
		snapshotID, StateRolledBack, rolledBackAt, StateRollingBack)
	if err != nil {
		return RollbackResult{}, newActivationError(CodeDatabaseUnavailable, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return RollbackResult{}, newActivationError(CodeDatabaseUnavailable, err)
	}

	ObserveRollback(job.Scope, "success", "", time.Since(start))
	return RollbackResult{
		State:        StateRolledBack,
		RestoredRows: restoredRows,
		RolledBackAt: rolledBackAt,
	}, nil
}

func (r *pgRepository) Cleanup(ctx context.Context, snapshotID uuid.UUID) (CleanupResult, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return CleanupResult{}, newActivationError(CodeDatabaseUnavailable, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	job, err := loadJobForUpdate(ctx, tx, snapshotID)
	if err != nil {
		return CleanupResult{}, err
	}

	switch job.State {
	case StateFailed, StateCancelled, StateRolledBack:
	case StateActive:
		return CleanupResult{}, newActivationError(CodeActiveCleanupForbidden, nil)
	default:
		return CleanupResult{}, newActivationError(CodeInvalidRebuildState, nil)
	}

	tag, err := tx.Exec(ctx, `
DELETE FROM control_tower.shipment_status_projection_rebuild_stage WHERE snapshot_id=$1`, snapshotID)
	if err != nil {
		return CleanupResult{}, newActivationError(CodeDatabaseUnavailable, err)
	}
	stageRemoved := tag.RowsAffected()

	tag, err = tx.Exec(ctx, `
DELETE FROM control_tower.shipment_status_projection_rebuild_backup WHERE snapshot_id=$1`, snapshotID)
	if err != nil {
		return CleanupResult{}, newActivationError(CodeDatabaseUnavailable, err)
	}
	backupRemoved := tag.RowsAffected()

	cleanedAt := time.Now().UTC()
	_, err = tx.Exec(ctx, `
UPDATE control_tower.shipment_status_projection_rebuild_job
SET state=$2, updated_at=$3
WHERE snapshot_id=$1`,
		snapshotID, StateCleaned, cleanedAt)
	if err != nil {
		return CleanupResult{}, newActivationError(CodeDatabaseUnavailable, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return CleanupResult{}, newActivationError(CodeDatabaseUnavailable, err)
	}

	return CleanupResult{
		State:             StateCleaned,
		StageRowsRemoved:  stageRemoved,
		BackupRowsRemoved: backupRemoved,
	}, nil
}

func (r *pgRepository) GetRollbackEligibility(ctx context.Context, snapshotID uuid.UUID) (RollbackEligibility, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return RollbackEligibility{}, newActivationError(CodeDatabaseUnavailable, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	job, err := loadJobByID(ctx, tx, snapshotID)
	if err != nil {
		return RollbackEligibility{}, err
	}
	if job.State != StateActive {
		return RollbackEligibility{Eligible: false, Reason: CodeSnapshotNotActive}, nil
	}
	return assessRollbackEligibility(ctx, tx, job)
}

type lockedJob struct {
	SnapshotID    uuid.UUID
	SchemaVersion int
	Scope         string
	TenantID      *uuid.UUID
	State         string
	ExpectedRows  *int64
	ImportedRows  int64
	TenantCount   *int64
	ExpectedSHA   *string
	ActualSHA     *string
	ActivatedRows *int64
	BackupRows    *int64
}

func loadJobForUpdate(ctx context.Context, tx pgx.Tx, snapshotID uuid.UUID) (lockedJob, error) {
	job, err := loadJobByID(ctx, tx, snapshotID, true)
	if err != nil {
		return lockedJob{}, err
	}
	return job, nil
}

func loadJobByID(ctx context.Context, tx pgx.Tx, snapshotID uuid.UUID, forUpdate ...bool) (lockedJob, error) {
	q := `
SELECT snapshot_id, schema_version, scope, tenant_id, state, expected_rows, imported_rows, tenant_count,
       expected_sha256, actual_sha256, activated_rows, backup_rows
FROM control_tower.shipment_status_projection_rebuild_job
WHERE snapshot_id=$1`
	if len(forUpdate) > 0 && forUpdate[0] {
		q += " FOR UPDATE"
	}
	var job lockedJob
	var expectedSHA, actualSHA *string
	err := tx.QueryRow(ctx, q, snapshotID).Scan(
		&job.SnapshotID, &job.SchemaVersion, &job.Scope, &job.TenantID, &job.State,
		&job.ExpectedRows, &job.ImportedRows, &job.TenantCount, &expectedSHA, &actualSHA,
		&job.ActivatedRows, &job.BackupRows,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return lockedJob{}, ErrJobNotFound
	}
	if err != nil {
		return lockedJob{}, newActivationError(CodeDatabaseUnavailable, err)
	}
	job.ExpectedSHA = expectedSHA
	job.ActualSHA = actualSHA
	return job, nil
}

func revalidateStageForActivation(ctx context.Context, tx pgx.Tx, job lockedJob) error {
	if job.SchemaVersion != SupportedRebuildSchemaVersion {
		return newActivationError(CodeUnsupportedSchemaVersion, nil)
	}
	if job.ExpectedRows == nil || job.TenantCount == nil || job.ExpectedSHA == nil || job.ActualSHA == nil {
		return newActivationError(CodeInvalidRebuildState, nil)
	}
	if *job.ExpectedRows != job.ImportedRows {
		return newActivationError(CodeStageRowCountMismatch, nil)
	}
	if *job.ExpectedSHA != *job.ActualSHA {
		return newActivationError(CodeChecksumMismatch, nil)
	}

	var stageRows, stageTenants int64
	if err := tx.QueryRow(ctx, `
SELECT COUNT(*)::BIGINT, COUNT(DISTINCT tenant_id)::BIGINT
FROM control_tower.shipment_status_projection_rebuild_stage WHERE snapshot_id=$1`,
		job.SnapshotID).Scan(&stageRows, &stageTenants); err != nil {
		return newActivationError(CodeDatabaseUnavailable, err)
	}
	if stageRows != *job.ExpectedRows {
		return newActivationError(CodeStageRowCountMismatch, nil)
	}
	if stageTenants != *job.TenantCount {
		return newActivationError(CodeStageTenantCountMismatch, nil)
	}

	if job.Scope == string(statussnapshot.ScopeTenant) {
		if job.TenantID == nil {
			return newActivationError(CodeStageScopeMismatch, nil)
		}
		var foreign int64
		if err := tx.QueryRow(ctx, `
SELECT COUNT(*)::BIGINT FROM control_tower.shipment_status_projection_rebuild_stage
WHERE snapshot_id=$1 AND tenant_id <> $2`, job.SnapshotID, *job.TenantID).Scan(&foreign); err != nil {
			return newActivationError(CodeDatabaseUnavailable, err)
		}
		if foreign > 0 {
			return newActivationError(CodeStageScopeMismatch, nil)
		}
	}

	rows, err := tx.Query(ctx, `
SELECT current_status, aggregate_version
FROM control_tower.shipment_status_projection_rebuild_stage WHERE snapshot_id=$1`, job.SnapshotID)
	if err != nil {
		return newActivationError(CodeDatabaseUnavailable, err)
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var version int64
		if err := rows.Scan(&status, &version); err != nil {
			return newActivationError(CodeDatabaseUnavailable, err)
		}
		if !domain.IsAllowedShipmentStatus(status) {
			return newActivationError(CodeUnknownShipmentStatus, nil)
		}
		if version < 1 {
			return newActivationError(CodeInvalidAggregateVersion, nil)
		}
	}
	return rows.Err()
}

func countScopedActive(ctx context.Context, tx pgx.Tx, job lockedJob) (int64, error) {
	var count int64
	var err error
	if job.Scope == string(statussnapshot.ScopeTenant) && job.TenantID != nil {
		err = tx.QueryRow(ctx, `
SELECT COUNT(*)::BIGINT FROM control_tower.shipment_status_projection WHERE tenant_id=$1`,
			*job.TenantID).Scan(&count)
	} else {
		err = tx.QueryRow(ctx, `
SELECT COUNT(*)::BIGINT FROM control_tower.shipment_status_projection`).Scan(&count)
	}
	if err != nil {
		return 0, newActivationError(CodeDatabaseUnavailable, err)
	}
	return count, nil
}

func insertBackupFromActive(ctx context.Context, tx pgx.Tx, job lockedJob, backedUpAt time.Time) error {
	var err error
	if job.Scope == string(statussnapshot.ScopeTenant) && job.TenantID != nil {
		_, err = tx.Exec(ctx, `
INSERT INTO control_tower.shipment_status_projection_rebuild_backup (
    snapshot_id, tenant_id, shipment_id, shipment_version, current_status, previous_status,
    last_event_id, last_source_event_id, last_event_type, last_occurred_at, last_consumed_at,
    complete, gap_detected, gap_from_version, gap_to_version,
    projection_source, snapshot_id_prev, authoritative_as_of, rebuilt_at,
    created_at, updated_at, backed_up_at
)
SELECT $1, tenant_id, shipment_id, shipment_version, current_status, previous_status,
       last_event_id, last_source_event_id, last_event_type, last_occurred_at, last_consumed_at,
       complete, gap_detected, gap_from_version, gap_to_version,
       projection_source, snapshot_id, authoritative_as_of, rebuilt_at,
       created_at, updated_at, $2
FROM control_tower.shipment_status_projection
WHERE tenant_id=$3`,
			job.SnapshotID, backedUpAt, *job.TenantID)
	} else {
		_, err = tx.Exec(ctx, `
INSERT INTO control_tower.shipment_status_projection_rebuild_backup (
    snapshot_id, tenant_id, shipment_id, shipment_version, current_status, previous_status,
    last_event_id, last_source_event_id, last_event_type, last_occurred_at, last_consumed_at,
    complete, gap_detected, gap_from_version, gap_to_version,
    projection_source, snapshot_id_prev, authoritative_as_of, rebuilt_at,
    created_at, updated_at, backed_up_at
)
SELECT $1, tenant_id, shipment_id, shipment_version, current_status, previous_status,
       last_event_id, last_source_event_id, last_event_type, last_occurred_at, last_consumed_at,
       complete, gap_detected, gap_from_version, gap_to_version,
       projection_source, snapshot_id, authoritative_as_of, rebuilt_at,
       created_at, updated_at, $2
FROM control_tower.shipment_status_projection`,
			job.SnapshotID, backedUpAt)
	}
	if err != nil {
		return newActivationError(CodeDatabaseUnavailable, err)
	}
	return nil
}

func deleteScopedActive(ctx context.Context, tx pgx.Tx, job lockedJob) error {
	var err error
	if job.Scope == string(statussnapshot.ScopeTenant) && job.TenantID != nil {
		_, err = tx.Exec(ctx, `
DELETE FROM control_tower.shipment_status_projection WHERE tenant_id=$1`, *job.TenantID)
	} else {
		_, err = tx.Exec(ctx, `DELETE FROM control_tower.shipment_status_projection`)
	}
	if err != nil {
		return newActivationError(CodeDatabaseUnavailable, err)
	}
	return nil
}

func insertStageIntoActive(ctx context.Context, tx pgx.Tx, job lockedJob, activatedAt time.Time) (int64, error) {
	var err error
	const insertSQL = `
INSERT INTO control_tower.shipment_status_projection (
    tenant_id, shipment_id, shipment_version, current_status, previous_status,
    last_event_id, last_source_event_id, last_event_type,
    last_occurred_at, last_consumed_at, complete, gap_detected,
    gap_from_version, gap_to_version,
    projection_source, snapshot_id, authoritative_as_of, rebuilt_at,
    created_at, updated_at
)
SELECT
    s.tenant_id, s.shipment_id, s.aggregate_version::INTEGER, s.current_status, s.previous_status,
    COALESCE(s.last_event_id, $2), COALESCE(s.last_source_event_id, $2), s.last_event_type,
    s.source_updated_at, $3, TRUE, FALSE,
    NULL, NULL,
    $4, $1, s.source_updated_at, $3,
    $3, $3
FROM control_tower.shipment_status_projection_rebuild_stage s
WHERE s.snapshot_id = $1`
	if job.Scope == string(statussnapshot.ScopeTenant) && job.TenantID != nil {
		_, err = tx.Exec(ctx, insertSQL+` AND s.tenant_id = $5 ORDER BY s.tenant_id, s.shipment_id`,
			job.SnapshotID, nilEventUUID, activatedAt,
			ProjectionSourceAuthoritativeSnapshot, *job.TenantID)
	} else {
		_, err = tx.Exec(ctx, insertSQL+` ORDER BY s.tenant_id, s.shipment_id`,
			job.SnapshotID, nilEventUUID, activatedAt,
			ProjectionSourceAuthoritativeSnapshot)
	}
	if err != nil {
		return 0, newActivationError(CodeDatabaseUnavailable, err)
	}
	var activatedRows int64
	if err := tx.QueryRow(ctx, `
SELECT COUNT(*)::BIGINT FROM control_tower.shipment_status_projection
WHERE snapshot_id=$1`, job.SnapshotID).Scan(&activatedRows); err != nil {
		return 0, newActivationError(CodeDatabaseUnavailable, err)
	}
	return activatedRows, nil
}

func verifyActiveMatchesStage(ctx context.Context, tx pgx.Tx, job lockedJob, activatedAt time.Time) error {
	var mismatch int64
	scopeFilterP := ""
	scopeFilterS := ""
	args := []any{job.SnapshotID, nilEventUUID, ProjectionSourceAuthoritativeSnapshot}
	if job.Scope == string(statussnapshot.ScopeTenant) && job.TenantID != nil {
		scopeFilterP = " AND p.tenant_id = $4"
		scopeFilterS = " AND s.tenant_id = $4"
		args = append(args, *job.TenantID)
	}

	err := tx.QueryRow(ctx, fmt.Sprintf(`
SELECT COUNT(*)::BIGINT FROM (
    SELECT p.tenant_id, p.shipment_id, p.shipment_version, p.current_status,
           COALESCE(p.previous_status, '') AS previous_status,
           p.last_event_id, p.last_source_event_id, COALESCE(p.last_event_type, '') AS last_event_type,
           p.complete, p.gap_detected,
           p.projection_source, p.snapshot_id
    FROM control_tower.shipment_status_projection p
    WHERE p.snapshot_id = $1%s
    EXCEPT
    SELECT s.tenant_id, s.shipment_id, s.aggregate_version::INTEGER, s.current_status,
           COALESCE(s.previous_status, '') AS previous_status,
           COALESCE(s.last_event_id, $2), COALESCE(s.last_source_event_id, $2),
           COALESCE(s.last_event_type, '') AS last_event_type,
           TRUE, FALSE, $3, $1
    FROM control_tower.shipment_status_projection_rebuild_stage s
    WHERE s.snapshot_id = $1%s
    UNION ALL
    SELECT s.tenant_id, s.shipment_id, s.aggregate_version::INTEGER, s.current_status,
           COALESCE(s.previous_status, '') AS previous_status,
           COALESCE(s.last_event_id, $2), COALESCE(s.last_source_event_id, $2),
           COALESCE(s.last_event_type, '') AS last_event_type,
           TRUE, FALSE, $3, $1
    FROM control_tower.shipment_status_projection_rebuild_stage s
    WHERE s.snapshot_id = $1%s
    EXCEPT
    SELECT p.tenant_id, p.shipment_id, p.shipment_version, p.current_status,
           COALESCE(p.previous_status, '') AS previous_status,
           p.last_event_id, p.last_source_event_id, COALESCE(p.last_event_type, '') AS last_event_type,
           p.complete, p.gap_detected,
           p.projection_source, p.snapshot_id
    FROM control_tower.shipment_status_projection p
    WHERE p.snapshot_id = $1%s
) diff`, scopeFilterP, scopeFilterS, scopeFilterS, scopeFilterP), args...).Scan(&mismatch)
	if err != nil {
		return newActivationError(CodeDatabaseUnavailable, err)
	}
	if mismatch > 0 {
		return newActivationError(CodeActiveProjectionMismatch, nil)
	}

	var stageRows int64
	if err := tx.QueryRow(ctx, `
SELECT COUNT(*)::BIGINT FROM control_tower.shipment_status_projection_rebuild_stage WHERE snapshot_id=$1`,
		job.SnapshotID).Scan(&stageRows); err != nil {
		return newActivationError(CodeDatabaseUnavailable, err)
	}
	var activeRows int64
	activeQ := `SELECT COUNT(*)::BIGINT FROM control_tower.shipment_status_projection WHERE snapshot_id=$1`
	activeArgs := []any{job.SnapshotID}
	if job.Scope == string(statussnapshot.ScopeTenant) && job.TenantID != nil {
		activeQ = `SELECT COUNT(*)::BIGINT FROM control_tower.shipment_status_projection WHERE snapshot_id=$1 AND tenant_id=$2`
		activeArgs = append(activeArgs, *job.TenantID)
	}
	if err := tx.QueryRow(ctx, activeQ, activeArgs...).Scan(&activeRows); err != nil {
		return newActivationError(CodeDatabaseUnavailable, err)
	}
	if activeRows != stageRows {
		return newActivationError(CodeActiveProjectionMismatch, nil)
	}

	var bad int64
	if err := tx.QueryRow(ctx, `
SELECT COUNT(*)::BIGINT FROM control_tower.shipment_status_projection
WHERE snapshot_id=$1 AND (complete IS DISTINCT FROM TRUE OR projection_source <> $2)`,
		job.SnapshotID, ProjectionSourceAuthoritativeSnapshot).Scan(&bad); err != nil {
		return newActivationError(CodeDatabaseUnavailable, err)
	}
	if bad > 0 {
		return newActivationError(CodeActiveProjectionMismatch, nil)
	}
	return nil
}

func restoreBackupIntoActive(ctx context.Context, tx pgx.Tx, job lockedJob) (int64, error) {
	_, err := tx.Exec(ctx, `
INSERT INTO control_tower.shipment_status_projection (
    tenant_id, shipment_id, shipment_version, current_status, previous_status,
    last_event_id, last_source_event_id, last_event_type,
    last_occurred_at, last_consumed_at, complete, gap_detected,
    gap_from_version, gap_to_version,
    projection_source, snapshot_id, authoritative_as_of, rebuilt_at,
    created_at, updated_at
)
SELECT tenant_id, shipment_id, shipment_version, current_status, previous_status,
       last_event_id, last_source_event_id, last_event_type,
       last_occurred_at, last_consumed_at, complete, gap_detected,
       gap_from_version, gap_to_version,
       projection_source, snapshot_id_prev, authoritative_as_of, rebuilt_at,
       created_at, updated_at
FROM control_tower.shipment_status_projection_rebuild_backup
WHERE snapshot_id=$1`, job.SnapshotID)
	if err != nil {
		return 0, newActivationError(CodeDatabaseUnavailable, err)
	}
	var restored int64
	if job.Scope == string(statussnapshot.ScopeTenant) && job.TenantID != nil {
		err = tx.QueryRow(ctx, `
SELECT COUNT(*)::BIGINT FROM control_tower.shipment_status_projection WHERE tenant_id=$1`,
			*job.TenantID).Scan(&restored)
	} else {
		err = tx.QueryRow(ctx, `
SELECT COUNT(*)::BIGINT FROM control_tower.shipment_status_projection`).Scan(&restored)
	}
	if err != nil {
		return 0, newActivationError(CodeDatabaseUnavailable, err)
	}
	return restored, nil
}

func verifyRestoredMatchesBackup(ctx context.Context, tx pgx.Tx, job lockedJob) error {
	var mismatch int64
	err := tx.QueryRow(ctx, `
SELECT COUNT(*)::BIGINT FROM (
    SELECT tenant_id, shipment_id, shipment_version, current_status,
           COALESCE(previous_status, ''), last_event_id, last_source_event_id,
           last_event_type, complete, gap_detected,
           COALESCE(gap_from_version, -1), COALESCE(gap_to_version, -1),
           projection_source, COALESCE(snapshot_id::text, ''), COALESCE(authoritative_as_of, 'epoch'::timestamptz)
    FROM control_tower.shipment_status_projection p
    WHERE EXISTS (
        SELECT 1 FROM control_tower.shipment_status_projection_rebuild_backup b
        WHERE b.snapshot_id=$1 AND b.tenant_id=p.tenant_id AND b.shipment_id=p.shipment_id
    )
    EXCEPT
    SELECT tenant_id, shipment_id, shipment_version, current_status,
           COALESCE(previous_status, ''), last_event_id, last_source_event_id,
           last_event_type, complete, gap_detected,
           COALESCE(gap_from_version, -1), COALESCE(gap_to_version, -1),
           projection_source, COALESCE(snapshot_id_prev::text, ''), COALESCE(authoritative_as_of, 'epoch'::timestamptz)
    FROM control_tower.shipment_status_projection_rebuild_backup
    WHERE snapshot_id=$1
    UNION ALL
    SELECT tenant_id, shipment_id, shipment_version, current_status,
           COALESCE(previous_status, ''), last_event_id, last_source_event_id,
           last_event_type, complete, gap_detected,
           COALESCE(gap_from_version, -1), COALESCE(gap_to_version, -1),
           projection_source, COALESCE(snapshot_id_prev::text, ''), COALESCE(authoritative_as_of, 'epoch'::timestamptz)
    FROM control_tower.shipment_status_projection_rebuild_backup
    WHERE snapshot_id=$1
    EXCEPT
    SELECT tenant_id, shipment_id, shipment_version, current_status,
           COALESCE(previous_status, ''), last_event_id, last_source_event_id,
           last_event_type, complete, gap_detected,
           COALESCE(gap_from_version, -1), COALESCE(gap_to_version, -1),
           projection_source, COALESCE(snapshot_id::text, ''), COALESCE(authoritative_as_of, 'epoch'::timestamptz)
    FROM control_tower.shipment_status_projection p
    WHERE EXISTS (
        SELECT 1 FROM control_tower.shipment_status_projection_rebuild_backup b
        WHERE b.snapshot_id=$1 AND b.tenant_id=p.tenant_id AND b.shipment_id=p.shipment_id
    )
) diff`, job.SnapshotID).Scan(&mismatch)
	if err != nil {
		return newActivationError(CodeDatabaseUnavailable, err)
	}
	if mismatch > 0 {
		return newActivationError(CodeActiveProjectionMismatch, nil)
	}
	if job.BackupRows != nil {
		var activeCount int64
		if job.Scope == string(statussnapshot.ScopeTenant) && job.TenantID != nil {
			err = tx.QueryRow(ctx, `
SELECT COUNT(*)::BIGINT FROM control_tower.shipment_status_projection WHERE tenant_id=$1`,
				*job.TenantID).Scan(&activeCount)
		} else {
			err = tx.QueryRow(ctx, `
SELECT COUNT(*)::BIGINT FROM control_tower.shipment_status_projection`).Scan(&activeCount)
		}
		if err != nil {
			return newActivationError(CodeDatabaseUnavailable, err)
		}
		if activeCount != *job.BackupRows {
			return newActivationError(CodeActiveProjectionMismatch, nil)
		}
	}
	return nil
}

func assessRollbackEligibility(ctx context.Context, tx pgx.Tx, job lockedJob) (RollbackEligibility, error) {
	if job.State != StateActive {
		return RollbackEligibility{Eligible: false, Reason: CodeSnapshotNotActive}, nil
	}
	if job.ActivatedRows == nil || job.BackupRows == nil {
		return RollbackEligibility{Eligible: false, Reason: CodeRollbackWindowClosed}, nil
	}

	var backupCount int64
	if err := tx.QueryRow(ctx, `
SELECT COUNT(*)::BIGINT FROM control_tower.shipment_status_projection_rebuild_backup WHERE snapshot_id=$1`,
		job.SnapshotID).Scan(&backupCount); err != nil {
		return RollbackEligibility{}, newActivationError(CodeDatabaseUnavailable, err)
	}
	if backupCount != *job.BackupRows {
		return RollbackEligibility{Eligible: false, Reason: CodeRollbackWindowClosed}, nil
	}

	activeScopeQ := `SELECT COUNT(*)::BIGINT FROM control_tower.shipment_status_projection WHERE snapshot_id=$1 AND projection_source=$2`
	activeArgs := []any{job.SnapshotID, ProjectionSourceAuthoritativeSnapshot}
	if job.Scope == string(statussnapshot.ScopeTenant) && job.TenantID != nil {
		activeScopeQ = `SELECT COUNT(*)::BIGINT FROM control_tower.shipment_status_projection
WHERE snapshot_id=$1 AND projection_source=$2 AND tenant_id=$3`
		activeArgs = append(activeArgs, *job.TenantID)
	}
	var snapshotRows int64
	if err := tx.QueryRow(ctx, activeScopeQ, activeArgs...).Scan(&snapshotRows); err != nil {
		return RollbackEligibility{}, newActivationError(CodeDatabaseUnavailable, err)
	}
	if snapshotRows != *job.ActivatedRows {
		return RollbackEligibility{Eligible: false, Reason: CodeRollbackWindowClosed}, nil
	}

	var liveRows int64
	liveQ := `SELECT COUNT(*)::BIGINT FROM control_tower.shipment_status_projection
WHERE (projection_source <> $1 OR snapshot_id IS DISTINCT FROM $2)`
	liveArgs := []any{ProjectionSourceAuthoritativeSnapshot, job.SnapshotID}
	if job.Scope == string(statussnapshot.ScopeTenant) && job.TenantID != nil {
		liveQ += ` AND tenant_id = $3`
		liveArgs = append(liveArgs, *job.TenantID)
	}
	if err := tx.QueryRow(ctx, liveQ, liveArgs...).Scan(&liveRows); err != nil {
		return RollbackEligibility{}, newActivationError(CodeDatabaseUnavailable, err)
	}
	if liveRows > 0 {
		return RollbackEligibility{Eligible: false, Reason: CodeRollbackWindowClosed}, nil
	}

	if err := verifyActiveMatchesStage(ctx, tx, job, time.Time{}); err != nil {
		return RollbackEligibility{Eligible: false, Reason: CodeRollbackWindowClosed}, nil
	}

	return RollbackEligibility{Eligible: true, Reason: ""}, nil
}

func (r *pgRepository) recordActivationError(ctx context.Context, snapshotID uuid.UUID, code string) {
	_, _ = r.pool.Exec(ctx, `
UPDATE control_tower.shipment_status_projection_rebuild_job
SET activation_error_code=$2, updated_at=NOW()
WHERE snapshot_id=$1 AND state=$3`, snapshotID, code, StateValidated)
}
