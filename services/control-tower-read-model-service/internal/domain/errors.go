package domain

import "fmt"

type ValidationError string

func (e ValidationError) Error() string { return string(e) }

type PermanentError struct {
	Code string
	Err  error
}

func (e *PermanentError) Error() string {
	if e.Err != nil {
		return e.Code + ": " + e.Err.Error()
	}
	return e.Code
}

func (e *PermanentError) Unwrap() error { return e.Err }

const (
	ErrorInvalidJSON              = "INVALID_JSON"
	ErrorUnsupportedSchemaVersion = "UNSUPPORTED_SCHEMA_VERSION"
	ErrorInvalidEventType         = "INVALID_EVENT_TYPE"
	ErrorInvalidEventID           = "INVALID_EVENT_ID"
	ErrorInvalidSourceEventID     = "INVALID_SOURCE_EVENT_ID"
	ErrorInvalidTenantID          = "INVALID_TENANT_ID"
	ErrorInvalidAggregate         = "INVALID_AGGREGATE"
	ErrorInvalidEventData         = "INVALID_EVENT_DATA"
	ErrorEventSchemaViolation     = "EVENT_SCHEMA_VIOLATION"
)

func Permanent(code string, err error) *PermanentError {
	return &PermanentError{Code: code, Err: err}
}

func IsPermanent(err error) (*PermanentError, bool) {
	var pe *PermanentError
	if err == nil {
		return nil, false
	}
	if AsPermanent(err, &pe) {
		return pe, true
	}
	return nil, false
}

func AsPermanent(err error, target **PermanentError) bool {
	if err == nil {
		return false
	}
	if pe, ok := err.(*PermanentError); ok {
		*target = pe
		return true
	}
	return false
}

func fmtErr(code string, format string, args ...any) *PermanentError {
	return Permanent(code, fmt.Errorf(format, args...))
}
