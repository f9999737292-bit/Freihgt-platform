package statussnapshot

import (
	"errors"
	"fmt"
)

const (
	CodeInvalidJSON              = "INVALID_JSON"
	CodeUnknownRecordType        = "UNKNOWN_RECORD_TYPE"
	CodeUnsupportedSchemaVersion = "UNSUPPORTED_SCHEMA_VERSION"
	CodeMissingManifest          = "MISSING_MANIFEST"
	CodeDuplicateManifest        = "DUPLICATE_MANIFEST"
	CodeSnapshotIDMismatch       = "SNAPSHOT_ID_MISMATCH"
	CodeInvalidUUID              = "INVALID_UUID"
	CodeUnknownStatus            = "UNKNOWN_STATUS"
	CodeInvalidAggregateVersion  = "INVALID_AGGREGATE_VERSION"
	CodeDuplicateShipment        = "DUPLICATE_SHIPMENT"
	CodeRowCountMismatch         = "ROW_COUNT_MISMATCH"
	CodeTenantCountMismatch      = "TENANT_COUNT_MISMATCH"
	CodeChecksumMismatch         = "CHECKSUM_MISMATCH"
	CodeMissingCompletion        = "MISSING_COMPLETION"
	CodeDataAfterCompletion      = "DATA_AFTER_COMPLETION"
	CodeRecordTooLarge           = "RECORD_TOO_LARGE"
	CodeBrokenStream             = "BROKEN_STREAM"
	CodeInvalidScope             = "INVALID_SCOPE"
	CodeInvalidTimestamp         = "INVALID_TIMESTAMP"
	CodeShipmentBeforeManifest   = "MISSING_MANIFEST"
	CodeTenantScopeMismatch      = "TENANT_SCOPE_MISMATCH"
	CodeInvalidOrdering          = "INVALID_ORDERING"
	CodeRecordOrderViolation     = "RECORD_ORDER_VIOLATION"
	CodeInconsistentMetadata     = "INCONSISTENT_METADATA"
)

type ValidationError struct {
	Code string
	Err  error
}

func (e *ValidationError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("statussnapshot validation failed: %s", e.Code)
}

func (e *ValidationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func ValidationCode(err error) string {
	var ve *ValidationError
	if errors.As(err, &ve) {
		return ve.Code
	}
	return ""
}

func AsValidationError(err error) (*ValidationError, bool) {
	var ve *ValidationError
	if errors.As(err, &ve) {
		return ve, true
	}
	return nil, false
}
