package statussnapshot

import (
	"errors"
	"fmt"
)

const (
	CodeUnsupportedShipmentStatus      = "UNSUPPORTED_SHIPMENT_STATUS"
	CodeMissingCanonicalStatusHistory  = "MISSING_CANONICAL_STATUS_HISTORY"
	CodeAuthoritativeStatusMismatch    = "AUTHORITATIVE_STATUS_MISMATCH"
	CodeMissingAggregateVersion        = "MISSING_AGGREGATE_VERSION"
	CodeSourceRowCountMismatch         = "SOURCE_ROW_COUNT_MISMATCH"
	CodeSourceTenantCountMismatch      = "SOURCE_TENANT_COUNT_MISMATCH"
	CodeExportCancelled                = "EXPORT_CANCELLED"
	CodeDatabaseUnavailable            = "DATABASE_UNAVAILABLE"
	CodeAmbiguousEventMetadata         = "AMBIGUOUS_EVENT_METADATA"
	CodeOutboxAggregateIDMismatch      = "OUTBOX_AGGREGATE_ID_MISMATCH"
	CodeOutboxAggregateVersionMismatch = "OUTBOX_AGGREGATE_VERSION_MISMATCH"
	CodeInconsistentOutboxEventID      = "INCONSISTENT_OUTBOX_EVENT_ID"
	CodeInconsistentOutboxEventType    = "INCONSISTENT_OUTBOX_EVENT_TYPE"
)

type ExportError struct {
	Code string
	Err  error
}

func (e *ExportError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("statussnapshot export failed: %s", e.Code)
}

func (e *ExportError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func ExportErrorCode(err error) string {
	var ee *ExportError
	if errors.As(err, &ee) {
		return ee.Code
	}
	if code := errors.Unwrap(err); code != nil {
		if c := ExportErrorCode(code); c != "" {
			return c
		}
	}
	return ""
}

func newExportError(code string, err error) error {
	return &ExportError{Code: code, Err: err}
}
