package rebuild

import "errors"

const (
	CodeSnapshotAlreadyImported     = "SNAPSHOT_ALREADY_IMPORTED"
	CodeSnapshotImportInProgress    = "SNAPSHOT_IMPORT_IN_PROGRESS"
	CodeSnapshotImportReuseForbidden = "SNAPSHOT_IMPORT_REUSE_FORBIDDEN"
	CodeStageRowCountMismatch       = "STAGE_ROW_COUNT_MISMATCH"
	CodeStageTenantCountMismatch    = "STAGE_TENANT_COUNT_MISMATCH"
	CodeStageScopeMismatch          = "STAGE_SCOPE_MISMATCH"
	CodeImportCancelled             = "IMPORT_CANCELLED"
	CodeDatabaseUnavailable         = "DATABASE_UNAVAILABLE"
	CodeDatabaseConstraintViolation = "DATABASE_CONSTRAINT_VIOLATION"
)

type ImportError struct {
	Code string
	Err  error
}

func (e *ImportError) Error() string {
	if e == nil {
		return ""
	}
	return "rebuild import failed: " + e.Code
}

func (e *ImportError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func ImportErrorCode(err error) string {
	var ie *ImportError
	if errors.As(err, &ie) {
		return ie.Code
	}
	return ""
}

func newImportError(code string, err error) error {
	return &ImportError{Code: code, Err: err}
}
