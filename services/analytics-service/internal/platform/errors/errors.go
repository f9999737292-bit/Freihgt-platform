package errors

import "net/http"

type Code string

const (
	CodeUnauthorized Code = "UNAUTHORIZED"
	CodeForbidden    Code = "FORBIDDEN"
	CodeValidation   Code = "VALIDATION_ERROR"
	CodeNotFound     Code = "NOT_FOUND"
	CodeUnavailable  Code = "SERVICE_UNAVAILABLE"
)

type Error struct {
	Code    Code
	Message string
}

func (e *Error) Error() string { return string(e.Code) + ": " + e.Message }

func (e *Error) Status() int {
	switch e.Code {
	case CodeUnauthorized:
		return http.StatusUnauthorized
	case CodeForbidden:
		return http.StatusForbidden
	case CodeValidation:
		return http.StatusBadRequest
	case CodeNotFound:
		return http.StatusNotFound
	case CodeUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

func Unauthorized(message string) *Error { return &Error{Code: CodeUnauthorized, Message: message} }
func Forbidden(message string) *Error    { return &Error{Code: CodeForbidden, Message: message} }
func Validation(message string) *Error   { return &Error{Code: CodeValidation, Message: message} }
func NotFound(message string) *Error     { return &Error{Code: CodeNotFound, Message: message} }
func Unavailable(message string) *Error  { return &Error{Code: CodeUnavailable, Message: message} }
