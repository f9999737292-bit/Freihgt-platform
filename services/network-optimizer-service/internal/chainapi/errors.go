package chainapi

import "net/http"

// StatusError is a public chain-search failure. Message and details are safe to return.
// Cause is logged only and must not be copied into the response.
type StatusError struct {
	Status  int
	Code    string
	Message string
	Details map[string]any
	Headers map[string]string
	Cause   error
}

func (e *StatusError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func (e *StatusError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func badRequest(message string) *StatusError {
	return &StatusError{Status: http.StatusBadRequest, Code: "VALIDATION_ERROR", Message: message, Details: map[string]any{}}
}

func unprocessable(message string, details map[string]any) *StatusError {
	if details == nil {
		details = map[string]any{}
	}
	return &StatusError{Status: http.StatusUnprocessableEntity, Code: "VALIDATION_ERROR", Message: message, Details: details}
}

func notFound() *StatusError {
	return &StatusError{Status: http.StatusNotFound, Code: "NOT_FOUND", Message: "chain search not found", Details: map[string]any{}}
}

func unavailable(cause error) *StatusError {
	return &StatusError{
		Status:  http.StatusServiceUnavailable,
		Code:    "SERVICE_UNAVAILABLE",
		Message: "chain search dependency unavailable",
		Details: map[string]any{},
		Cause:   cause,
	}
}
