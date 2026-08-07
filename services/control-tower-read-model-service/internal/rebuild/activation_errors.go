package rebuild

import "errors"

const (
	CodeActivationConfirmationRequired = "ACTIVATION_CONFIRMATION_REQUIRED"
	CodeRollbackConfirmationRequired   = "ROLLBACK_CONFIRMATION_REQUIRED"
	CodeCleanupConfirmationRequired    = "CLEANUP_CONFIRMATION_REQUIRED"
	CodeProjectionLockTimeout          = "PROJECTION_LOCK_TIMEOUT"
	CodeActivationCancelled            = "ACTIVATION_CANCELLED"
	CodeRollbackCancelled              = "ROLLBACK_CANCELLED"
	CodeInvalidRebuildState            = "INVALID_REBUILD_STATE"
	CodeSnapshotNotActive              = "SNAPSHOT_NOT_ACTIVE"
	CodeSnapshotAlreadyActive          = "SNAPSHOT_ALREADY_ACTIVE"
	CodeSnapshotAlreadyRolledBack      = "SNAPSHOT_ALREADY_ROLLED_BACK"
	CodeRollbackWindowClosed           = "ROLLBACK_WINDOW_CLOSED"
	CodeBackupAlreadyExists            = "BACKUP_ALREADY_EXISTS"
	CodeBackupRowCountMismatch         = "BACKUP_ROW_COUNT_MISMATCH"
	CodeActiveProjectionMismatch       = "ACTIVE_PROJECTION_MISMATCH"
	CodeActiveCleanupForbidden         = "ACTIVE_CLEANUP_FORBIDDEN"
	CodeUnsupportedSchemaVersion       = "UNSUPPORTED_SCHEMA_VERSION"
	CodeChecksumMismatch               = "CHECKSUM_MISMATCH"
	CodeUnknownShipmentStatus          = "UNKNOWN_SHIPMENT_STATUS"
	CodeInvalidAggregateVersion        = "INVALID_AGGREGATE_VERSION"
)

type ActivationError struct {
	Code string
	Err  error
}

func (e *ActivationError) Error() string {
	if e == nil {
		return ""
	}
	return "rebuild activation failed: " + e.Code
}

func (e *ActivationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func ActivationErrorCode(err error) string {
	var ae *ActivationError
	if errors.As(err, &ae) {
		return ae.Code
	}
	if code := ImportErrorCode(err); code != "" {
		return code
	}
	return ""
}

func newActivationError(code string, err error) error {
	return &ActivationError{Code: code, Err: err}
}
